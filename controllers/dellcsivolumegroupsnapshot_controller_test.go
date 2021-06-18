package controllers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dell/dell-csi-volumegroup-snapshotter/api/v1alpha1"
	connection "github.com/dell/dell-csi-volumegroup-snapshotter/pkg/connection"
	"github.com/dell/dell-csi-volumegroup-snapshotter/pkg/csiclient"
	"github.com/dell/dell-csi-volumegroup-snapshotter/test/mock-server/server"
	"github.com/dell/dell-csi-volumegroup-snapshotter/test/shared/common"
	fake_client "github.com/dell/dell-csi-volumegroup-snapshotter/test/shared/fake-client"
	s1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	scName        = "fake-sc"
	label         = "vg-snap-label"
	systemId      = "4d4a2e5a36080e0f"
	srcVolId      = systemId + "-12345"
	wrongSrcVolId = "123-123"
	vgName        = "vg-snap"
	vgNameTestErr = "vg-snap-error"
	fakePvName1   = "fake-pv1"
	fakePvName2   = "fake-pv2"
	fakePvcName1  = "fake-pvc1"
	fakePvcName2  = "fake-pvc2"
	port          = "localhost:4772"
	vscName       = "vsc"
	ctx           = context.Background()
)

type VGSControllerTestSuite struct {
	suite.Suite
	driver    common.Driver
	mockUtils *fake_client.MockUtils
}

func (suite *VGSControllerTestSuite) SetupTest() {
	fmt.Println("Init test suite...")
	suite.Init()
	server.RunServer("../test/mock-server/stubs")
}

func (suite *VGSControllerTestSuite) Init() {
	v1alpha1.AddToScheme(scheme.Scheme)
	s1.AddToScheme(scheme.Scheme)
	suite.driver = common.Driver{
		DriverName: "csi-fake",
	}

	var obj []runtime.Object
	c, err := fake_client.NewFakeClient(obj, nil)
	suite.NoError(err)

	suite.mockUtils = &fake_client.MockUtils{
		FakeClient: c,
		Specs:      common.Common{Namespace: "fake-ns"},
	}
}

func (suite *VGSControllerTestSuite) TearDownTest() {
	fmt.Println("Cleaning up resources...")
	server.StopMockServer()
}

func TestMain(t *testing.T) {
	testSuite := new(VGSControllerTestSuite)
	suite.Run(t, testSuite)
}

// ======================= Test cases start here =======================

// test a succesful reconcile
func (suite *VGSControllerTestSuite) TestReconcileWithNoError() {
	suite.makeFakeVG(ctx, label, vgName)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolId)
	suite.makeFakePVC(ctx, label, fakePvcName1, fakePvName1)
	suite.runFakeVGManager(vgName, "")
}

// test reconcile without a VG and error should be volumegroup not found
func (suite *VGSControllerTestSuite) TestVgNotFound() {
	suite.runFakeVGManager(vgName, "DellCsiVolumeGroupSnapshot.volumegroup.storage.dell.com \"vg-snap\" not found")
}

// test reconcile without a VSC and result should be vsc not found
func (suite *VGSControllerTestSuite) TestVscNotFound() {
	suite.makeFakeVG(ctx, label, vgName)
	suite.runFakeVGManager(vgName, "VolumeSnapshotClass.snapshot.storage.k8s.io \"vsc\" not found")
}

// test getSourceVolIds with with non-empty pvcLabel but no matching pvc. pvcList should be empty and return error
func (suite *VGSControllerTestSuite) TestNoMatchingPVC() {
	noMatchingLbl := "noMatchingLbl"
	suite.makeFakeVG(ctx, noMatchingLbl, vgName)
	suite.makeFakeVSC(ctx)
	suite.runFakeVGManager(vgName, fmt.Sprintf("VG Snapshotter vg create failed, pvc with label missing %s", noMatchingLbl))
}

// test getSourceVolIds with empty pvcLabel and return error
func (suite *VGSControllerTestSuite) TestEmptyLabelAndSystemWithNoPVC() {
	suite.makeFakeVG(ctx, "", vgName)
	suite.makeFakeVSC(ctx)
	suite.runFakeVGManager(vgName, "VG Snapshotter vg create failed, pvc with label missing")
}

// test getSourceVolIds where PVC is found, but there is no corresponding PV
func (suite *VGSControllerTestSuite) TestNoPvFoundInGetSourceVolIds() {
	suite.makeFakeVG(ctx, label, vgName)
	suite.makeFakeVSC(ctx)
	suite.makeFakePVC(ctx, label, fakePvcName1, fakePvName1)
	suite.runFakeVGManager(vgName, "VG Snapshotter vg create failed, unable to find pv for")
}

// test getSourceVolIds where two PVCs are created, but corresponding PVs have different system ID
func (suite *VGSControllerTestSuite) TestDifferentSystemPV() {
	suite.makeFakeVG(ctx, label, vgName)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolId)
	suite.makeFakePV(ctx, fakePvName2, wrongSrcVolId)
	suite.makeFakePVC(ctx, label, fakePvcName1, fakePvName1)
	suite.makeFakePVC(ctx, label, fakePvcName2, fakePvName2)
	suite.runFakeVGManager(vgName, "VG Snapshotter vg create failed, VG Snapshotter systemIDs are different")
}

// test getSourceVolIds where two PVCs are created, but PVC is not Bound to PV
func (suite *VGSControllerTestSuite) TestPVCNotBound() {
	suite.makeFakeVG(ctx, label, vgName)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolId)
	suite.makeFakePVCNotBound(ctx, label, fakePvcName1, fakePvName1)
	suite.runFakeVGManager(vgName, "VG Snapshotter matching pvc label with pv failed")
}

// test grpc call error to driver
func (suite *VGSControllerTestSuite) TestCsiDriverCreateFail() {
	suite.makeFakeVG(ctx, label, vgNameTestErr)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolId)
	suite.makeFakePVC(ctx, label, fakePvcName1, fakePvName1)
	suite.runFakeVGManager(vgNameTestErr, "deliberately return error to test csi driver extension grpc error")
}

// test where snapshotName is reused
func (suite *VGSControllerTestSuite) TestReuseSnapName() {
	suite.makeFakeVG(ctx, label, "bad-vg-snap")
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolId)
	suite.makeFakePVC(ctx, label, fakePvcName1, fakePvName1)
	suite.runFakeVGManager("bad-vg-snap", "already exists")

}

// test idempotent case
func (suite *VGSControllerTestSuite) TestIdempotentCase() {
	suite.makeFakeVG(ctx, label, vgName)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolId)
	suite.makeFakePVC(ctx, label, fakePvcName1, fakePvName1)
	suite.runFakeVGManager(vgName, "")
	suite.runFakeVGManager(vgName, "")
}

//Bad reference test
func (suite *VGSControllerTestSuite) TestReconcileBadRef() {

	key := fake_client.StorageKey{
		Name:      "fake-pvc1",
		Namespace: "fake-ns",
		Kind:      "PersistentVolumeClaim",
	}

	suite.makeFakeVG(ctx, label, vgName)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolId)
	suite.makeFakePVC(ctx, label, fakePvcName1, fakePvName1)
	suite.runGetReference(vgName, suite.mockUtils.FakeClient.Objects[key], "no kind is registered for the type")

}

func (suite *VGSControllerTestSuite) TestGetRefToRef() {
	suite.makeFakeVSC(ctx)

	key := fake_client.StorageKey{
		Name: vscName,
		Kind: "VolumeSnapshotClass",
	}

	ref := suite.runGetReference(vgName, suite.mockUtils.FakeClient.Objects[key], "")
	suite.runGetReference(vgName, ref, "")

}

func (suite *VGSControllerTestSuite) TestGetRefToUnknownObj() {
	unknown := runtime.Unknown{}
	suite.runGetReference(vgName, &unknown, "object does not implement the common interface for accessing the SelfLink")
}

// Test GetReference with nil obj
func (suite *VGSControllerTestSuite) TestGetRefWithNilObj() {
	suite.runGetReference(vgName, nil, "VG Snapshotter vg created ok but nil reference oject for VolumeSnapshot")
}

// ===================== All helper methods =====================

func (suite *VGSControllerTestSuite) makeFakePV(ctx context.Context, name, localSrcVolId string) {
	volumeAttributes := map[string]string{
		"StoragePool": "pool1",
	}

	pvObj := common.MakePV(name, suite.driver.DriverName, localSrcVolId, scName, volumeAttributes)
	err := suite.mockUtils.FakeClient.Create(ctx, &pvObj)
	assert.NotNil(suite.T(), pvObj)
	assert.Nil(suite.T(), err)
}

func (suite *VGSControllerTestSuite) makeFakePVCNotBound(ctx context.Context, lbl, name, volumeName string) {
	lbls := labels.Set{
		"name": lbl,
	}
	ns := suite.mockUtils.Specs.Namespace
	pvcObj := common.MakePVC(name, ns, scName, volumeName, lbls)
	pvcObj.Status.Phase = core_v1.ClaimLost
	err := suite.mockUtils.FakeClient.Create(ctx, &pvcObj)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pvcObj)
}

func (suite *VGSControllerTestSuite) makeFakePVC(ctx context.Context, lbl, name, volumeName string) {
	lbls := labels.Set{
		"name": lbl,
	}
	ns := suite.mockUtils.Specs.Namespace
	pvcObj := common.MakePVC(name, ns, scName, volumeName, lbls)
	err := suite.mockUtils.FakeClient.Create(ctx, &pvcObj)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pvcObj)
}

func (suite *VGSControllerTestSuite) makeFakeVSC(ctx context.Context) {
	vsc := common.MakeVSC(vscName, suite.driver.DriverName)
	err := suite.mockUtils.FakeClient.Create(ctx, &vsc)
	assert.Nil(suite.T(), err)
}

func (suite *VGSControllerTestSuite) makeFakeVG(ctx context.Context, localPvcLabel, localVgName string) {
	ns := suite.mockUtils.Specs.Namespace
	volumeGroup := common.MakeVG(localVgName, ns, suite.driver.DriverName, localPvcLabel, vscName)
	err := suite.mockUtils.FakeClient.Create(ctx, &volumeGroup)
	assert.Nil(suite.T(), err)
}

func (suite *VGSControllerTestSuite) runFakeVGManager(localVgName, expectedErr string) {
	csiConn, err := connection.Connect(port)
	if err != nil {
		fmt.Printf("failed to connect to CSI driver, err: %#v", err)
		os.Exit(1)
	}

	fakeRecorder := record.NewFakeRecorder(100)
	// make a Reconciler object with grpc client
	// notice Client is set to fake client: the k8s mock
	vgReconcile := &DellCsiVolumeGroupSnapshotReconciler{
		Client:        suite.mockUtils.FakeClient,
		Log:           logf.Log.WithName("vg-controller"),
		EventRecorder: fakeRecorder,
		Scheme:        common.Scheme,
		VGClient:      csiclient.New(csiConn, ctrl.Log.WithName("volumegroup-client"), 100*time.Second),
	}

	// make a request object to pass to Reconcile method in controller
	// this does not contain any k8s object , using the name passed in
	// Reconcile can select object from fake-client in memory objects
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: suite.mockUtils.Specs.Namespace,
			Name:      localVgName,
		},
	}

	// invoke controller Reconcile to test. typically k8s would call this when resource is changed
	res, err := vgReconcile.Reconcile(context.Background(), req)

	fmt.Printf("reconcile response res=%#v\n", res)

	if expectedErr == "" {
		assert.NoError(suite.T(), err)
	}

	if err != nil {
		fmt.Printf("Error returned is: %s", err.Error())
		assert.True(suite.T(), strings.Contains(err.Error(), expectedErr))
	}
}

func (suite *VGSControllerTestSuite) runGetReference(localVgName string, obj runtime.Object, expectedErr string) *core_v1.ObjectReference {
	csiConn, err := connection.Connect(port)
	if err != nil {
		fmt.Printf("failed to connect to CSI driver, err: %#v", err)
		os.Exit(1)
	}

	// make a Reconciler object with grpc client
	// notice Client is set to fake client: the k8s mock
	vgReconcile := &DellCsiVolumeGroupSnapshotReconciler{
		Client:   suite.mockUtils.FakeClient,
		Log:      logf.Log.WithName("vg-controller"),
		Scheme:   common.Scheme,
		VGClient: csiclient.New(csiConn, ctrl.Log.WithName("volumegroup-client"), 100*time.Second),
	}

	ref, err := vgReconcile.GetReference(vgReconcile.Scheme, obj)

	fmt.Printf("error is: %v \n", err)
	if err != nil {
		assert.True(suite.T(), strings.Contains(err.Error(), expectedErr))
	}
	return ref
}
