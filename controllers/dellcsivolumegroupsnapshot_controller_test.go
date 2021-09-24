package controllers

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dell/dell-csi-volumegroup-snapshotter/api/v1alpha2"
	pkg_common "github.com/dell/dell-csi-volumegroup-snapshotter/pkg/common"
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
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	scName         = "fake-sc"
	label          = "vg-snap-label"
	systemId       = "4d4a2e5a36080e0f"
	srcVolID       = systemId + "-12345"
	wrongSrcVolID  = "123-123"
	vgName         = "vg-snap"
	vgNameTestErr  = "vg-snap-error"
	fakePvName1    = "fake-pv1"
	fakePvName2    = "fake-pv2"
	fakePvcName1   = "fake-pvc1"
	fakePvcName2   = "fake-pvc2"
	port           = "localhost:4772"
	vscName        = "vsc"
	drivernamerr   = "powerflex"
	ctx            = context.Background()
	createVSCError bool
	updateVSCError bool
	getVCSError    bool
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
	v1alpha2.AddToScheme(scheme.Scheme)
	s1.AddToScheme(scheme.Scheme)
	suite.driver = common.Driver{
		DriverName: common.DriverName,
	}

	var obj []runtime.Object
	c, err := fake_client.NewFakeClient(obj, suite)
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

func TestCustom(t *testing.T) {
	testSuite := new(VGSControllerTestSuite)
	suite.Run(t, testSuite)
}

func (suite *VGSControllerTestSuite) ShouldFail(method string, obj runtime.Object) error {
	switch v := obj.(type) {
	case *s1.VolumeSnapshotContent:
		vsc := obj.(*s1.VolumeSnapshotContent)
		if method == "Create" && createVSCError {
			fmt.Printf("[ShouldFail] force vsc error for vsc named %+v\n", vsc.Name)
			fmt.Printf("[ShouldFail] force vsc error for obj of type %+v\n", v)
			return errors.New("unable to create VolsnapContent")
		} else if method == "Update" && updateVSCError {
			fmt.Printf("[ShouldFail] force update vs error for obj of type %+v\n", v)
			return errors.New("unable to update VolsnapContent")
		} else if method == "Get" && getVCSError {
			fmt.Printf("[ShouldFail] force get vs error for obj of type %+v\n", v)
			return errors.New("unable to get VolsnapContent")
		}
	default:
	}
	return nil
}

// ======================= Test cases start here =======================

// test a reconcile with broken Create
func (suite *VGSControllerTestSuite) TestReconcileWithCreateError() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	createVSCError = true
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "unable to create VolsnapContent")
	createVSCError = false
}

// test a reconcile with broken Get
func (suite *VGSControllerTestSuite) TestReconcileWithGetError() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	getVCSError = true
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "unable to get VolsnapContent")
	getVCSError = false
}

// test a reconcile with broken Update
func (suite *VGSControllerTestSuite) TestReconcileWithUpdateError() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	updateVSCError = true
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "unable to update VolsnapContent")
	updateVSCError = false
}

// test a successful reconcile and delete with retain reclaim policy
func (suite *VGSControllerTestSuite) TestReconcileWithNoErrorRetain() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
	suite.deleteFakeVG(ctx, vgName, true)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
}

// test a successful reconcile and delete with delete reclaim policy
func (suite *VGSControllerTestSuite) TestReconcileWithNoErrorDelete() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Delete", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
	suite.deleteFakeVG(ctx, vgName, true)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
}

// test a successful reconcile and delete with all content deleted
func (suite *VGSControllerTestSuite) TestReconcileWithAllContentDeleted() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Delete", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
	suite.deleteFakeVG(ctx, vgName, true)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
}

// test a failed reconcile due to using reclaim policy other than retain/delete
func (suite *VGSControllerTestSuite) TestDeleteError() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "test", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
	suite.deleteFakeVG(ctx, vgName, true)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "memberReclaimPolicy should be either delete or retain")
}

// test a succesful reconcile
func (suite *VGSControllerTestSuite) TestReconcileWithNoError() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
}

// test a succesful reconcile with two VGs in different namespaces
func (suite *VGSControllerTestSuite) TestReconcileWithSameNameInDiffNamespaces() {
	ns1 := suite.mockUtils.Specs.Namespace
	ns2 := suite.mockUtils.Specs.Namespace + "-1"
	suite.makeFakeVG(ctx, label, vgName, ns1, "Retain", nil)
	suite.makeFakeVG(ctx, label, vgName, ns2, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, ns1, fakePvName1)
	suite.makeFakePVC(ctx, label, fakePvcName1, ns2, fakePvName1)
	suite.runFakeVGManager(vgName, ns1, "")
	suite.runFakeVGManager(vgName, ns2, "")
}

// test a failed reconcile with providing both pvc label and list
func (suite *VGSControllerTestSuite) TestProvidingLabelAndList() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Delete", make([]string, 2))
	suite.makeFakeVSC(ctx)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "PvcLabel and PvcList can't be provided at the same time.")
}

// test a successful reconcile with setup
func (suite *VGSControllerTestSuite) TestSetupManager() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManagerSetup(vgName, "")
}

// test reconcile without a VG and error should be volumegroup not found
func (suite *VGSControllerTestSuite) TestVgNotFound() {
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "DellCsiVolumeGroupSnapshot.volumegroup.storage.dell.com \"vg-snap\" not found")
}

// test reconcile without a VSC and result should be vsc not found
func (suite *VGSControllerTestSuite) TestVscNotFound() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "VolumeSnapshotClass.snapshot.storage.k8s.io \"vsc\" not found")
}

// test getSourceVolIDs with with non-empty pvcLabel but no matching pvc. pvcList should be empty and return error
func (suite *VGSControllerTestSuite) TestNoMatchingPVC() {
	noMatchingLbl := "noMatchingLbl"
	suite.makeFakeVG(ctx, noMatchingLbl, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, fmt.Sprintf("VG Snapshotter vg create failed, pvc with label missing %s", noMatchingLbl))
}

// test getSourceVolIDs where PVC is found, but there is no corresponding PV
func (suite *VGSControllerTestSuite) TestNoPvFoundInGetSourceVolIDs() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "VG Snapshotter vg create failed, unable to find pv for")
}

// test getSourceVolIDs where two PVCs are created, but corresponding PVs have different system ID
func (suite *VGSControllerTestSuite) TestDifferentSystemPV() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePV(ctx, fakePvName2, wrongSrcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.makeFakePVC(ctx, label, fakePvcName2, suite.mockUtils.Specs.Namespace, fakePvName2)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "VG Snapshotter vg create failed, VG Snapshotter systemIDs are different")
}

// test getSourceVolIDs where two PVCs are created, but PVC is not Bound to PV
func (suite *VGSControllerTestSuite) TestPVCNotBound() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVCNotBound(ctx, label, fakePvcName1, fakePvName1)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "VG Snapshotter matching pvc label with pv failed")
}

// test grpc call error to driver
func (suite *VGSControllerTestSuite) TestCsiDriverCreateFail() {
	suite.makeFakeVG(ctx, label, vgNameTestErr, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager(vgNameTestErr, suite.mockUtils.Specs.Namespace, "deliberately return error to test csi driver extension grpc error")
}

// Probe driver name to verify
func (suite *VGSControllerTestSuite) TestDriverVerification() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSCDriver(ctx)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "VG Snapshotter vg create failed, VolumeSnapshotClass driver name does not match volumegroupsnapshotter")
}

// test where snapshotName is reused
func (suite *VGSControllerTestSuite) TestReuseSnapName() {
	suite.makeFakeVG(ctx, label, "reuse-vg-snap", suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager("reuse-vg-snap", suite.mockUtils.Specs.Namespace, "")
	suite.runFakeVGManager("reuse-vg-snap", suite.mockUtils.Specs.Namespace, "exits")
}

func (suite *VGSControllerTestSuite) TestReuseVolSnapAndVolSnapContent() {
	suite.makeFakeVG(ctx, label, "reuse-vg-snap", suite.mockUtils.Specs.Namespace, "Delete", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager("reuse-vg-snap", suite.mockUtils.Specs.Namespace, "")
	suite.deleteVGForReUse("reuse-vg-snap", "")
	suite.runFakeVGManager("reuse-vg-snap", suite.mockUtils.Specs.Namespace, "")
}

func (suite *VGSControllerTestSuite) TestReuseVolSnapContentError() {
	suite.makeFakeVG(ctx, label, "reuse-vg-snap", suite.mockUtils.Specs.Namespace, "Delete", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager("reuse-vg-snap", suite.mockUtils.Specs.Namespace, "")
	suite.deleteVGForReUse("reuse-vg-snap", "contentError")
	suite.runFakeVGManager("reuse-vg-snap", suite.mockUtils.Specs.Namespace, "wrongClass")
}

func (suite *VGSControllerTestSuite) TestReuseVolSnapError() {
	suite.makeFakeVG(ctx, label, "reuse-vg-snap", suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager("reuse-vg-snap", suite.mockUtils.Specs.Namespace, "")
	suite.deleteVGForReUse("reuse-vg-snap", "volSnapError")
	suite.runFakeVGManager("reuse-vg-snap", suite.mockUtils.Specs.Namespace, "volumesnapshot exits with different")
}

// test idempotent case
func (suite *VGSControllerTestSuite) TestIdempotentCase() {
	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
}

// test getSourceVolIDsFromPvcNames
func (suite *VGSControllerTestSuite) TestGetSourceVolIDsFromPvcNames() {
	pvcNames := []string{fakePvcName1}
	suite.makeFakeVG(ctx, "", vgName, suite.mockUtils.Specs.Namespace, "Retain", pvcNames)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
}

// test getSourceVolIDsFromPvcNames with no such name pvc
func (suite *VGSControllerTestSuite) TestGetSourceVolIDsFromPvcNamesWithNoName() {
	pvcNames := []string{fakePvcName1}
	suite.makeFakeVSC(ctx)
	suite.makeFakeVG(ctx, "", vgName, suite.mockUtils.Specs.Namespace, "Retain", pvcNames)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "VG Snapshotter vg create failed, unable to find pvc for pvc Name")
}

// test getSourceVolIDsFromNs
func (suite *VGSControllerTestSuite) TestGetSourceVolIDsFromNs() {
	suite.makeFakeVG(ctx, "", vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "")
}

// test getSourceVolIDsFromNs with no pvc under ns
func (suite *VGSControllerTestSuite) TestGetSourceVolIDsFromNsEmptyPvcUnderNs() {
	suite.makeFakeVG(ctx, "", vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.runFakeVGManager(vgName, suite.mockUtils.Specs.Namespace, "VG Snapshotter vg create failed, no pvc found under ns")
}

// test bad reference
func (suite *VGSControllerTestSuite) TestReconcileBadRef() {

	key := fake_client.StorageKey{
		Name:      fakePvcName1,
		Namespace: "fake-ns",
		Kind:      "PersistentVolumeClaim",
	}

	suite.makeFakeVG(ctx, label, vgName, suite.mockUtils.Specs.Namespace, "Retain", nil)
	suite.makeFakeVSC(ctx)
	suite.makeFakePV(ctx, fakePvName1, srcVolID)
	suite.makeFakePVC(ctx, label, fakePvcName1, suite.mockUtils.Specs.Namespace, fakePvName1)
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

func (suite *VGSControllerTestSuite) makeFakePV(ctx context.Context, name, localSrcVolID string) {
	volumeAttributes := map[string]string{
		"StoragePool": "pool1",
	}

	pvObj := common.MakePV(name, suite.driver.DriverName, localSrcVolID, scName, volumeAttributes)
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

func (suite *VGSControllerTestSuite) makeFakePVC(ctx context.Context, lbl, name, namespace, volumeName string) {
	lbls := labels.Set{
		"name": lbl,
	}
	ns := namespace
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

func (suite *VGSControllerTestSuite) makeFakeVSCDriver(ctx context.Context) {
	vsc := common.MakeVSC(vscName, drivernamerr)
	err := suite.mockUtils.FakeClient.Create(ctx, &vsc)
	assert.Nil(suite.T(), err)
}

func (suite *VGSControllerTestSuite) makeFakeVG(ctx context.Context, localPvcLabel, localVgName, ns string, reclaimPolicy v1alpha2.MemberReclaimPolicy, localPvcList []string) {
	volumeGroup := common.MakeVG(localVgName, ns, suite.driver.DriverName, localPvcLabel, vscName, reclaimPolicy, localPvcList)
	err := suite.mockUtils.FakeClient.Create(ctx, &volumeGroup)
	assert.Nil(suite.T(), err)
}

func (suite *VGSControllerTestSuite) deleteFakeVG(ctx context.Context, localVgName string, setDeletionTimeStamp bool) {
	ns := suite.mockUtils.Specs.Namespace
	vg := new(v1alpha2.DellCsiVolumeGroupSnapshot)
	suite.mockUtils.FakeClient.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      localVgName,
	}, vg)

	// set timestamp to hit delete implemention
	if setDeletionTimeStamp {
		suite.mockUtils.FakeClient.SetDeletionTimeStamp(ctx, vg)
	}

	suite.mockUtils.FakeClient.Delete(ctx, vg)
}

func (suite *VGSControllerTestSuite) runFakeVGManagerSetup(localVgName, expectedErr string) {
	csiConn, err := connection.Connect(port)
	if err != nil {
		fmt.Printf("failed to connect to CSI driver, err: %#v", err)
		os.Exit(1)
	}

	opts := zap.Options{
		Development: true,
	}
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	fakeRecorder := record.NewFakeRecorder(100)
	// make a Reconciler object with grpc client
	// notice Client is set to fake client: the k8s mock
	vgReconcile := &DellCsiVolumeGroupSnapshotReconciler{
		Client:        suite.mockUtils.FakeClient,
		Log:           ctrl.Log.WithName("controllers").WithName("unit-test"),
		EventRecorder: fakeRecorder,
		Scheme:        common.Scheme,
		VGClient:      csiclient.New(csiConn, ctrl.Log.WithName("volumegroup-client"), 100*time.Second),
		DriverName:    common.DriverName,
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

	mgr, _ := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             runtime.NewScheme(),
		MetricsBindAddress: ":8080",
		Port:               9443,
		LeaderElection:     false,
		LeaderElectionID:   pkg_common.DellCSIVolumegroup,
	})

	expRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Minute)

	vgReconcile.SetupWithManager(mgr, expRateLimiter, 5)

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

func (suite *VGSControllerTestSuite) deleteVGForReUse(localVgName string, induceError string) {

	// find existing vg , get volsnap , get volsnapcotent

	vg := &v1alpha2.DellCsiVolumeGroupSnapshot{}
	nameSpacedName := types.NamespacedName{
		Namespace: suite.mockUtils.Specs.Namespace,
		Name:      localVgName,
	}
	suite.mockUtils.FakeClient.Get(ctx, nameSpacedName, vg)
	snaps := vg.Status.Snapshots

	for _, snap := range strings.Split(snaps, ",") {
		fmt.Printf("fake snap created  %s \n", snap)
		vs := new(s1.VolumeSnapshot)
		nameSpacedName.Name = snap

		suite.mockUtils.FakeClient.Get(ctx, nameSpacedName, vs)
		fmt.Printf("fake client found volsnap %s\n", vs.Name)

		// update vol snap or content and call reconcile again
		vcList := &s1.VolumeSnapshotContentList{}
		suite.mockUtils.FakeClient.List(context.Background(), vcList, nil)
		log.Info("VG Snapshotter", "volumesnapshotcontent count ", len(vcList.Items))

		vc := &s1.VolumeSnapshotContent{}
		for _, c := range vcList.Items {
			fmt.Printf("fake client found content  ReadyToUse %#v \n", *c.Status.ReadyToUse)
			vc = &c
		}
		*vc.Spec.Source.SnapshotHandle = "badid"
		if induceError == "contentError" {
			vc.Spec.Driver = "wrongDriver"
			vc.Spec.DeletionPolicy = s1.VolumeSnapshotContentRetain
			*vc.Spec.VolumeSnapshotClassName = "wrongClass"
		}

		suite.mockUtils.FakeClient.Update(context.Background(), vc)
		if induceError == "volSnapError" {
			class := "wrongClass"
			vs.Spec.VolumeSnapshotClassName = &class
			bound := "badboundContent"
			status := false
			vs.Status = &s1.VolumeSnapshotStatus{
				BoundVolumeSnapshotContentName: &bound,
				ReadyToUse:                     &status,
			}
			suite.mockUtils.FakeClient.Update(context.Background(), vs)
		}
	}

	// delete existing vg
	suite.mockUtils.FakeClient.Delete(context.Background(), vg)

	// create vg again expect volsnap and content to be reused
	suite.makeFakeVG(ctx, label, "reuse-vg-snap", suite.mockUtils.Specs.Namespace, "Retain", nil)
}

func (suite *VGSControllerTestSuite) runFakeVGManager(localVgName, namespace, expectedErr string) {
	csiConn, err := connection.Connect(port)
	if err != nil {
		fmt.Printf("failed to connect to CSI driver, err: %#v", err)
		os.Exit(1)
	}

	opts := zap.Options{
		Development: true,
	}
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	fakeRecorder := record.NewFakeRecorder(100)
	// make a Reconciler object with grpc client
	// notice Client is set to fake client: the k8s mock
	vgReconcile := &DellCsiVolumeGroupSnapshotReconciler{
		Client:        suite.mockUtils.FakeClient,
		Log:           ctrl.Log.WithName("controllers").WithName("unit-test"),
		EventRecorder: fakeRecorder,
		Scheme:        common.Scheme,
		VGClient:      csiclient.New(csiConn, ctrl.Log.WithName("volumegroup-client"), 100*time.Second),
		DriverName:    common.DriverName,
	}

	// make a request object to pass to Reconcile method in controller
	// this does not contain any k8s object , using the name passed in
	// Reconcile can select object from fake-client in memory objects
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      localVgName,
		},
	}

	// invoke controller Reconcile to test. typically k8s would call this when resource is changed
	res, err := vgReconcile.Reconcile(context.Background(), req)

	fmt.Printf("reconcile response res=%#v\n", res)
	if err != nil {
		fmt.Printf("reconcile response err=%s\n", err.Error())
	}

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
