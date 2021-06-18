/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	csiext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	volumegroupv1alpha1 "github.com/dell/dell-csi-volumegroup-snapshotter/api/v1alpha1"
	"github.com/dell/dell-csi-volumegroup-snapshotter/pkg/common"
	csidriver "github.com/dell/dell-csi-volumegroup-snapshotter/pkg/csiclient"
	uuid "github.com/google/uuid"

	"github.com/go-logr/logr"
	s1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	t1 "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	reconcile "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
)

var log logr.Logger

const (
	eventTypeNormal    = "Normal"
	eventTypeWarning   = "Warning"
	eventReasonUpdated = "Updated"
)

// DellCsiVolumeGroupSnapshotReconciler reconciles a DellCsiVolumeGroupSnapshot object
type DellCsiVolumeGroupSnapshotReconciler struct {
	client.Client
	Log           logr.Logger
	VGClient      csidriver.VolumeGroupSnapshot
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

//+kubebuilder:rbac:groups=volumegroup.storage.dell.com,resources=dellcsivolumegroupsnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=volumegroup.storage.dell.com,resources=dellcsivolumegroupsnapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=volumegroup.storage.dell.com,resources=dellcsivolumegroupsnapshots/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DellCsiVolumeGroupSnapshot object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *DellCsiVolumeGroupSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log = r.Log.WithValues("dellcsivolumegroupsnapshot", req.NamespacedName)

	log.Info("VG Snapshotter Triggered")

	// to delete :  volumesnapshotcontent  remove finalizer , this rids of content and snapshot

	schemaErr := s1.AddToScheme(r.Scheme)
	if schemaErr != nil {
		log.Error(schemaErr, fmt.Sprintf("VG Snapshotter vg create failed add to scheme vgname= %s", req.NamespacedName.Name))
		return ctrl.Result{}, schemaErr
	}

	vg := new(volumegroupv1alpha1.DellCsiVolumeGroupSnapshot)
	log.Info("VG Snapshotter reconcile  namespace", "req", fmt.Sprintf("%#v", req.NamespacedName.Namespace))
	if err := r.Get(ctx, req.NamespacedName, vg); err != nil {
		log.Error(err, "VG Snapshotter vg create failed VolumeGroup not found")
		r.EventRecorder.Eventf(vg, eventTypeWarning, eventReasonUpdated, "Failed to create Snapshot of volumegroup with namespace : %#v . error is : %s", req.NamespacedName.Namespace, err.Error())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if vg.Status.SnapshotGroupID != "" {
		// idempotent case
		log.Info("VG Snapshotter loop ..vg exists")
		return ctrl.Result{}, nil
	}

	// set the namespace based on request
	vg.Namespace = req.NamespacedName.Namespace

	// dont proceed if snapshotclass is not found
	snapClassName := vg.Spec.Volumesnapshotclass
	sc := new(s1.VolumeSnapshotClass)
	if err := r.Get(ctx, client.ObjectKey{Name: snapClassName}, sc); err != nil {
		log.Error(err, "VG Snapshotter vg create failed VolumeSnapshotClass not found")
		r.EventRecorder.Eventf(vg, eventTypeWarning, eventReasonUpdated, "Failed vg. error: %s", err.Error())
		return ctrl.Result{}, err
	}

	// get source volume ids
	pvclabel := vg.Spec.PvcLabel

	srcVodIds, volIdPvcNameMap, err := r.getSourceVolids(ctx, pvclabel, vg.Namespace)
	if err != nil {
		log.Error(err, "VG Snapshotter VolumeGroup vg create failed, source volume ids not found")
		r.EventRecorder.Eventf(vg, eventTypeWarning, eventReasonUpdated, "Failed vg. : %s . error: %s", req.Name, err.Error())
		return ctrl.Result{}, err
	}

	// make grpc call to driver
	res, grpcErr := r.VGClient.CreateVolumeGroupSnapshot(req.Name, srcVodIds, nil)
	if grpcErr != nil {
		log.Error(grpcErr, "VG Snapshotter vg create failed in csi driver extension")
		r.EventRecorder.Eventf(vg, eventTypeWarning, eventReasonUpdated, "Failed vg : %s . error: %s", req.Name, grpcErr)
		return ctrl.Result{}, grpcErr
	}
	r.EventRecorder.Eventf(vg, eventTypeNormal, eventReasonUpdated, "Created VG snapshotter vg with name: %s", req.Name)

	// process create VG response
	if _, err := r.processResponse(ctx, res, volIdPvcNameMap, vg, sc); err != nil {
		log.Error(err, "VG Snapshotter vg created ok unable to process create VG response")
		r.EventRecorder.Eventf(vg, eventTypeWarning, eventReasonUpdated, "Failed vg.  error: %s", err.Error())
		return ctrl.Result{}, err
	}

	vg.Status.ContentReadyToUse = true

	// save status and vg-id to k8s vg object
	if err := r.Status().Update(ctx, vg); err != nil {
		log.Error(err, "VG Snapshotter vg created ok unable to update VG status")
		r.EventRecorder.Eventf(vg, eventTypeWarning, eventReasonUpdated, "Failed to update Snapshot of volume group with name : %s . error: %s", req.Name, err.Error())
		return ctrl.Result{}, err
	}
	r.EventRecorder.Eventf(vg, eventTypeNormal, eventReasonUpdated, "Updated VG snapshotter vg with name: %s", req.Name)

	return ctrl.Result{}, grpcErr
}

func (r *DellCsiVolumeGroupSnapshotReconciler) processResponse(ctx context.Context,
	res *csiext.CreateVolumeGroupSnapshotResponse,
	volIdPvcNameMap map[string]string,
	vg *volumegroupv1alpha1.DellCsiVolumeGroupSnapshot,
	sc *s1.VolumeSnapshotClass) (ctrl.Result, error) {

	log.Info("VG Snapshotter reconcile VG-response ", "SnapshotGroupID ", res.SnapshotGroupID)

	snapshots := res.Snapshots
	var snaps string

	var volumeSnapshotName string
	// create ns for query
	nameSpacedName := t1.NamespacedName{
		Namespace: vg.Namespace,
		Name:      "",
	}

	var createdAt *time.Time
	unixTime := time.Unix(0, res.CreationTime)
	createdAt = &unixTime

	// no point in retry hence dont return , keep track of errors in a map
	failedMap := make(map[string]string)
	for _, s := range snapshots {

		log.Info("VG Snapshotter snap info", "snap-name ", s.Name)

		// make a VolumeSnapshot
		pvcName := volIdPvcNameMap[s.SourceId]
		//var volumeSnapshotName string
		//todo change to v1 from betav1
		if s.Name != "" {
			volumeSnapshotName = s.Name + "-" + pvcName
		} else {
			volumeSnapshotName = vg.Name + "-" + pvcName
		}

		snaps = snaps + volumeSnapshotName + ","

		log.Info("VG Snapshotter snap info", "volumesnapshot-name ", volumeSnapshotName)

		contentName := "snapcontent-" + uuid.New().String()

		// dont set this , external snapshotter ends up creating a new snap and a content PersistentVolumeClaimName: &pvcName,

		volsnap := r.makeVolSnap(vg.Namespace, vg.Name, volumeSnapshotName, vg.Spec.Volumesnapshotclass, contentName)

		log.Info("VG Snapshotter create", "VolumeSnapshot for pvc", volumeSnapshotName)

		snapRef, err := r.GetReference(r.Scheme, volsnap)
		if err != nil {
			log.Error(err, "VG Snapshotter vg created ok unable to get Reference to snapvol ")
			failedMap[s.Name] = fmt.Sprintf("VG Snapshotter vg created ok unable to get Reference to VolumeSnapshot %s in %s", volumeSnapshotName, vg.Name)
			return ctrl.Result{}, err
		}

		volsnapcontent := r.makeVolSnapContent(contentName, *snapRef, s, sc)
		log.Info("VG Snapshotter create", "create volume snapshotcontent for", contentName)

		if err := r.Create(ctx, volsnapcontent); err != nil {
			log.Error(err, "VG Snapshotter vg created ok unable to create VolsnapContent ")
			r.EventRecorder.Eventf(vg, eventTypeWarning, eventReasonUpdated, "Failed to create VolumeSnapshotContent %s in %s. error: %s", contentName, vg.Name, err.Error())
			failedMap[s.Name] += fmt.Sprintf("VG Snapshotter vg created ok unable to create VolumeSnapshotContent %s in %s", contentName, vg.Name)
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, volsnap); err != nil {
			log.Error(err, "VG Snapshotter vg created ok unable to create VolumeSnapshot")
			r.EventRecorder.Eventf(vg, eventTypeWarning, eventReasonUpdated, "Failed to Create Volumesnapshot %s in vg %s. error : %s", volumeSnapshotName, vg.Name, err.Error())
			failedMap[s.Name] += fmt.Sprintf("VG Snapshotter vg created ok unable to create VolumeSnapshot  %s in %s", volumeSnapshotName, vg.Name)
			return ctrl.Result{}, err
		}

		// after create update status
		// =============================================

		vc := new(s1.VolumeSnapshotContent)

		nameSpacedName.Namespace = ""
		nameSpacedName.Name = contentName
		if err := r.Get(ctx, nameSpacedName, vc); err != nil {
			log.Error(err, "VG Snapshotter vg created ok  get VolumeSnapshotContent error")
			r.EventRecorder.Eventf(vc, eventTypeWarning, eventReasonUpdated, "Failed to get newly created VolumeSnapshotContent %s in vg %s. error : %s", contentName, vg.Name, err.Error())
			failedMap[s.Name] += fmt.Sprintf("VG Snapshotter vg created ok unable to get newly created VolumeSnapshotContent  %s in %s", contentName, vg.Name)
			return ctrl.Result{}, err
		} else {

			vc.Status = &s1.VolumeSnapshotContentStatus{
				SnapshotHandle: &s.SnapId,
				CreationTime:   &s.CreationTime,
				ReadyToUse:     &s.ReadyToUse,
				RestoreSize:    &s.CapacityBytes,
			}

			//  explict update is needed for vsc

			if err := r.Status().Update(ctx, vc); err != nil {
				log.Error(err, "VG Snapshotter vg created ok unable to update volsnapcontent")
				failedMap[s.Name] = fmt.Sprintf("VG Snapshotter vg created ok unable to update VolumeSnapshotContent  %s in %s", contentName, vg.Name)
				return ctrl.Result{}, err
			}

		}

		log.Info("VG Snapshotter create", "get VolumeSnapshotContent status ", vc.Status.ReadyToUse)
		r.EventRecorder.Eventf(vc, eventTypeNormal, eventReasonUpdated, "Created VG Volumesnapshotcontent in vg %s with name: %s", vg.Name, contentName)
		// =============================================

		// now go back and bind content to snapshot

		vs := new(s1.VolumeSnapshot)
		nameSpacedName.Namespace = vg.Namespace
		nameSpacedName.Name = volumeSnapshotName

		if err := r.Get(ctx, nameSpacedName, vs); err != nil {
			log.Error(err, "VG Snapshotter vg created ok  get VolumeSnapshot error")
			r.EventRecorder.Eventf(vs, eventTypeWarning, eventReasonUpdated, "Failed to get newly created VolumeSnapshot %s in vg %s. error : %s", volumeSnapshotName, vg.Name, err.Error())
			failedMap[s.Name] = fmt.Sprintf("VG Snapshotter vg created ok unable to get newly created VolumeSnapshot %s in %s", volumeSnapshotName, vg.Name)
		} else {
			vs.Status = &s1.VolumeSnapshotStatus{
				BoundVolumeSnapshotContentName: &contentName,
			}
			// works ok without explicit call to update
		}
		log.Info("VG Snapshotter create", "get VolumeSnapshot status ", vs.Status.ReadyToUse)
		r.EventRecorder.Eventf(vs, eventTypeNormal, eventReasonUpdated, "Created VG Volumesnapshots in vg %s with name: %s", vg.Name, volumeSnapshotName)

		// note: external snapshotter updates volumesnapshot status fields
	}

	// todo : add to annotation of vg or status::state
	if len(failedMap) > 0 {
		for k, v := range failedMap {
			log.Error(errors.New("VG Snapshotter Process Response summary"), k, v)
		}
	}

	snaps = strings.TrimRight(snaps, ",")
	vg.Status.SnapshotGroupID = res.SnapshotGroupID
	vg.Status.Snapshots = snaps
	vg.Status.CreationTime = metav1.Time{Time: *createdAt}
	vg.Status.ReadyToUse = true
	vg.Status = volumegroupv1alpha1.DellCsiVolumeGroupSnapshotStatus{
		SnapshotGroupID: res.SnapshotGroupID,
		Snapshots:       snaps,
		CreationTime:    metav1.Time{Time: *createdAt},
		ReadyToUse:      true,
	}

	return ctrl.Result{}, nil
}

func (r *DellCsiVolumeGroupSnapshotReconciler) getSourceVolids(ctx context.Context,
	label string, ns string) ([]string, map[string]string, error) {
	srcVodIds := make([]string, 0)
	volIdPvcNameMap := make(map[string]string)
	pvclabel := label
	pvcList := &v1.PersistentVolumeClaimList{}
	log.Info("VG Snapshotter find matching pvc ", "label", label)
	if pvclabel != "" {
		lbls := labels.Set{
			"volume-group": label,
		}
		// find pvc list with label
		err := r.List(ctx, pvcList, &client.ListOptions{
			Namespace:     ns,
			LabelSelector: labels.SelectorFromSet(lbls),
		})
		if err != nil || len(pvcList.Items) < 1 {
			log.Info("VG Snapshotter unable to create VG pvc not found")
			return nil, nil, fmt.Errorf("VG Snapshotter vg create failed, pvc with label missing %s", label)
		}
	} else {
		log.Info("VG Snapshotter unable to create VG as label not found")
		return nil, nil, fmt.Errorf("VG Snapshotter vg create failed, pvc with label missing")
	}

	var systemID string
	for k, pvc := range pvcList.Items {
		pvName := pvc.Spec.VolumeName
		log.Info("DEBUG VG Snapshotter", "pvc", pvc)
		log.Info("VG Snapshotter found pvc", "name", pvName)
		// find pv for pvc
		pv := &v1.PersistentVolume{}
		err := r.Get(ctx, client.ObjectKey{
			Name: pvName,
		}, pv)
		if err != nil {
			log.Error(err, "VG Snapshotter vg create failed, unable to find pv for pvc")
			//
			return nil, nil, fmt.Errorf("VG Snapshotter vg create failed, unable to find pv for %s", pvName)
		}
		srcVolID := pv.Spec.PersistentVolumeSource.CSI.VolumeHandle
		// check pvc and pv status
		pvcStatus := pvc.Status
		pvStatus := pv.Status
		if pvcStatus.Phase != v1.ClaimBound || pvStatus.Phase != v1.VolumeBound {
			log.Error(errors.New("VG Snapshotter pvc/pv does not have expected status"), "pvc phase", pvcStatus, "pv status", pvStatus)
			log.Error(errors.New("VG Snapshotter pv for pvc with unexpected state"), "id", srcVolID)
			// skip this pvc with matching label
			continue
		}

		if k == 0 {
			getsystemID := strings.Split(srcVolID, "-")
			systemID = getsystemID[0]
		}
		log.Info("VG Snapshotter found systemID", "systemID", systemID)
		currentSystemId := strings.Split(srcVolID, "-")[0]
		if systemID == currentSystemId {
			srcVodIds = append(srcVodIds, srcVolID)
		} else {
			log.Info("VG Snapshotter systemIDs are different")
			return nil, nil, fmt.Errorf("VG Snapshotter vg create failed, VG Snapshotter systemIDs are different %s", systemID)
		}
		log.Info("VG Snapshotter found pvc ", "name", pvc.Name)
		log.Info("VG Snapshotter found pv ", "volume", srcVolID)
		volIdPvcNameMap[srcVolID] = pvc.Name
	}
	if len(srcVodIds) < 1 || len(volIdPvcNameMap) < 1 {
		return nil, nil, fmt.Errorf("VG Snapshotter matching pvc label with pv failed to find source volume ids, label=%s, namespace=%s", label, ns)
	}
	return srcVodIds, volIdPvcNameMap, nil
}

func (r *DellCsiVolumeGroupSnapshotReconciler) makeVolSnap(ns string, vgname string, name string, snapClassName string, content string) *s1.VolumeSnapshot {
	// add label
	labels := make(map[string]string)
	labels[common.LabelSnapshotGroup] = vgname

	volsnap := &s1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    labels,
		},
		Spec: s1.VolumeSnapshotSpec{
			VolumeSnapshotClassName: &snapClassName,

			Source: s1.VolumeSnapshotSource{
				VolumeSnapshotContentName: &content,
			},
		},
	}
	return volsnap
}

func (r *DellCsiVolumeGroupSnapshotReconciler) makeVolSnapContent(contentName string, snapRef v1.ObjectReference, s *csiext.Snapshot, sc *s1.VolumeSnapshotClass) *s1.VolumeSnapshotContent {
	volsnapcontent := &s1.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: contentName,
		},
		Spec: s1.VolumeSnapshotContentSpec{
			VolumeSnapshotRef: snapRef,
			Source: s1.VolumeSnapshotContentSource{
				SnapshotHandle: &s.SnapId,
			},
			VolumeSnapshotClassName: &sc.Name,
			DeletionPolicy:          sc.DeletionPolicy,
			Driver:                  sc.Driver,
		},
	}
	return volsnapcontent
}

// borrowed from external snapshotter

// GetReference returns an ObjectReference which refers to the given
// object, or an error if the object doesn't follow the conventions
// that would allow this.
func (r *DellCsiVolumeGroupSnapshotReconciler) GetReference(scheme *runtime.Scheme, obj runtime.Object) (*v1.ObjectReference, error) {
	log := r.Log.WithValues("VG Snapshotter get reference", "getref")

	if obj == nil {
		return nil, errors.New("VG Snapshotter vg created ok but nil reference oject for VolumeSnapshot")
	}
	if ref, ok := obj.(*v1.ObjectReference); ok {
		// Don't make a reference to a reference.
		return ref, nil
	}

	// An object that implements only List has enough metadata to build a reference
	var listMeta metav1.Common
	objectMeta, err := meta.Accessor(obj)
	if err != nil {
		listMeta, err = meta.CommonAccessor(obj)
		if err != nil {
			log.Error(err, "VG Snapshotter vg created ok unable to get Reference listmeta for VolumeSnapshot")
			return nil, err
		}
	} else {
		listMeta = objectMeta
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	// If object meta doesn't contain data about kind and/or version,
	// we are falling back to scheme.
	//
	if gvk.Empty() {
		gvks, _, err := scheme.ObjectKinds(obj)
		if err != nil {
			log.Error(err, "VG Snapshotter vg created ok unable to get Reference kind")
			return nil, err
		}
		if len(gvks) == 0 || gvks[0].Empty() {
			return nil, fmt.Errorf("VG Snapshotter vg created ok unexpected gvks registered for object %T: %v", obj, gvks)
		}
		gvk = gvks[0]
	}

	kind := gvk.Kind
	version := gvk.GroupVersion().String()

	// only has list metadata
	if objectMeta == nil {
		return &v1.ObjectReference{
			Kind:            kind,
			APIVersion:      version,
			ResourceVersion: listMeta.GetResourceVersion(),
		}, nil
	}

	return &v1.ObjectReference{
		Kind:            kind,
		APIVersion:      version,
		Name:            objectMeta.GetName(),
		Namespace:       objectMeta.GetNamespace(),
		UID:             objectMeta.GetUID(),
		ResourceVersion: objectMeta.GetResourceVersion(),
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
// watch For DellCsiVolumeGroupSnapshot events
func (r *DellCsiVolumeGroupSnapshotReconciler) SetupWithManager(mgr ctrl.Manager, limiter ratelimiter.RateLimiter, maxReconcilers int) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&volumegroupv1alpha1.DellCsiVolumeGroupSnapshot{}).
		WithOptions(reconcile.Options{
			RateLimiter:             limiter,
			MaxConcurrentReconciles: maxReconcilers,
		}).
		Complete(r)
}
