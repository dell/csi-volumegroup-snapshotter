// Copyright © 2021 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fakeclient

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	vgsv1 "github.com/dell/csi-volumegroup-snapshotter/api/v1"
	"github.com/dell/csi-volumegroup-snapshotter/test/shared/common"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	core_v1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// ErrorInjector to force error
type ErrorInjector interface {
	ShouldFail(method string, obj runtime.Object) error
}

// StorageKey metadata of object to store
type StorageKey struct {
	Namespace string
	Name      string
	Kind      string
}

// Client Objects mocks k8s resources
// ErrorInjector is used to force errors from controller for test
// refer steps.go in int-test folder
type Client struct {
	Objects       map[StorageKey]runtime.Object
	ErrorInjector ErrorInjector
}

// MockUtils fake struct
type MockUtils struct {
	// FakeClient client
	FakeClient *Client
	// Specs client.WithWatch
	Specs common.Common
}

func getKey(obj runtime.Object) (StorageKey, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return StorageKey{}, err
	}
	gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
	if err != nil {
		return StorageKey{}, err
	}
	return StorageKey{
		Name:      accessor.GetName(),
		Namespace: accessor.GetNamespace(),
		Kind:      gvk.Kind,
	}, nil
}

// NewFakeClient create fake client
func NewFakeClient(initialObjects []runtime.Object, errorInjector ErrorInjector) (*Client, error) {
	client := &Client{
		Objects:       map[StorageKey]runtime.Object{},
		ErrorInjector: errorInjector,
	}

	for _, obj := range initialObjects {
		key, err := getKey(obj)
		if err != nil {
			return nil, err
		}
		client.Objects[key] = obj
	}
	return client, nil
}

// Get fake object
func (f Client) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("Get", obj); err != nil {
			return err
		}
	}

	gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
	if err != nil {
		return err
	}
	k := StorageKey{
		Name:      key.Name,
		Namespace: key.Namespace,
		Kind:      gvk.Kind,
	}
	o, found := f.Objects[k]
	if !found {
		gvr := schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}
		return errors.NewNotFound(gvr, key.Name)
	}

	j, err := json.Marshal(o)
	if err != nil {
		return err
	}
	decoder := scheme.Codecs.UniversalDecoder()
	_, _, err = decoder.Decode(j, nil, obj)
	return err
}

// List fake objects
func (f Client) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("List", list); err != nil {
			return err
		}
	}
	switch list.(type) {
	case *vgsv1.DellCsiVolumeGroupSnapshotList:
		return f.listVG(list.(*vgsv1.DellCsiVolumeGroupSnapshotList))
	case *core_v1.PersistentVolumeClaimList:
		return f.listPersistentVolumeClaim(list.(*core_v1.PersistentVolumeClaimList), opts[0])
	case *core_v1.PersistentVolumeList:
		return f.listPersistentVolume(list.(*core_v1.PersistentVolumeList))
	case *snapv1.VolumeSnapshotContentList:
		return f.listVolumeSnapshotContent(list.(*snapv1.VolumeSnapshotContentList))
	default:
		return fmt.Errorf("fake client unknown type: %s", reflect.TypeOf(list))
	}
}

func (f *Client) listVolumeSnapshotContent(list *snapv1.VolumeSnapshotContentList) error {
	for k, v := range f.Objects {
		if k.Kind == "VolumeSnapshotContent" {
			list.Items = append(list.Items, *v.(*snapv1.VolumeSnapshotContent))
		}
	}
	return nil
}

func (f *Client) listVG(list *vgsv1.DellCsiVolumeGroupSnapshotList) error {
	for k, v := range f.Objects {
		if k.Kind == "DellCsiVolumeGroupSnapshot" {
			list.Items = append(list.Items, *v.(*vgsv1.DellCsiVolumeGroupSnapshot))
		}
	}
	return nil
}

func (f *Client) listPersistentVolumeClaim(list *core_v1.PersistentVolumeClaimList, opts client.ListOption) error {
	//        opts.ListOptions{
	//          LabelSelector: labels.SelectorFromSet(lbls),
	lo := &client.ListOptions{}
	// selector := opts.LabelSelector
	opts.ApplyToList(lo)
	// labels.internalSelector{labels.Requirement{key:"name", operator:"=", strValues:[]string{"xxxxx"}}}
	// fmt.Printf("debug pvc list labelSelector %#v\n", lo.LabelSelector)

	ls := lo.LabelSelector
	ns := lo.Namespace
	if ls != nil {
		fmt.Printf("debug pvc list for ns = %s ls value =%#v\n", ns, ls.String())
	} else {
		fmt.Printf("debug pvc list for ns = %s", ns)
	}

	// debug pvc list lo labels.internalSelector{labels.Requirement{key:"name", operator:"=", strValues:[]string{"vg-snap-label"}}}
	// debug pvc list lo "name=vg-snap-label"
	for k, v := range f.Objects {
		if k.Kind == "PersistentVolumeClaim" && v != nil {
			vol := *v.(*core_v1.PersistentVolumeClaim)

			if vol.ObjectMeta.Namespace != ns {
				fmt.Printf("debug pvc not in same namespace %s %s\n", vol.ObjectMeta.Name, ns)
				continue
			}

			if ls != nil {
				lbs := vol.ObjectMeta.Labels
				for _, l := range lbs {
					if ls.String() == "volume-group="+l {
						fmt.Printf("debug pvc ns %s", vol.ObjectMeta.Namespace)
						fmt.Printf("debug pvc found %#v\n", vol.ObjectMeta.Name)
						list.Items = append(list.Items, vol)
					}
				}
			} else {
				list.Items = append(list.Items, vol)
			}
		}
	}
	for _, v := range list.Items {
		fmt.Printf("debug list found \t %s\n", v.ObjectMeta.Name)
	}

	return nil
}

func (f *Client) listPersistentVolume(list *core_v1.PersistentVolumeList) error {
	for k, v := range f.Objects {
		if k.Kind == "PersistentVolume" {
			list.Items = append(list.Items, *v.(*core_v1.PersistentVolume))
		}
	}
	return nil
}

// Create fake object
func (f Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("Create", obj); err != nil {
			return err
		}
	}
	k, err := getKey(obj)
	if err != nil {
		return err
	}
	_, found := f.Objects[k]
	if found {
		gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
		if err != nil {
			return err
		}
		gvr := schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}
		return errors.NewAlreadyExists(gvr, k.Name)
	}
	f.Objects[k] = obj
	return nil
}

// Delete fake object
func (f Client) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if len(opts) > 0 {
		return fmt.Errorf("delete options are not supported")
	}
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("Delete", obj); err != nil {
			return err
		}
	}

	k, err := getKey(obj)
	if err != nil {
		return err
	}
	_, found := f.Objects[k]
	if !found {
		gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
		if err != nil {
			return err
		}
		gvr := schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}
		return errors.NewNotFound(gvr, k.Name)
	}

	// if deletiontimestamp is not zero, we want to go into deletion logic
	if !obj.GetDeletionTimestamp().IsZero() {
		return nil
	}

	// if obj is volumesnapshot, check its delete policy
	// if it is 'delete', delete volumesnapshotcontent
	// if it is 'retain', delete vgs and not volumesnapshots
	if o, ok := obj.(*snapv1.VolumeSnapshot); ok {
		contentname := o.Spec.Source.VolumeSnapshotContentName
		vc := &snapv1.VolumeSnapshotContent{}
		err := f.Get(ctx, client.ObjectKey{
			Name: *contentname,
		}, vc)
		if err != nil {
			return err
		}
		if vc.Spec.DeletionPolicy == "Delete" {
			k2, err := getKey(vc)
			if err != nil {
				return err
			}

			delete(f.Objects, k2)
		}
	}

	delete(f.Objects, k)

	return nil
}

// SetDeletionTimeStamp set deletion timestamp so that reconcile can go into deletion part of code
func (f Client) SetDeletionTimeStamp(ctx context.Context, obj client.Object) error {
	k, err := getKey(obj)
	if err != nil {
		return err
	}

	if len(obj.GetFinalizers()) > 0 {
		obj.SetDeletionTimestamp(&v1.Time{Time: time.Now()})
		f.Objects[k] = obj
		return nil
	}

	return fmt.Errorf("failed to set timestamp")
}

// Update fake object
func (f Client) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("Update", obj); err != nil {
			return err
		}
	}
	k, err := getKey(obj)
	if err != nil {
		return err
	}
	_, found := f.Objects[k]
	if !found {
		gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
		if err != nil {
			return err
		}
		gvr := schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}
		return errors.NewNotFound(gvr, k.Name)
	}
	f.Objects[k] = obj
	return nil
}

// Patch fake object
func (f Client) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	panic("implement me")
}

// DeleteAllOf delete all objects
func (f Client) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	panic("implement me")
}

// Status of client
func (f Client) Status() client.StatusWriter {
	return f
}

// Scheme of client
func (f Client) Scheme() *runtime.Scheme {
	panic("implement me")
}

// RESTMapper for client
func (f Client) RESTMapper() meta.RESTMapper {
	panic("implement me")
}
