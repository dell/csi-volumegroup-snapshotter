package fake_client

import (
	"context"
	"encoding/json"
	"fmt"
	storagev1alpha1 "github.com/dell/dell-csi-volumegroup-snapshotter/api/v1alpha1"
	"github.com/dell/dell-csi-volumegroup-snapshotter/test/shared/common"
	core_v1 "k8s.io/api/core/v1"
	//storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type ErrorInjector interface {
	ShouldFail(method string, obj runtime.Object) error
}

type StorageKey struct {
	Namespace string
	Name      string
	Kind      string
}

// Objects mocks k8s resources
// ErrorInjector is used to force errors from controller for test
// refer steps.go in int-test folder
type Client struct {
	Objects       map[StorageKey]runtime.Object
	ErrorInjector ErrorInjector
}

type MockUtils struct {
	FakeClient *Client
	Specs      common.Common
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

func (f Client) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("List", list); err != nil {
			return err
		}
	}
	switch list.(type) {
	case *storagev1alpha1.DellCsiVolumeGroupSnapshotList:
		return f.listVG(list.(*storagev1alpha1.DellCsiVolumeGroupSnapshotList))
	case *core_v1.PersistentVolumeClaimList:
		return f.listPersistentVolumeClaim(list.(*core_v1.PersistentVolumeClaimList), opts[0])
	case *core_v1.PersistentVolumeList:
		return f.listPersistentVolume(list.(*core_v1.PersistentVolumeList))
	default:
		return fmt.Errorf("unknown type: %s", reflect.TypeOf(list))
	}
}

func (f *Client) listVG(list *storagev1alpha1.DellCsiVolumeGroupSnapshotList) error {
	for k, v := range f.Objects {
		if k.Kind == "DellCsiVolumeGroupSnapshot" {
			list.Items = append(list.Items, *v.(*storagev1alpha1.DellCsiVolumeGroupSnapshot))
		}
	}
	return nil
}

func (f *Client) listPersistentVolumeClaim(list *core_v1.PersistentVolumeClaimList, opts client.ListOption) error {
	//        opts.ListOptions{
	//          LabelSelector: labels.SelectorFromSet(lbls),
	lo := &client.ListOptions{}
	//selector := opts.LabelSelector
	opts.ApplyToList(lo)
	//labels.internalSelector{labels.Requirement{key:"name", operator:"=", strValues:[]string{"xxxxx"}}}
	//fmt.Printf("debug pvc list labelSelector %#v\n", lo.LabelSelector)

	ls := lo.LabelSelector
	ns := lo.Namespace
	fmt.Printf("debug pvc list for ns = %s ls value =%#v\n", ns, ls.String())

	//debug pvc list lo labels.internalSelector{labels.Requirement{key:"name", operator:"=", strValues:[]string{"vg-snap-label"}}}
	//debug pvc list lo "name=vg-snap-label"
	for k, v := range f.Objects {
		if k.Kind == "PersistentVolumeClaim" && v != nil {
			vol := *v.(*core_v1.PersistentVolumeClaim)

			if vol.ObjectMeta.Namespace != ns {
				fmt.Printf("debug pvc not in same namespace %s %s\n", vol.ObjectMeta.Name, ns)
				continue
			}
			lbs := vol.ObjectMeta.Labels
			for _, l := range lbs {
				if ls.String() == "volume-group="+l {
					fmt.Printf("debug pvc ns %s", vol.ObjectMeta.Namespace)
					fmt.Printf("debug pvc found %#v\n", vol.ObjectMeta.Name)
					list.Items = append(list.Items, vol)
				}
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
	delete(f.Objects, k)
	return nil
}

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

func (f Client) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	panic("implement me")
}

func (f Client) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	panic("implement me")
}

func (f Client) Status() client.StatusWriter {
	return f
}

func (f Client) Scheme() *runtime.Scheme {
	panic("implement me")
}

func (f Client) RESTMapper() meta.RESTMapper {
	panic("implement me")
}
