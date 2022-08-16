package common

import (
	vgsv1 "github.com/dell/csi-volumegroup-snapshotter/api/v1"
	s1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

//Driver name
type Driver struct {
	DriverName string
}

//DriverName default
var DriverName = "csi-vxflexos"

//Scheme runtime type
var Scheme = runtime.NewScheme()

//Common namespace
type Common struct {
	Namespace string
}

//InitializeSchemes init scheme
func InitializeSchemes() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(vgsv1.AddToScheme(Scheme))
	// +kubebuilder:scaffold:scheme
}

// move utility functions from test code to here

//MakePV create a PV object
func MakePV(name, driverName, srcVolID, scName string, volumeAttributes map[string]string) core_v1.PersistentVolume {
	pvObj := core_v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: core_v1.PersistentVolumeSpec{
			PersistentVolumeSource: core_v1.PersistentVolumeSource{
				CSI: &core_v1.CSIPersistentVolumeSource{
					Driver:           driverName,
					VolumeHandle:     srcVolID,
					VolumeAttributes: volumeAttributes,
				},
			},
			StorageClassName: scName,
		},
		Status: core_v1.PersistentVolumeStatus{Phase: core_v1.VolumeBound},
	}

	return pvObj
}

//MakePVC create a PVC object
func MakePVC(name, ns, scName, volumeName string, lbl labels.Set) core_v1.PersistentVolumeClaim {
	pvcObj := core_v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    lbl,
		},
		Spec: core_v1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			AccessModes:      []core_v1.PersistentVolumeAccessMode{core_v1.ReadWriteOnce},
			Selector: &metav1.LabelSelector{
				MatchLabels: lbl,
			},
			Resources: core_v1.ResourceRequirements{
				Requests: core_v1.ResourceList{
					core_v1.ResourceStorage: resource.MustParse("3Gi"),
				},
			},
		},
	}

	pvcObj.Status.Phase = core_v1.ClaimBound
	pvcObj.Spec.VolumeName = volumeName

	return pvcObj
}

//MakeVSC create a Volumesnapshot Class object
func MakeVSC(name, driver string) s1.VolumeSnapshotClass {
	vsc := s1.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Driver:         driver,
		DeletionPolicy: s1.VolumeSnapshotContentDelete,
	}

	return vsc
}

//MakeVG create a volume group snapshotter object
func MakeVG(name, ns, driver, pvcLabel, vsc string, reclaimPolicy vgsv1.MemberReclaimPolicy, pvcList []string) vgsv1.DellCsiVolumeGroupSnapshot {
	volumeGroup := vgsv1.DellCsiVolumeGroupSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: vgsv1.DellCsiVolumeGroupSnapshotSpec{
			DriverName:          driver,
			MemberReclaimPolicy: reclaimPolicy,
			Volumesnapshotclass: vsc,
			PvcLabel:            pvcLabel,
			PvcList:             pvcList,
			Timeout:             90,
		},
		Status: vgsv1.DellCsiVolumeGroupSnapshotStatus{
			SnapshotGroupID: "",
			Snapshots:       "",
		},
	}

	return volumeGroup
}
