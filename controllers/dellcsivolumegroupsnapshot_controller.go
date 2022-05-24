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
	"sort"
	"strings"
	"sync"
	"time"

	vgsv1 "github.com/dell/csi-volumegroup-snapshotter/api/v1"
	"github.com/dell/csi-volumegroup-snapshotter/pkg/common"
	csidriver "github.com/dell/csi-volumegroup-snapshotter/pkg/csiclient"
	csiext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	uuid "github.com/google/uuid"

	"github.com/go-logr/logr"
	s1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	sclient "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned"
	sinformer "github.com/kubernetes-csi/external-snapshotter/client/v4/informers/externalversions"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	t1 "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	reconcile "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
)

var log logr.Logger

// DellCsiVolumeGroupSnapshotReconciler reconciles a DellCsiVolumeGroupSnapshot object
type DellCsiVolumeGroupSnapshotReconciler struct {
	client.Client
	Log           logr.Logger
	VGClient      csidriver.VolumeGroupSnapshot
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
	DriverName    string
	SnapClient    sclient.Interface
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

	vg := new(vgsv1.DellCsiVolumeGroupSnapshot)
	log.Info("VG Snapshotter reconcile namespace", "req", req.NamespacedName.Namespace)
	if err := r.Get(ctx, req.NamespacedName, vg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// handle deletion by checking for deletion timestamp
	if !vg.DeletionTimestamp.IsZero() {
		if vg.Status.Status == common.EventStatusComplete && containString(vg.GetFinalizers(), common.FinalizerName) {
			err := r.deleteVg(ctx, vg)
			return ctrl.Result{}, err
		}
		log.Info("VG Snapshotter stauts is incomplete. If you still want to delete, manually clean up")
		return ctrl.Result{}, fmt.Errorf("VG Snapshotter status is incomplete. Nothing is deleted")
	}

	if vg.Status.SnapshotGroupID != "" && vg.Status.Status == common.EventStatusComplete {
		// idempotent case
		log.Info("VG Snapshotter reconcile ..vg exists", vg.Name, vg.Status.SnapshotGroupID)
		r.EventRecorder.Eventf(vg, v1.EventTypeWarning, common.EventReasonUpdated, "VG exists")
		return ctrl.Result{}, nil
	}

	if vg.Status.Status == common.EventStatusPending {
		log.Info("VG Snapshotter reconcile found vg exists in pending state", "groupID", vg.Status.SnapshotGroupID)
		log.Info("VG Snapshotter reconcile found vg exists in pending state", "members list", vg.Status.Snapshots)
		return ctrl.Result{}, fmt.Errorf("VG Snapshotter status is pending. Avoid creation of another snapshot")
	}

	// set vg status to pending
	vg.Status.Status = common.EventStatusPending
	if err := r.Status().Update(ctx, vg); err != nil {
		log.Error(err, "VG Snapshotter vg status pending update")
	}

	// when user provides both pvc label and pvc name list, log error and return
	if vg.Spec.PvcLabel != "" && vg.Spec.PvcList != nil {
		err := fmt.Errorf("PvcLabel and PvcList can't be provided at the same time. PvcLabel=%s, PvcList=%v", vg.Spec.PvcLabel, vg.Spec.PvcList)
		log.Error(err, "VG Snapshotter vg create failed both pvc label and name list found")
		r.EventRecorder.Eventf(vg, v1.EventTypeWarning, common.EventReasonUpdated, "Failed vg create: error: both pvc label and list not supported ")
		return ctrl.Result{}, err
	}

	// dont proceed if snapshotclass is not found
	snapClassName := vg.Spec.Volumesnapshotclass
	sc := new(s1.VolumeSnapshotClass)
	if scerr := r.Get(ctx, client.ObjectKey{Name: snapClassName}, sc); scerr != nil {
		vg.Status.Status = common.EventStatusError
		if err := r.Status().Update(ctx, vg); err != nil {
			log.Error(err, "VG Snapshotter vg status update")
		}
		log.Error(scerr, "VG Snapshotter vg create failed VolumeSnapshotClass not found")
		r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Failed vg create: error: %s", scerr.Error())
		return ctrl.Result{}, scerr
	}

	if !r.driverVerification(sc) {
		derr := errors.New("VG Snapshotter vg create failed, VolumeSnapshotClass driver name does not match volumegroupsnapshotter")
		log.Error(derr, sc.Driver)
		vg.Status.Status = common.EventStatusError
		if err := r.Status().Update(ctx, vg); err != nil {
			log.Error(err, "VG Snapshotter vg status update")
		}
		r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Failed vg create: error: %s", derr.Error())
		return ctrl.Result{}, derr
	}

	// get source volume ids
	ns := vg.Namespace
	var srcVolIDs []string
	var volIDPvcNameMap map[string]string
	var volErr error
	if vg.Spec.PvcList != nil {
		srcVolIDs, volIDPvcNameMap, volErr = r.getSourceVolIdsFromPvcName(ctx, vg.Spec.PvcList, ns)
	} else if vg.Spec.PvcLabel != "" {
		srcVolIDs, volIDPvcNameMap, volErr = r.getSourceVolIdsFromLabel(ctx, vg.Spec.PvcLabel, ns)
	} else {
		// snapshot pvcs under given namespace
		srcVolIDs, volIDPvcNameMap, volErr = r.getSourceVolIdsFromNs(ctx, ns)
	}

	if volErr != nil {
		vg.Status.Status = common.EventStatusError
		if err := r.Status().Update(ctx, vg); err != nil {
			log.Error(err, "VG Snapshotter VolumeGroup vg create failed, source volume ids not found")
		}
		r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Failed vg create: %s error: %s", req.Name, volErr.Error())
		return ctrl.Result{}, volErr
	}

	// if req name has a suffix timestamp, just use it
	groupName := req.Name

	// set VG name in status if array supports it

	sort.Strings(srcVolIDs)
	// make grpc call to driver
	otherParams := make(map[string]string)
	//remove systemID
	gid := vg.Status.SnapshotGroupID
	tokens := strings.Split(gid, "-")
	index := len(tokens)
	if index > 0 {
		otherParams[common.ExistingGroupID] = tokens[index-1]
	}
	log.Info("VG Snapshotter vg create", "existing GroupID", otherParams[common.ExistingGroupID])
	res, grpcErr := r.VGClient.CreateVolumeGroupSnapshot(groupName, srcVolIDs, otherParams)
	if grpcErr != nil {
		vg.Status.Status = common.EventStatusError
		if err := r.Status().Update(ctx, vg); err != nil {
			log.Error(err, "VG Snapshotter vg status create vg using driver update")
		}
		log.Error(grpcErr, "VG Snapshotter vg create failed in csi driver. Snapshot consistency group may exist on array. Please check and clean up manually")
		r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Failed vg create: %s error: %s", groupName, grpcErr)
		return ctrl.Result{}, grpcErr
	}
	r.EventRecorder.Eventf(vg, common.EventTypeNormal, common.EventReasonUpdated, "Created snapshot consistency group on array with id: %s", res.SnapshotGroupID)

	// add finalizer to vg
	if !containString(vg.GetFinalizers(), common.FinalizerName) {
		log.Info("VG Snapshotter vg create adding finalizer to vg", "vg name", groupName)
		controllerutil.AddFinalizer(vg, common.FinalizerName)
		// update VG
		if err := r.Update(ctx, vg); err != nil {
			log.Error(err, "VG Snapshotter vg create failed, unable to update to add finalizer")
			return ctrl.Result{}, err
		}
	}

	if vg.Status.SnapshotGroupID == "" && res.SnapshotGroupID != "" {
		log.Info("VG Snapshotter vg create set group id", "vg name", groupName, "vg id", res.SnapshotGroupID)
		vg.Status.SnapshotGroupID = res.SnapshotGroupID
		if err := r.Update(ctx, vg); err != nil {
			log.Error(err, "VG Snapshotter vg create failed, unable to update to add group id")
			return ctrl.Result{}, err
		}
	}

	// process create VG response
	var pErr error
	if _, pErr = r.processResponse(ctx, res, volIDPvcNameMap, vg, sc, groupName); pErr != nil {
		log.Error(pErr, "VG Snapshotter vg processSnapshots failed")
	}

	// save status after processing VG response
	if err := r.Status().Update(ctx, vg); err != nil {
		log.Error(err, "VG Snapshotter vg created ok unable to update VG status after processing response ")
		r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Failed vg create: %s error: %s", groupName, err.Error())
		return ctrl.Result{}, err
	}
	// update vg and then return error
	if pErr != nil {
		return ctrl.Result{}, pErr
	}

	r.EventRecorder.Eventf(vg, common.EventTypeNormal, common.EventReasonUpdated, "Created volume group with id: %s", res.SnapshotGroupID)
	return ctrl.Result{}, nil
}

func (r *DellCsiVolumeGroupSnapshotReconciler) driverVerification(sc *s1.VolumeSnapshotClass) bool {
	// Check for the driver match
	if sc.Driver != r.DriverName {
		log.Info("Volumesnapshotclass created using the driver name, not matching one on this volumegroupsnapshotter", "driverName", sc.Driver)
		return false
	}
	return true
}

func (r *DellCsiVolumeGroupSnapshotReconciler) processResponse(ctx context.Context,
	res *csiext.CreateVolumeGroupSnapshotResponse,
	volIDPvcNameMap map[string]string,
	vg *vgsv1.DellCsiVolumeGroupSnapshot,
	sc *s1.VolumeSnapshotClass,
	groupName string) (ctrl.Result, error) {

	log.Info("VG Snapshotter reconcile VG-response ", "SnapshotGroupID ", res.SnapshotGroupID)

	snapshots := res.Snapshots
	var snaps string

	var volumeSnapshotName string
	var createdAt *time.Time
	unixTime := time.Unix(0, res.CreationTime)
	createdAt = &unixTime

	// keep track of errors in a map to print summary for troubleshooting
	failedMap := make(map[string]string)

	go r.snapWatch()

	snapErrors := make([]error, 0)
	for _, s := range snapshots {

		log.Info("VG Snapshotter snap info", "snap-name ", s.Name)

		// make a VolumeSnapshot
		pvcName := volIDPvcNameMap[s.SourceId]
		// var volumeSnapshotName string
		if s.Name != "" {
			volumeSnapshotName = s.Name + "-" + pvcName
		} else {
			volumeSnapshotName = vg.Name + "-" + pvcName
		}

		snaps = snaps + volumeSnapshotName + ","

		log.Info("VG Snapshotter snap info", "volumesnapshot-name ", volumeSnapshotName)

		contentName := "snapcontent-" + uuid.New().String()

		// dont set this , external snapshotter ends up creating a new snap and a content
		updateSnapIDForContent := false
		var createErr error
		if updateSnapIDForContent, contentName, createErr = r.createVolumeSnapshot(ctx, volumeSnapshotName, contentName, vg, s, sc, failedMap); createErr != nil {
			r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "create snapshor error  %s", createErr.Error())
			snapErrors = append(snapErrors, createErr)
		}

		// after create update status
		if updateErr := r.updateVolumeSnapshotContentStatus(ctx, contentName, updateSnapIDForContent,
			s, vg.Name, failedMap); updateErr != nil {
			r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "snapshot content status failed %s", updateErr.Error())
			snapErrors = append(snapErrors, updateErr)
		}
	}

	// note: external snapshotter updates volumesnapshot status fields
	// wait till snapshot status is updated to done for all snapshots

	ok, err := r.checkSnapshotStatus(ctx, vg.Namespace, snaps)
	status := common.EventStatusError
	if ok {
		status = common.EventStatusComplete
		r.EventRecorder.Eventf(vg, common.EventTypeNormal, common.EventReasonUpdated, "all snapshots ok")
	}

	if err != nil {
		r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "snapshots state not Ready %s", err.Error())
	}

	// todo : add to annotation of vg or status::state
	if len(failedMap) > 0 {
		log.Error(errors.New("VG Snapshotter Process Response"), "summary")
		for k, v := range failedMap {
			log.Info("VG error details", k, v)
		}
	}

	snaps = strings.TrimRight(snaps, ",")
	vg.Status = vgsv1.DellCsiVolumeGroupSnapshotStatus{
		SnapshotGroupID: res.SnapshotGroupID,
		Snapshots:       snaps,
		CreationTime:    metav1.Time{Time: *createdAt},
		ReadyToUse:      ok,
		Status:          status,
	}
	// save status after processing VG response
	if err := r.Status().Update(ctx, vg); err != nil {
		log.Error(err, "VG Snapshotter vg created ok unable to update VG status while processing response")
		r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Failed vg create: %s error: %s", groupName, err.Error())
		snapErrors = append(snapErrors, err)
	}
	if len(snapErrors) > 0 {
		msg := " "
		for _, e := range snapErrors {
			msg += e.Error() + " "
		}
		return ctrl.Result{}, errors.New("VG Snapshotter Process Response failed " + msg)
	}
	return ctrl.Result{}, nil
}

func (r *DellCsiVolumeGroupSnapshotReconciler) checkReadyToUse(ctx context.Context, ns string, snapshots string) (bool, error) {
	snaps := strings.Split(snapshots, ",")
	for _, snapName := range snaps {
		log.Info("VG check snaps", "name=", snapName)
		// get snap
		snap := new(s1.VolumeSnapshot)
		nameSpacedName := t1.NamespacedName{
			Namespace: ns,
			Name:      snapName,
		}
		_ = r.Get(ctx, nameSpacedName, snap)
		if snap.Name != "" && snap.Status != nil {
			status := *snap.Status.ReadyToUse
			if !status {
				log.Info("VG check snap ReadyToUse", "ReadyToUse", "false")
				return false, nil
			}
			log.Info("VG check snap ready", snapName, status)
		} else {
			log.Info("VG check snap", "not ready", snapName)
			continue
		}
	}
	return true, nil
}

func (r *DellCsiVolumeGroupSnapshotReconciler) checkSnapshotStatus(ctx context.Context, ns string, snaps string) (bool, error) {
	timeout := time.After(10 * time.Second)
	ticker := time.Tick(500 * time.Millisecond)
	// Keep trying until we're timed out or get a result/error
	for {
		select {
		// Got a timeout! fail with error
		case <-timeout:
			return false, errors.New("timed out")
		// Got a tick, we should check on checkReadyToUse()
		case <-ticker:
			log.Info("VG check snap", "snaps", snaps)
			ok, err := r.checkReadyToUse(ctx, ns, snaps)
			if err != nil {
				// We may return, or ignore the error
				return false, err
				// checkReadyToUse() done! let's return
			} else if ok {
				return true, nil
			}
			// checkReadyToUse() isn't done yet, but it didn't fail either, let's try again
		}
	}
}

func (r *DellCsiVolumeGroupSnapshotReconciler) bindContentToSnapshot(
	ctx context.Context,
	volumeSnapshotName string,
	contentName string,
	vgNamespace string,
	vgName string,
	failedMap map[string]string) error {

	vs := new(s1.VolumeSnapshot)
	nameSpacedName := t1.NamespacedName{
		Namespace: vgNamespace,
		Name:      volumeSnapshotName,
	}

	if err := r.Get(ctx, nameSpacedName, vs); err != nil {
		log.Error(err, "VG Snapshotter vg created ok  get VolumeSnapshot error")
		r.EventRecorder.Eventf(vs, common.EventTypeWarning, common.EventReasonUpdated, "Failed to get newly created VolumeSnapshot %s in vg %s. error : %s", volumeSnapshotName, vgName, err.Error())
	} else {
		vs.Status = &s1.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &contentName,
		}
		// works ok without explicit call to update
	}
	log.Info("VG Snapshotter create", "get VolumeSnapshot status ", vs.Status)
	r.EventRecorder.Eventf(vs, common.EventTypeNormal, common.EventReasonUpdated, "Created VG Volumesnapshots in vg %s with name: %s", vgName, volumeSnapshotName)
	return nil
}

// handles vg deletion
func (r *DellCsiVolumeGroupSnapshotReconciler) deleteVg(
	ctx context.Context,
	vg *vgsv1.DellCsiVolumeGroupSnapshot) error {
	log.Info("VG Snapshotter deletion triggered")

	// get deletion policy from volumesnapshotclass
	vsc := new(s1.VolumeSnapshotClass)
	vscName := vg.Spec.Volumesnapshotclass
	if err := r.Get(ctx, client.ObjectKey{Name: vscName}, vsc); err != nil {
		return fmt.Errorf("VG Snapshotter can't find volume snapshot class %s", vscName)
	}
	deletionPolicy := vsc.DeletionPolicy

	ns := vg.Namespace
	snapshotNames := strings.Split(vg.Status.Snapshots, ",")

	// if deletion policy is delete, delete VolumeSnapshot members
	if deletionPolicy == s1.VolumeSnapshotContentDelete {
		for _, snapName := range snapshotNames {
			snapshot := &s1.VolumeSnapshot{}
			if err := r.Get(ctx, client.ObjectKey{
				Namespace: ns,
				Name:      snapName,
			}, snapshot); err != nil {
				// user manually deleted this snapshot, continue deleting the others
				log.Info("VG Snapshotter can't find snap to delete", "snap name", snapName, "err", err)
				continue
			}

			if err := r.Delete(ctx, snapshot); err != nil {
				log.Error(err, "vg snapshotter vg deletion failed")
				r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "failed to delete snapshots of volumegroup")
				return err
			}
			log.Info(fmt.Sprintf("VG Snapshotter snapshot %s deleted", snapshot.Name))
		}

		controllerutil.RemoveFinalizer(vg, common.FinalizerName)
		if err := r.Update(ctx, vg); err != nil {
			log.Error(err, "VG Snapshotter failed updating after remove finalizer")
			return err
		}
		log.Info("VG Snapshotter deleted", "vg name", vg.Name)
	}

	// if deletion policy is retain, do nothing
	if deletionPolicy == s1.VolumeSnapshotContentRetain {
		log.Info("VG Snapshotter specified storage class has DeletionPolicy as Retain, no operation is performed")
	}

	return nil
}

func (r *DellCsiVolumeGroupSnapshotReconciler) createVolumeSnapshot(
	ctx context.Context,
	volumeSnapshotName string,
	contentName string,
	vg *vgsv1.DellCsiVolumeGroupSnapshot,
	s *csiext.Snapshot,
	sc *s1.VolumeSnapshotClass,
	failedMap map[string]string) (bool, string, error) {

	volsnap := new(s1.VolumeSnapshot)
	nameSpacedName := t1.NamespacedName{
		Namespace: vg.Namespace,
		Name:      volumeSnapshotName,
	}

	volumeSnapshotExists := false
	_ = r.Get(ctx, nameSpacedName, volsnap)
	if volsnap.Name == "" {
		volsnap = r.makeVolSnap(vg.Namespace, vg.Name, volumeSnapshotName, vg.Spec.Volumesnapshotclass, contentName)
		log.Info("VG Snapshotter create", "VolumeSnapshot for pvc", volumeSnapshotName)
		volumeSnapshotExists = false
	} else {
		volumeSnapshotExists = true
		log.Info("VG Snapshotter", "volumesnapshot exists skip create", volsnap.Name)
		volumeSnapshotExistsHasError, volsnapErr := r.checkExistingVolSnapshot(ctx, volsnap, vg, s.SnapId)
		if volumeSnapshotExistsHasError {
			return false, "", volsnapErr
		}
	}

	var snapRef *v1.ObjectReference
	var refErr error
	snapRef, refErr = r.GetReference(r.Scheme, volsnap)
	if refErr != nil {
		log.Error(refErr, "VG Snapshotter vg created ok unable to get Reference to snapvol ")
		failedMap[s.Name] = fmt.Sprintf("VG Snapshotter vg created ok unable to get Reference to VolumeSnapshot %s in %s", volumeSnapshotName, vg.Name)
		return false, "", refErr
	}

	updateSnapIDForContent := false
	var vgName = vg.Name
	var contentNameExists string
	var contentErr error
	if volumeSnapshotExists {
		var err error
		contentNameExists, updateSnapIDForContent, contentErr = r.checkExistingVolsnapcontent(ctx, volsnap, vg, s.SnapId)
		if contentErr != nil {
			log.Error(err, "VG Snapshotter vg created ok unable to use existing volsnapcontent")
			failedMap[s.Name] = fmt.Sprintf("VG Snapshotter vg created ok unable to update VolumeSnapshotContent  %s in %s", contentName, vgName)
			return false, "", contentErr
		}
		log.Info("VG Snapshotter create", "volume snapshotcontent exists", updateSnapIDForContent)
	} else {
		log.Info("VG Snapshotter create", "create volume snapshotcontent", contentName)
		volsnapcontent := r.makeVolSnapContent(contentName, *snapRef, s, sc)
		if err := r.Create(ctx, volsnapcontent); err != nil {
			log.Error(err, "VG Snapshotter vg created ok unable to create VolsnapContent ")
			r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Failed to create VolumeSnapshotContent %s in %s. error: %s", contentName, vg.Name, err.Error())
			failedMap[s.Name] += fmt.Sprintf("VG Snapshotter vg created ok unable to create VolumeSnapshotContent %s in %s", contentName, vgName)
			return false, "", err
		}

		// after content create above proceed to create volsnap

		if err := r.Create(ctx, volsnap); err != nil {
			log.Error(err, "VG Snapshotter vg created ok unable to create VolumeSnapshot")
			r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Failed to Create Volumesnapshot %s in vg %s. error : %s", volumeSnapshotName, vg.Name, err.Error())
			failedMap[s.Name] += fmt.Sprintf("VG Snapshotter vg created ok unable to create VolumeSnapshot  %s in %s", volumeSnapshotName, vgName)
			return false, "", err
		}
		contentNameExists = contentName
		updateSnapIDForContent = false
	}
	log.Info("VG Snapshotter create", "snap content name", contentNameExists)
	return updateSnapIDForContent, contentNameExists, nil
}

func (r *DellCsiVolumeGroupSnapshotReconciler) updateVolumeSnapshotContentStatus(
	ctx context.Context,
	contentName string,
	updateSnapIDForContent bool,
	s *csiext.Snapshot,
	vgName string,
	failedMap map[string]string) error {

	vc := &s1.VolumeSnapshotContent{}
	nameSpacedName := t1.NamespacedName{
		Namespace: "",
		Name:      contentName,
	}

	if err := r.Get(ctx, nameSpacedName, vc); err != nil {
		log.Error(err, "VG Snapshotter vg created ok  get VolumeSnapshotContent error")
		r.EventRecorder.Eventf(vc, common.EventTypeWarning, common.EventReasonUpdated, "Failed to get newly created VolumeSnapshotContent %s in vg %s. error : %s", contentName, vgName, err.Error())
		failedMap[s.Name] += fmt.Sprintf("VG Snapshotter vg created ok unable to get newly created VolumeSnapshotContent  %s in %s", contentName, vgName)
		return err
	}

	if updateSnapIDForContent {
		// update existing volsnapcontent stale snapid
		log.Info("VG Snapshotter content update existing", "snapId to", s.SnapId)
		vc.Spec.Source = s1.VolumeSnapshotContentSource{
			SnapshotHandle: &s.SnapId,
		}

		vc.Status = &s1.VolumeSnapshotContentStatus{
			SnapshotHandle: &s.SnapId,
			CreationTime:   &s.CreationTime,
			ReadyToUse:     &s.ReadyToUse,
			RestoreSize:    &s.CapacityBytes,
		}

		if err := r.Status().Update(ctx, vc); err != nil {
			log.Error(err, "VG Snapshotter vg created ok unable to update volsnapcontent")
			failedMap[s.Name] = fmt.Sprintf("VG Snapshotter vg created ok unable to update VolumeSnapshotContent  %s in %s", contentName, vgName)
			return err
		}
		if err := r.Get(ctx, nameSpacedName, vc); err != nil {
			log.Error(err, "VG Snapshotter vg created ok unable to get updated volsnapcontent")
			failedMap[s.Name] = fmt.Sprintf("VG Snapshotter vg created ok unable to update VolumeSnapshotContent  %s in %s", contentName, vgName)
			return err
		}
	}
	log.Info("VG Snapshotter content SnapshotHandle", "snapshot volume", vc.Spec.Source.SnapshotHandle)
	if vc.Status != nil {
		log.Info("VG Snapshotter content create", "get VolumeSnapshotContent status ", vc.Status.ReadyToUse)
	}
	r.EventRecorder.Eventf(vc, common.EventTypeNormal, common.EventReasonUpdated, "Created VG Volumesnapshotcontent in vg %s with name: %s", vgName, contentName)
	return nil
}

func (r *DellCsiVolumeGroupSnapshotReconciler) checkExistingVolsnapcontent(
	ctx context.Context,
	volsnap *s1.VolumeSnapshot,
	vg *vgsv1.DellCsiVolumeGroupSnapshot,
	snapID string) (string, bool, error) {

	vcList := &s1.VolumeSnapshotContentList{}
	err := r.List(ctx, vcList, &client.ListOptions{})
	if err != nil || len(vcList.Items) < 1 {
		log.Error(err, "VG Snapshotter unable to do volsnap contentlist")
		return "", false, err
	}
	var contentNameExists string
	updateSnapIDForContent := false
	var contentErr error
	volumeSnapshotName := volsnap.Name
	volumeSnapshotNamespace := volsnap.Namespace
	for _, vc := range vcList.Items {
		if volumeSnapshotName == vc.Spec.VolumeSnapshotRef.Name && volumeSnapshotNamespace == vc.Spec.VolumeSnapshotRef.Namespace {
			log.Info("VG Snapshotter", "found VolumeSnapshotContent exists ", vc.Name)
			log.Info("VG Snapshotter", "found existing VolumeSnapshot name", vc.Spec.VolumeSnapshotRef.Name, "namespace", vc.Spec.VolumeSnapshotRef.Namespace)
			contentNameExists = vc.Name
			updateSnapIDForContent, contentErr = r.validateExistingVolSnapcontent(vc, volsnap, vg, snapID)
			break
		}
	}
	return contentNameExists, updateSnapIDForContent, contentErr
}

func (r *DellCsiVolumeGroupSnapshotReconciler) validateExistingVolSnapcontent(
	vc s1.VolumeSnapshotContent,
	volsnap *s1.VolumeSnapshot,
	vg *vgsv1.DellCsiVolumeGroupSnapshot,
	snapID string) (bool, error) {
	var existVolumeSnapshotContentError bool
	updateSnapIDForContent := false
	var contentErr error

	if *vc.Spec.Source.SnapshotHandle != snapID {
		log.Info("VG Snapshotter vg created ok existing VolumeSnapshotContent Source.SnapshotHandle update needed")
		updateSnapIDForContent = true
	}

	if vc.Spec.Driver != vg.Spec.DriverName {
		contentErr = fmt.Errorf("VolumeSnapshotContent Driver %s %s", vc.Spec.Driver, vg.Spec.DriverName)
		log.Error(contentErr, "VG Snapshotter vg created ok  VolumeSnapshotContent not matching vg fix manually")
		existVolumeSnapshotContentError = true
	}

	existingSnapshotClasName := volsnap.Spec.VolumeSnapshotClassName
	if *existingSnapshotClasName != *vc.Spec.VolumeSnapshotClassName {
		contentErr = fmt.Errorf("VolumeSnapshotContent VolumeSnapshotClass %s %s", *vc.Spec.VolumeSnapshotClassName, *existingSnapshotClasName)
		log.Error(contentErr, "VG Snapshotter vg created ok VolumeSnapshotContent not matching volumesnapshot fix manually")
		existVolumeSnapshotContentError = true
	}

	if vc.Spec.VolumeSnapshotRef.Name != volsnap.Name || vc.Spec.VolumeSnapshotRef.Namespace != volsnap.Namespace {
		contentErr = fmt.Errorf("VolumeSnapshotContent VolumeSnapshotRef name %s %s namespace %s %s", vc.Spec.VolumeSnapshotRef.Name, volsnap.Name, vc.Spec.VolumeSnapshotRef.Namespace, volsnap.Namespace)
		log.Error(contentErr, "VG Snapshotter vg created ok VolumeSnapshotContent not matching volumesnapshot fix manually")
		existVolumeSnapshotContentError = true
	}

	if existVolumeSnapshotContentError {
		r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Warning existing volumeSnapshotContent found %s in %s not matching. error: %s", vc.Name, vg.Name, contentErr.Error())
		updateSnapIDForContent = false
	}
	return updateSnapIDForContent, contentErr
}

func (r *DellCsiVolumeGroupSnapshotReconciler) checkExistingVolSnapshot(
	ctx context.Context,
	volsnap *s1.VolumeSnapshot,
	vg *vgsv1.DellCsiVolumeGroupSnapshot,
	snapID string) (bool, error) {

	log.Info("VG Snapshotter", "volumesnapshot exists skip create name=", volsnap.Name)
	var existVolumeSnapshotError bool
	var snapErr error
	var contentErr error
	existingContentname := volsnap.Spec.Source.VolumeSnapshotContentName
	vc := &s1.VolumeSnapshotContent{}
	nameSpacedName := t1.NamespacedName{
		Namespace: "",
		Name:      *existingContentname,
	}
	if err := r.Get(ctx, nameSpacedName, vc); err != nil {
		snapErr = err
		log.Error(err, "VG Snapshotter vg created ok  get existing VolumeSnapshotContent error")
		existVolumeSnapshotError = true
	}

	log.Info("VG Snapshotter volsnap", "existing VolumeSnapshotContent name", existingContentname)
	if volsnap.Status != nil {
		if !*volsnap.Status.ReadyToUse {
			snapErr = errors.New("VG Snapshotter vg created ok existing VolumeSnapshot Status.ReadyToUse")
			log.Error(snapErr, "VG Snapshotter vg created ok existing VolumeSnapshot Status.ReadyToUse  error")
			existVolumeSnapshotError = true
		}
		log.Info("VG Snapshotter volsnap", "BoundVolumeSnapshotcontent", volsnap.Status.BoundVolumeSnapshotContentName)

		if *volsnap.Status.BoundVolumeSnapshotContentName != *existingContentname {
			contentErr = errors.New("VG Snapshotter vg created ok VolumeSnapshot exists with BoundVolumeSnapshotContentName  different from VolumeSnapshotContentName")
			log.Error(contentErr, "VG Snapshotter vg created ok VolumeSnapshot not matching")
			existVolumeSnapshotError = true
		}
	} else {
		log.Info("VG Snapshotter vg created ok VolumeSnapshot matching, proceed to re use")
	}

	if *vc.Spec.Source.SnapshotHandle != snapID {
		log.Info("VG Snapshotter vg created ok existing VolumeSnapshot refers to VolumeSnapshotContent with Source.SnapshotHandle mismatch procced to reuse")
		// check if snapContent volid exists on array
	}
	existingSnapshotClasName := volsnap.Spec.VolumeSnapshotClassName
	if *existingSnapshotClasName != vg.Spec.Volumesnapshotclass {
		snapErr = errors.New("VG Snapshotter vg created ok volumesnapshot exits with different Volumesnapshotclass")
		log.Error(snapErr, "VG Snapshotter vg created ok VolumeSnapshot not matching, fix manually")
		existVolumeSnapshotError = true
	}
	if existVolumeSnapshotError && snapErr != nil {
		r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Warning existing volumeSnapshot found %s in %s. error: %s", volsnap.Name, vg.Name, snapErr)
		return existVolumeSnapshotError, snapErr
	}
	if existVolumeSnapshotError && contentErr != nil {
		r.EventRecorder.Eventf(vg, common.EventTypeWarning, common.EventReasonUpdated, "Warning volumeSnapshotContent %s cannot be used in %s. error: %s", *existingContentname, vg.Name, contentErr.Error())
		return existVolumeSnapshotError, contentErr
	}
	return false, nil

}

// get a list of source volume Ids, a map from these Ids to corresponding pvc names based on the given pvc label
func (r *DellCsiVolumeGroupSnapshotReconciler) getSourceVolIdsFromLabel(ctx context.Context,
	label string, ns string) ([]string, map[string]string, error) {
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

	return r.mapVolIDToPvcName(ctx, pvcList.Items)
}

// get a list of source volume Ids, a map from these Ids to corresponding pvc names based on a list of pvc names
func (r *DellCsiVolumeGroupSnapshotReconciler) getSourceVolIdsFromPvcName(ctx context.Context, pvcNames []string, ns string) ([]string, map[string]string, error) {
	duplicateMap := make(map[string]bool)
	pvcList := make([]v1.PersistentVolumeClaim, 0)
	for _, pvcName := range pvcNames {
		// check if pvcNames containes duplicate elements
		_, present := duplicateMap[pvcName]
		if present {
			log.Info("VG Snapshotter vg finds duplicate PVC names ", "pvc name", pvcName)
			continue
		} else {
			duplicateMap[pvcName] = true
		}

		// find pvc for the given pvc Name
		pvc := &v1.PersistentVolumeClaim{}
		err := r.Get(ctx, client.ObjectKey{
			Name:      pvcName,
			Namespace: ns,
		}, pvc)
		if err != nil {
			log.Error(err, "VG Snapshotter vg create failed, unable to find pvc for pvc name")
			return nil, nil, fmt.Errorf("VG Snapshotter vg create failed, unable to find pvc for pvc Name %s", pvcName)
		}

		pvcList = append(pvcList, *pvc)
	}

	return r.mapVolIDToPvcName(ctx, pvcList)
}

func (r *DellCsiVolumeGroupSnapshotReconciler) getSourceVolIdsFromNs(ctx context.Context, ns string) ([]string, map[string]string, error) {
	pvcList := &v1.PersistentVolumeClaimList{}
	log.Info("VG Snapshotter find matching pvc", "namespace", ns)

	err := r.List(ctx, pvcList, &client.ListOptions{
		Namespace: ns,
	})
	if err != nil || len(pvcList.Items) < 1 {
		log.Info("VG Snapshotter unable to create VG pvc not found")
		return nil, nil, fmt.Errorf("VG Snapshotter vg create failed, no pvc found under ns %s", ns)
	}

	return r.mapVolIDToPvcName(ctx, pvcList.Items)
}

// helper function for getSourceVolIds.
// takes a list of PVCs and return a list of source volume Ids, a map from these Ids to corresponding pvc names
func (r *DellCsiVolumeGroupSnapshotReconciler) mapVolIDToPvcName(ctx context.Context, pvcs []v1.PersistentVolumeClaim) ([]string, map[string]string, error) {
	srcVolIDs := make([]string, 0)
	volIDPvcNameMap := make(map[string]string)
	var systemID string
	for k, pvc := range pvcs {
		pvName := pvc.Spec.VolumeName
		log.Info("VG Snapshotter found pvc", "name", pvName)

		// find pv for pvc
		pv := &v1.PersistentVolume{}
		err := r.Get(ctx, client.ObjectKey{
			Name: pvName,
		}, pv)
		if err != nil {
			log.Error(err, "VG Snapshotter vg create failed, unable to find pv for pvc")
			return nil, nil, fmt.Errorf("VG Snapshotter vg create failed, unable to find pv for %s", pvName)
		}
		srcVolID := pv.Spec.PersistentVolumeSource.CSI.VolumeHandle

		// check pvc and pv status
		if pvc.Status.Phase != v1.ClaimBound || pv.Status.Phase != v1.VolumeBound {
			statusErr := fmt.Errorf("pvc/pv %s does not have expected status phase : %s", srcVolID, pvc.Status.Phase)
			log.Error(statusErr, "VG Snapshotter pv for pvc has unexpected state")
			// skip this pvc with matching label
			continue
		}

		if k == 0 {
			systemID = strings.Split(srcVolID, "-")[0]
			log.Info("VG Snapshotter found systemID", "systemID", systemID)
		}
		currentsystemID := strings.Split(srcVolID, "-")[0]
		if systemID == currentsystemID {
			srcVolIDs = append(srcVolIDs, srcVolID)
		} else {
			log.Info("VG Snapshotter systemIDs are different", "first system Id:", systemID, "current system Id:", currentsystemID)
			return nil, nil, fmt.Errorf("VG Snapshotter vg create failed, VG Snapshotter systemIDs are different %s", systemID)
		}

		log.Info("VG Snapshotter found pvc ", "name", pvc.Name)
		log.Info("VG Snapshotter found pv ", "volumeId", srcVolID)
		volIDPvcNameMap[srcVolID] = pvc.Name
	}

	if len(srcVolIDs) < 1 || len(volIDPvcNameMap) < 1 {
		return nil, nil, fmt.Errorf("VG Snapshotter matching pvc label with pv failed to find source volume ids")
	}

	return srcVolIDs, volIDPvcNameMap, nil
}

func (r *DellCsiVolumeGroupSnapshotReconciler) makeVolSnap(ns string, vgname string, name string, snapClassName string, content string) *s1.VolumeSnapshot {
	// add label
	lbls := make(map[string]string)
	lbls[common.LabelSnapshotGroup] = vgname

	volsnap := &s1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    lbls,
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

func (r *DellCsiVolumeGroupSnapshotReconciler) ignoreUpdatePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to status in which case metadata.Generation does not change
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Evaluates to false if the object has been confirmed deleted.
			return !e.DeleteStateUnknown
		},
	}
}

func (r *DellCsiVolumeGroupSnapshotReconciler) handleSnapUpdate(oldObj interface{}, obj interface{}) {
	nsnapshot, _ := obj.(*s1.VolumeSnapshot)

	// vgs-helm-test-1-pvol1
	ctx := context.Background()
	ns := nsnapshot.Namespace
	vgsName := nsnapshot.Labels[common.LabelSnapshotGroup]
	namespacedName := t1.NamespacedName{
		Name:      vgsName,
		Namespace: ns,
	}
	vg := new(vgsv1.DellCsiVolumeGroupSnapshot)
	err := r.Get(ctx, namespacedName, vg)
	if err == nil && nsnapshot.Status != nil {
		status := *nsnapshot.Status.ReadyToUse
		log.Info("VG Snapshotter update snap watch", "ReadyToUse=", status)
		msg := "snap " + nsnapshot.Name + " ReadyToUse=" + fmt.Sprintf("%t", status)
		r.EventRecorder.Eventf(vg, common.EventTypeNormal, common.EventReasonUpdated, msg)
		return
	}
	if err == nil && nsnapshot.Status == nil {
		// This snapshot is created by VG
		ctx := context.Background()
		contentName := nsnapshot.Spec.Source.VolumeSnapshotContentName
		log.Info("VG Snapshotter update snap watch call bind content", vgsName, ns)
		if err := r.bindContentToSnapshot(ctx, nsnapshot.Name, *contentName, ns, vgsName, nil); err != nil {
			log.Error(err, "VG Snapshotter vg created ok  bind VolumeSnapshot content error")
		}
		msg := "upadate snapshot bind content " + *contentName
		r.EventRecorder.Eventf(vg, common.EventTypeNormal, common.EventReasonUpdated, msg)
	}
	return
}

func (r *DellCsiVolumeGroupSnapshotReconciler) handleSnapCreate(obj interface{}) {

	snapshot, ok := obj.(*s1.VolumeSnapshot)
	if !ok {
		err := fmt.Errorf("VG Snapshotter watcher volumesnapshot watch fails to convert obj to volumeSnapshot")
		log.Error(err, "cast error")
		return
	}

	ctx := context.Background()

	snapName := snapshot.Name
	ns := snapshot.Namespace
	vgsName := snapshot.Labels[common.LabelSnapshotGroup]

	if vgsName != "" {
		// This snapshot is created by VG
		namespacedName := t1.NamespacedName{
			Name:      vgsName,
			Namespace: ns,
		}
		vg := new(vgsv1.DellCsiVolumeGroupSnapshot)
		err := r.Get(ctx, namespacedName, vg)
		if err == nil {
			// snap belongs to this vg check status
			if snapshot.Status != nil && *snapshot.Status.BoundVolumeSnapshotContentName != "" {
				log.Info("VG Snapshotter create snaphot", "status", snapshot.Status)
			}
		}
		msg := "create snap " + snapName
		r.EventRecorder.Eventf(vg, common.EventTypeNormal, common.EventReasonUpdated, msg)
	}
	return
}

func (r *DellCsiVolumeGroupSnapshotReconciler) snapWatch() error {

	var clientset sclient.Interface
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Info(err.Error(), "vg snapshotter setup snapWatch", "test mode")
		clientset = r.SnapClient
	} else {
		clientset, err = sclient.NewForConfig(config)
		if err != nil {
			log.Error(err, "vg snapshotter volumesnapshot watcher newforconfig err")
			return err
		}
	}

	sharedInformerFactory := sinformer.NewSharedInformerFactory(clientset, 0)
	contentInformer := sharedInformerFactory.Snapshot().V1().VolumeSnapshots().Informer()
	contentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    r.handleSnapCreate,
		UpdateFunc: r.handleSnapUpdate,
	})
	stop := make(chan struct{})
	sharedInformerFactory.Start(stop)
	return nil
}

func (r *DellCsiVolumeGroupSnapshotReconciler) snapContentWatch() error {

	var clientset sclient.Interface
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Info(err.Error(), "vg snapshotter setup snapWatch", "test mode")
		clientset = r.SnapClient
	} else {
		clientset, err = sclient.NewForConfig(config)
		if err != nil {
			log.Error(err, "vg snapshotter volumesnapshotcontent watcher newforconfig err")
			return err
		}
	}

	sharedInformerFactory := sinformer.NewSharedInformerFactory(clientset, time.Duration(time.Hour))
	contentInformer := sharedInformerFactory.Snapshot().V1().VolumeSnapshotContents().Informer()

	contentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: r.HandleSnapContentDelete,
	})

	stop := make(chan struct{})
	sharedInformerFactory.Start(stop)

	return nil
}

// HandleSnapContentDelete updates VG status based on changes to volumesnapshotcontent
func (r *DellCsiVolumeGroupSnapshotReconciler) HandleSnapContentDelete(obj interface{}) {

	content, ok := obj.(*s1.VolumeSnapshotContent)
	if !ok {
		err := fmt.Errorf("VG Snapshotter watcher volumesnapshotcontent watch fails to convert obj to vscontent")
		log.Error(err, "cast error")
		return
	}

	log.Info("VG Snapshotter watcher volumesnapcontent deleted:", "content name", content.Name)

	ctx := context.Background()

	// vgs-helm-test-1-pvol1
	snapName := content.Spec.VolumeSnapshotRef.Name
	ns := content.Spec.VolumeSnapshotRef.Namespace

	snap := new(s1.VolumeSnapshot)
	err := r.Get(ctx, client.ObjectKey{
		Name:      snapName,
		Namespace: ns,
	}, snap)

	if err != nil {
		// Snap already deleted. Need to add label like "snapshotGroup=vgs-helm-test" to content also
		log.Error(err, "VG Snapshotter watcher snapshotcontent watch can't find its bound snapshot %s")
		return
	}

	vgsName := snap.Labels[common.LabelSnapshotGroup]
	if vgsName != "" {
		// This snapshot is created by VG

		namespacedName := t1.NamespacedName{
			Name:      vgsName,
			Namespace: ns,
		}

		schemaMutex := sync.RWMutex{}
		schemaMutex.Lock()
		vg := new(vgsv1.DellCsiVolumeGroupSnapshot)

		err := r.Get(ctx, namespacedName, vg)
		if err == nil {
			log.Info("VG Snapshotter watcher finds the deleted volumesnapshotcontent is created by a VGS", "VGS name", vg.Name)
			snapshotsSlice := strings.Split(vg.Status.Snapshots, ",")

			// try removing this snapshot from vgs snapshots list
			for i, s := range snapshotsSlice {
				if s == snapName {
					// remove this element if match the deleted volumesnapshot
					snapshotsSlice = append(snapshotsSlice[:i], snapshotsSlice[i+1:]...)
					break
				}
			}

			snapshotsString := strings.Join(snapshotsSlice, ",")
			vg.Status.Snapshots = snapshotsString

			if err := r.Status().Update(ctx, vg); err != nil {
				log.Error(err, "VG Snapshotter failed to update status when snapshotcontent is deleted")
			}

			if snapshotsString == "" {
				// All snapshotcontents have been deleted and we can delete the VG object

				log.Info("VG Snapshotter watcher all snapshots under vg has been deleted. Deleting VG object")
				if err = r.Delete(ctx, vg); err != nil {
					log.Error(err, "VG Snapshotter watcher failed to delete VG in snapshotcontent watcher")
				}
			}
		}

		schemaMutex.Unlock()
	}
}

// SetupWithManager sets up the controller with the Manager.
// watch For DellCsiVolumeGroupSnapshot events
func (r *DellCsiVolumeGroupSnapshotReconciler) SetupWithManager(mgr ctrl.Manager, limiter ratelimiter.RateLimiter, maxReconcilers int) error {
	go r.snapContentWatch()

	return ctrl.NewControllerManagedBy(mgr).
		For(&vgsv1.DellCsiVolumeGroupSnapshot{}).
		WithEventFilter(r.ignoreUpdatePredicate()).
		WithOptions(reconcile.Options{
			RateLimiter:             limiter,
			MaxConcurrentReconciles: maxReconcilers,
		}).
		Complete(r)
}

// Helper functions to check and remove string from a slice of strings
func containString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
