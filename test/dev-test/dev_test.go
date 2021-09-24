package dev_testvg

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	storagev1alpha2 "github.com/dell/dell-csi-volumegroup-snapshotter/api/v1alpha2"
	controller "github.com/dell/dell-csi-volumegroup-snapshotter/controllers"
	"github.com/dell/dell-csi-volumegroup-snapshotter/pkg/connection"
	csiclient "github.com/dell/dell-csi-volumegroup-snapshotter/pkg/csiclient"
	"github.com/dell/dell-csi-volumegroup-snapshotter/test/shared/common"

	csi "github.com/container-storage-interface/spec/lib/go/csi"

	s1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zapcore"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	//"k8s.io/client-go/tools/record"
	"os"
	"testing"
	"time"

	fake_client "github.com/dell/dell-csi-volumegroup-snapshotter/test/shared/fake-client"
	ptypes "github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var testLog = logf.Log.WithName("dev-test")

var vscname = "fooclass"
var vgname = "vg-dev-snap"
var label = "vg-dev-snap-label"
var srcVolId = "4d4a2e5a36080e0f-a234c6c100000002,4d4a2e5a36080e0f-a234c6c200000003"
var ns = "helmtest-vxflexos"
var scName = "fake-sc"
var srcVolIds []string
var srcVolNames []string

// grpc client
var csiConn *grpc.ClientConn

// client to make csi calls to driver
var driverClient csi.ControllerClient

type FakeVGTestSuite struct {
	suite.Suite
	driverName string
	mockUtils  *fake_client.MockUtils
}

// blank assignment to verify client.Client method implementations
var _ client.Client = &fake_client.Client{}

func (suite *FakeVGTestSuite) SetupSuite() {
	suite.Init()
}

// setup csi grpc client , fake client
func (suite *FakeVGTestSuite) Init() {
	opts := zap.Options{
		Development:     true,
		Level:           zapcore.InfoLevel,
		StacktraceLevel: zapcore.PanicLevel,
	}
	opts.BindFlags(flag.CommandLine)
	flag.StringVar(&vgname, "vg", "vg-snap", "name of vg to create")
	flag.StringVar(&ns, "ns", "helmtest-vxflexos", "namespace of vg")
	flag.StringVar(&label, "label", "vg-snap-label", "name of label to use as pvc selector")
	flag.StringVar(&srcVolId, "volid", srcVolId, "id of src volume eg :system-volid")
	flag.Parse()
	srcVolIds = strings.Split(srcVolId, ",")

	logger := zap.New(zap.UseFlagOptions(&opts))
	logf.SetLogger(logger)

	_ = storagev1alpha2.AddToScheme(scheme.Scheme)
	_ = s1.AddToScheme(scheme.Scheme)

	var obj []runtime.Object
	c, err := fake_client.NewFakeClient(obj, nil)
	suite.NoError(err)

	suite.mockUtils = &fake_client.MockUtils{
		FakeClient: c,
		Specs:      common.Common{Namespace: ns},
	}
	suite.driverName = common.DriverName

	csiConn, err = connection.Connect("/root/csi-vxflexos/test/integration/unix_sock")
	if err != nil {
		testLog.Error(err, "failed to connect to CSI driver, make sure start_server.sh has kicked off server")
		os.Exit(1)
	}
	// used to call driver controller.go methods to verify or cleanup powerflex array snapshots
	driverClient = csi.NewControllerClient(csiConn)
}

func TestFakeVGTestSuite(t *testing.T) {
	testSuite := new(FakeVGTestSuite)
	suite.Run(t, testSuite)
	testSuite.TearDownTestSuite()
}

func (suite *FakeVGTestSuite) runFakeVGManager() {

	fakeRecorder := record.NewFakeRecorder(100)

	// make a Reconciler object with grpc client
	// notice Client is set to fake client :the k8s mock
	vgReconcile := &controller.DellCsiVolumeGroupSnapshotReconciler{
		Client:        suite.mockUtils.FakeClient,
		Log:           logf.Log.WithName("vg-controller"),
		EventRecorder: fakeRecorder,
		Scheme:        common.Scheme,
		VGClient:      csiclient.New(csiConn, ctrl.Log.WithName("volumegroup-client"), 100*time.Second),
	}

	// make a request object to pass to Reconcile method in controller
	// this does not contain any k8s object , using the name passed in Reconcile can select
	// object from fake-client in memory objects
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: suite.mockUtils.Specs.Namespace,
			Name:      "vg-snap",
		},
	}

	// invoke controller Reconcile to test. typically k8s would call this when resource is changed
	res, err := vgReconcile.Reconcile(context.Background(), req)

	testLog.Info("reconcile response", "res=", fmt.Sprintf("%#v", res))

	assert.NoError(suite.T(), err, "No error on RG reconcile")
	assert.Equal(suite.T(), res.Requeue, false, "Requeue should be set to false")
}

func (suite *FakeVGTestSuite) CleanupSnapOnArray(srcVolId string) {

	// if there is a snap on array then cleanup
	snapID, err := suite.iCallGetSnapshot(srcVolId)
	assert.Nil(suite.T(), err)
	if snapID != "" {
		err = suite.iCallDeleteSnapshot(snapID)
		assert.Nil(suite.T(), err)
	}
}

func (suite *FakeVGTestSuite) TestCreateVG() {

	// make a k8s object and save in memory ,
	// Reconcile is called to update this object and we can verify
	// hence there is no need to have a k8s environment

	// user want to make a vg with this src volume

	// if vg exists delete it allowing us to rerun without error
	// if there is a snap on array then cleanup

	suite.makeFakeVSC()

	for _, srcId := range srcVolIds {

		suite.CleanupSnapOnArray(srcId)

		// pre-req pv must  exist
		suite.makeFakePV(srcId)

		// user applies this label to pvc
		suite.makeFakePVC(label, srcId)

	}
	// user makes a vg create request to controller to select pvc with this label
	suite.makeFakeVG(vgname, label)

	// run the VG controller Reconcile
	suite.runFakeVGManager()

	suite.verify()
	for _, srcId := range srcVolIds {
		suite.CleanupSnapOnArray(srcId)
	}
}

func (suite *FakeVGTestSuite) verify() {

	// get the fake objects we created before reconcile
	objMap := suite.mockUtils.FakeClient.Objects

	var volGroup *storagev1alpha2.DellCsiVolumeGroupSnapshot
	var count = 0

	for k, v := range objMap {
		//testLog.Info("found fake object ", fmt.Sprintf("%s", k.Name), "created or updated by reconcile")

		if k.Name == "vg-snap" {
			// assert v is of desired type
			volGroup = v.(*storagev1alpha2.DellCsiVolumeGroupSnapshot)

			//testLog.Info("found fake object ", fmt.Sprintf("%s", k), volGroup.Name)
			if k.Name == volGroup.Name {

				// status VG ID was set by reconile in controller

				testLog.Info("found matching fake k8s object", " ID ", volGroup.Status.SnapshotGroupID)
				assert.NotNil(suite.T(), volGroup.Status.SnapshotGroupID, "vg id is present")

				// status was set by reconcile on this fake object , we confirm this

				snaps := volGroup.Status.Snapshots
				testLog.Info("found matching k8s object ", "snap volume = ", snaps)
				for _, t := range srcVolNames {
					count++
					if strings.Contains(t, "fake") {
						testLog.Info("create", "volname", t)
					}
					// todo : verify snapshots
				}
				// todo : verify all the properties like src-id , snap-id
				//assert.Equal(suite.T(), len(srcVolNames), count, "all sourceVoldIds are verified")
			}
		}
	}
	// verify snap exists on powerflex array
	if volGroup != nil {
		found, err := suite.iCallGetSnapshot(srcVolId)
		assert.Nil(suite.T(), err)
		assert.NotNil(suite.T(), found, "snap found on array")

	} else {
		assert.Fail(suite.T(), "VG Reconciliation failed")
	}
}

func (suite *FakeVGTestSuite) debugFakeObjects() {
	objMap := suite.mockUtils.FakeClient.Objects
	for k, v := range objMap {
		testLog.Info("found fake object ", "name", k.Name)
		testLog.Info("found fake object ", "object", fmt.Sprintf("%#v", v))
	}
}

func (suite *FakeVGTestSuite) makeFakeVSC() error {
	vsc := common.MakeVSC(vscname, "fake-dev-driver")
	ctx := context.Background()
	err := suite.mockUtils.FakeClient.Create(ctx, &vsc)
	return err
}

func (suite *FakeVGTestSuite) makeFakeVG(vgname string, label string) {

	// passing ids works , label on pvc also works

	ns := suite.mockUtils.Specs.Namespace
	volumeGroup := common.MakeVG(vgname, ns, suite.driverName, label, "fooclass", "Delete", nil)

	// make a k8s object and save in memory ,  Reconcile is called to update this object and this test can verify
	// hence there is no need to have a k8s environment

	ctx := context.Background()
	err := suite.mockUtils.FakeClient.Create(ctx, &volumeGroup)
	assert.Nil(suite.T(), err)
}

func (suite *FakeVGTestSuite) makeFakePVC(label string, srcVolID string) error {
	lbls := labels.Set{
		"volume-group": label,
	}

	pvcName := "fake-dev-pvc-" + srcVolID
	pvName := "fake-dev-pv-" + srcVolID
	pvcObj := common.MakePVC(pvcName, suite.mockUtils.Specs.Namespace, scName, pvName, lbls)

	ctx := context.Background()
	err := suite.mockUtils.FakeClient.Create(ctx, &pvcObj)
	//suite.debugObjects()
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pvcObj)
	return nil
}

func (suite *FakeVGTestSuite) makeFakePV(srcVolID string) {
	for _, id := range srcVolIds {
		name := "fake-dev-pv-" + id
		srcVolNames = append(srcVolNames, name)
	}
	volumeAttributes := map[string]string{
		"StoragePool": "pool1",
	}
	pvName := "fake-dev-pv-" + srcVolID
	pvObj := common.MakePV(pvName, suite.driverName, srcVolID, scName, volumeAttributes)
	ctx := context.Background()
	err := suite.mockUtils.FakeClient.Create(ctx, &pvObj)
	//suite.debugObjects()
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pvObj)
}

// helper method to call powerflex csi driver
func (suite *FakeVGTestSuite) iCallDeleteSnapshot(snapID string) error {
	ctx := context.Background()
	req := &csi.DeleteSnapshotRequest{
		SnapshotId: snapID,
	}
	// call csi driver using grpc client
	_, err := driverClient.DeleteSnapshot(ctx, req)
	if err != nil {
		fmt.Printf("test DeleteSnapshot returned error: %s\n", err.Error())
		return err
	} else {
		fmt.Printf("test cleanup DeleteSnapshot ok %s\n", req.SnapshotId)
	}
	return nil
}

// helper method to call powerflex csi driver
func (suite *FakeVGTestSuite) iCallGetSnapshot(srcVolID string) (string, error) {
	var err error
	ctx := context.Background()
	req := &csi.ListSnapshotsRequest{}

	// call csi-driver to query powerflex array
	snaps, err := driverClient.ListSnapshots(ctx, req)
	if err != nil {
		return "", err
	}
	nextToken := snaps.GetNextToken()
	if nextToken != "" {
		return "", errors.New("received NextToken on ListSnapshots but didn't expect one")
	}
	entries := snaps.GetEntries()
	for j := 0; j < len(entries); j++ {
		entry := entries[j]
		id := entry.GetSnapshot().SnapshotId
		ts := ptypes.TimestampString(entry.GetSnapshot().CreationTime)
		foundsrcID := entry.GetSnapshot().SourceVolumeId
		if strings.Contains(srcVolID, foundsrcID) {
			fmt.Printf("test found snapshot in powerflex: ID %s source ID %s timestamp %s\n", id, foundsrcID, ts)
			return id, nil
		}
	}
	return "", nil
}

func (suite *FakeVGTestSuite) TearDownTestSuite() {
	testLog.Info("Cleaning up resources...")
	csiConn.Close()
}
