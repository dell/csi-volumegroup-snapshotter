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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DellCsiVolumeGroupSnapshotSpec defines the desired state of DellCsiVolumeGroupSnapshot
type DellCsiVolumeGroupSnapshotSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	DriverName          string              `json:"driverName"`
	MemberReclaimPolicy MemberReclaimPolicy `json:"memberReclaimPolicy"`
	Volumesnapshotclass string              `json:"volumesnapshotclass"`
	PvcLabel            string              `json:"pvcLabel,omitempty"`
	PvcList             []string            `json:"pvcList,omitempty"`
}

// MemberReclaimPolicy describes a policy for end-of-life maintenance of VGS
// +kubebuilder:validation:Enum=Delete;Retain
type MemberReclaimPolicy string

const (
	//MemberReclaimDelete to delete VG
	MemberReclaimDelete = "Delete"
	//MemberReclaimRetain to retail VG
	MemberReclaimRetain = "Retain"
)

// DellCsiVolumeGroupSnapshotStatus defines the observed state of DellCsiVolumeGroupSnapshot
type DellCsiVolumeGroupSnapshotStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	SnapshotGroupID   string      `json:"snapshotGroupID"`
	SnapshotGroupName string      `json:"snapshotGroupName"`
	Snapshots         string      `json:"snapshots"`
	CreationTime      metav1.Time `json:"creationTime,omitempty"`
	ReadyToUse        bool        `json:"readyToUse,omitempty"`
	ContentReadyToUse bool        `json:"contentReadyToUse,omitempty"`
	Status            string      `json:"status,omitempty"`
}

// +kubebuilder:validation:Optional
// +kubebuilder:resource:scope=Namespaced,shortName={"vgs"}
// +kubebuilder:printcolumn:name="VolumeGroupname",type=string,JSONPath=`.status.snapshotGroupName`,description="Name of the VG"
// +kubebuilder:printcolumn:name="CreationTime",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`,description="Status of the VG"
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DellCsiVolumeGroupSnapshot is the Schema for the dellcsivolumegroupsnapshots API
type DellCsiVolumeGroupSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DellCsiVolumeGroupSnapshotSpec   `json:"spec,omitempty"`
	Status DellCsiVolumeGroupSnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DellCsiVolumeGroupSnapshotList contains a list of DellCsiVolumeGroupSnapshot
type DellCsiVolumeGroupSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DellCsiVolumeGroupSnapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DellCsiVolumeGroupSnapshot{}, &DellCsiVolumeGroupSnapshotList{})
}
