package integration_testvg

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	storagev1alpha2 "github.com/dell/dell-csi-volumegroup-snapshotter/api/v1alpha2"
	controller "github.com/dell/dell-csi-volumegroup-snapshotter/controllers"
	"github.com/dell/dell-csi-volumegroup-snapshotter/pkg/connection"
	csiclient "github.com/dell/dell-csi-volumegroup-snapshotter/pkg/csiclient"
	"github.com/dell/dell-csi-volumegroup-snapshotter/test/shared/common"
	fake_client "github.com/dell/dell-csi-volumegroup-snapshotter/test/shared/fake-client"
	core_v1 "k8s.io/api/core/v1"

	s1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/cucumber/godog"
	"github.com/golang/protobuf/ptypes"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	ns                = "helmtest-vxflexos"
	scname            = "fake-sc"
	setlabel          = "vg-int-snap-label"
	vgname            = "vg-int-snap"
	vscname           = "vxflexos-snapclass"
	PVC_NAME_PREFIX   = "vg-int-pvc"
	PV_NAME_PREFIX    = "vg-int-pv"
	reconcile_vgname  string
	LABEL_ERROR       bool
	NOPV_ERROR        bool
	NOVSC_ERROR       bool
	DRIVER_NAME_ERROR bool
	CREATE_VC_ERROR   bool
	CREATE_VS_ERROR   bool
	UPDATE_VC_ERROR   bool
	UPDATE_VG_ERROR   bool
	// driver client to pass to controller
	csiConn *grpc.ClientConn
)

// client used by test code to make csi calls to driver
var driverClient csi.ControllerClient

// zap logger
var testLog = logf.Log.WithName("int-test")

var fakeRecorder = record.NewFakeRecorder(100)

type FakeVGTestSuite struct {
	errs                []error
	driverName          string
	mockUtils           *fake_client.MockUtils
	capability          *csi.VolumeCapability
	createVolumeRequest *csi.CreateVolumeRequest
	volID               string
	srcVolIDs           []string
	srcVolNames         []string
	VolCount            int
	arrays              map[string]*ArrayConnectionData
	anotherSystemID     string
}

// blank assignment to verify client.Client method implementations
var _ client.Client = &fake_client.Client{}

// expected errors
func (suite *FakeVGTestSuite) addError(err error) {
	suite.errs = append(suite.errs, err)
}

func (suite *FakeVGTestSuite) thereAreNoErrors() error {
	if len(suite.errs) == 0 {
		return nil
	}
	return suite.errs[0]
}

// gherkin steps for feature
func VGFeatureContext(s *godog.ScenarioContext) {
	suite := &FakeVGTestSuite{}
	s.Step(`^a Vgs Controller$`, suite.aVgsController)
	s.Step(`^I Call Clean up Volumes On Array$`, suite.CleanupVolsOnArray)
	s.Step(`^I Call Test Create VG$`, suite.iCallTestCreateVG)
	s.Step(`^I Call Test Reconcile Error VG For "([^"]*)"$`, suite.iCallTestReconcileErrorVGFor)
	s.Step(`^I Call Test Delete VG$`, suite.iCallTestDeleteVG)
	s.Step(`^I Call Test Create VG With BadVsc$`, suite.iCallTestCreateVGWithBadVsc)
	s.Step(`^There are No Errors$`, suite.thereAreNoErrors)
	s.Step(`^I Call Create (\d+) Volumes "([^"]*)" "(\d+)"$`, suite.iCallCreateVolumes)
	s.Step(`^I Force PVC Label Error "([^"]*)"$`, suite.iForcePVCLabelError)
	s.Step(`^I Set VG name "([^"]*)"$`, suite.iSetVGName)
	s.Step(`^I Set Namespace "([^"]*)"$`, suite.iSetNSName)
	s.Step(`^I Set PVC Label "([^"]*)"$`, suite.iSetPVCLabel)
	s.Step(`^I Force NoPV Error "([^"]*)"$`, suite.iForceNoPVError)
	s.Step(`^I Force NoVSC Error "([^"]*)"$`, suite.iForceNoVSCError)
	s.Step(`^I Force Driver Error "([^"]*)"$`, suite.iForceDriverError)
	s.Step(`^I Force Bad VG Error "([^"]*)"$`, suite.iForceBadVGError)
	s.Step(`^I Force Create VC Error "([^"]*)"$`, suite.iForceCreateVCError)
	s.Step(`^I Force Create VS Error "([^"]*)"$`, suite.iForceCreateVSError)
	s.Step(`^I Force Update VC Error "([^"]*)"$`, suite.iForceUpdateVCError)
	s.Step(`^I Force Update VG Error "([^"]*)"$`, suite.iForceUpdateVGError)
	s.Step(`^I Set Another SystemID "([^"]*)"$`, suite.iSetAnotherSystemID)
	s.Step(`^The Error Message Should Contain "([^"]*)"$`, suite.theErrorMessageShouldContain)

}

// rune once to initialize
func (suite *FakeVGTestSuite) BeforeTestSuite() {
	testLog.Info("Onetime init for Feature...")
	opts := zap.Options{
		Development:     true,
		Level:           zapcore.InfoLevel,
		StacktraceLevel: zapcore.PanicLevel,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	logf.SetLogger(logger)
}

// new in godog : call before and after all scenarios
func CleanupTestSuite(s *godog.TestSuiteContext) {
	suite := &FakeVGTestSuite{}
	s.BeforeSuite(func() {
		suite.BeforeTestSuite()
	})
	s.AfterSuite(func() {
		suite.TearDownTestSuite()
	})
}

// force error methods
func (suite *FakeVGTestSuite) iForceUpdateVGError(value string) error {
	testLog.Info("force VG update error")
	reconcile_vgname = value[:13]
	UPDATE_VG_ERROR = true
	return nil
}

func (suite *FakeVGTestSuite) iForceUpdateVCError(value string) error {
	testLog.Info("force VS update error")
	reconcile_vgname = value[:13]
	UPDATE_VC_ERROR = true
	return nil
}

func (suite *FakeVGTestSuite) iForceCreateVSError(value string) error {
	testLog.Info("force VS create error")
	reconcile_vgname = value[:13]
	CREATE_VS_ERROR = true
	return nil
}

func (suite *FakeVGTestSuite) iForceCreateVCError(value string) error {
	testLog.Info("force VC create error")
	reconcile_vgname = value[:13]
	CREATE_VC_ERROR = true
	return nil
}

func (suite *FakeVGTestSuite) iForceBadVGError(value string) error {
	testLog.Info("force bad vg error", "vg name", value)
	reconcile_vgname = ""
	DRIVER_NAME_ERROR = true
	return nil
}

func (suite *FakeVGTestSuite) iForceDriverError(value string) error {
	testLog.Info("force driver name error", "name", value)
	if strings.Contains(value, "no-vg-name") {
		reconcile_vgname = ""
	}
	DRIVER_NAME_ERROR = true
	return nil
}

func (suite *FakeVGTestSuite) iForceNoVSCError(value string) error {
	testLog.Info("force no vsc error", "vsc", value)
	vgname = value[:13]
	NOVSC_ERROR = true
	return nil
}

func (suite *FakeVGTestSuite) iForceNoPVError(value string) error {
	testLog.Info("force no pv error", "pv", value)
	// vgname = value
	NOPV_ERROR = true
	return nil
}

func (suite *FakeVGTestSuite) iSetVGName(value string) error {
	testLog.Info("set vgname", "vg", value)
	reconcile_vgname = value
	return nil
}

func (suite *FakeVGTestSuite) iSetPVCLabel(value string) error {
	testLog.Info("set label", "label", value)
	setlabel = value
	return nil
}

func (suite *FakeVGTestSuite) iForcePVCLabelError(value string) error {
	testLog.Info("force label error", "label", value)
	vgname = value[:13]
	LABEL_ERROR = true
	return nil
}

// iSetNSName
func (suite *FakeVGTestSuite) iSetNSName(value string) error {
	testLog.Info("namespace", "ns", value)
	ns = value
	return nil
}

func (suite *FakeVGTestSuite) CleanupVolsOnArray() error {

	//  array cleanup
	testLog.Info("cleanup volumes")
	for _, srcId := range suite.srcVolIDs {
		testLog.Info("cleanup volume", "id", srcId)
		for {
			snapID, _ := suite.iCallGetSnapshot(srcId)
			if snapID != "" {
				_ = suite.iCallDeleteSnapshot(snapID)
			} else {
				_ = suite.iCallDeleteVolume(srcId)
				break
			}
		}
	}
	return nil
}

// cleanup by calling driver
func (suite *FakeVGTestSuite) iCallDeleteVolume(srcId string) error {
	ctx := context.Background()
	delVolReq := new(csi.DeleteVolumeRequest)
	delVolReq.VolumeId = srcId
	_, err := driverClient.DeleteVolume(ctx, delVolReq)

	if err != nil {
		testLog.Error(err, "DeleteVolume failed")
		return err
	} else {
		testLog.Info("DeleteVolume completed successfully", "volid", srcId)
	}
	return nil
}

//make volumes to test with
func (suite *FakeVGTestSuite) iCallCreateVolumes(count int, vname string, size int64) error {
	ctx := context.Background()
	suite.VolCount = count
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf(vname+"%d", i)
		req := suite.aBasicBlockVolumeRequest(name, 8)

		volResp, err := driverClient.CreateVolume(ctx, req)
		if err != nil {
			testLog.Error(err, "CreateVolume failed")
			suite.addError(err)
		} else {
			testLog.Info("CreateVolume", volResp.GetVolume().VolumeContext["Name"],
				volResp.GetVolume().VolumeId)
			suite.volID = volResp.GetVolume().VolumeId
			suite.srcVolIDs = append(suite.srcVolIDs, suite.volID)
		}
	}
	return nil
}

func (suite *FakeVGTestSuite) iSetAnotherSystemID(systemType string) error {

	if suite.arrays == nil {
		testLog.Info("Initialize ArrayConfig from", "file", configFile)
		var err error
		suite.arrays, err = getArrayConfig()
		if err != nil {
			return errors.New("Get multi array config failed " + err.Error())
		}
	}
	for _, a := range suite.arrays {
		if systemType == "altSystem" && !a.IsDefault {
			suite.anotherSystemID = a.SystemID
			break
		}
		if systemType == "defaultSystem" && a.IsDefault {
			suite.anotherSystemID = a.SystemID
			break
		}
	}
	testLog.Info("array selected for", systemType, suite.anotherSystemID)
	if suite.anotherSystemID == "" {
		return errors.New("Failed to get  multi array config for " + systemType)
	}
	return nil
}

// copy from driver int-test to make a volume
func (suite *FakeVGTestSuite) aBasicBlockVolumeRequest(name string, size int64) *csi.CreateVolumeRequest {
	req := new(csi.CreateVolumeRequest)
	params := make(map[string]string)
	params["storagepool"] = "pool1"
	params["thickprovisioning"] = "false"
	if len(suite.anotherSystemID) > 0 {
		params["systemID"] = suite.anotherSystemID
	}
	req.Parameters = params
	req.Name = name
	capacityRange := new(csi.CapacityRange)
	capacityRange.RequiredBytes = size * 1024 * 1024 * 1024
	req.CapacityRange = capacityRange
	capability := new(csi.VolumeCapability)
	block := new(csi.VolumeCapability_BlockVolume)
	blockType := new(csi.VolumeCapability_Block)
	blockType.Block = block
	capability.AccessType = blockType
	accessMode := new(csi.VolumeCapability_AccessMode)
	accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	capability.AccessMode = accessMode
	suite.capability = new(csi.VolumeCapability)
	capabilities := make([]*csi.VolumeCapability, 0)
	capabilities = append(capabilities, capability)
	req.VolumeCapabilities = capabilities
	suite.createVolumeRequest = req
	return req
}

func (suite *FakeVGTestSuite) theErrorMessageShouldContain(expected string) error {
	// If arg1 is none, we expect no error, any error received is unexpected

	if expected == "none" {
		if len(suite.errs) == 0 {
			return nil
		} else {
			return fmt.Errorf("unexpected error(s): %s", suite.errs[0])
		}
	}
	// We expect an error...
	if len(suite.errs) == 0 {
		return errors.New("there were no errors but we expected: " + expected)
	}
	var FOUND_ERROR = false
	for _, err := range suite.errs {
		testLog.V(1).Info("need controller error message", "msg=", expected)

		gotError := regexp.MustCompile(`\"(.*)\"`).ReplaceAllString(err.Error(), `$1`)

		testLog.V(1).Info("got controller error message", "msg=", gotError)

		if strings.Contains(gotError, expected) {
			testLog.Info("expected error message found", "msg=", err.Error())
			FOUND_ERROR = true
		}
	}
	if !FOUND_ERROR {
		testLog.Error(nil, "expected error message not found")
		return fmt.Errorf("error %s expected message not found", expected)
	}
	suite.errs = nil
	return nil
}

// refer fake_client.go , force calls to k8s to return error during controller error handling

func (suite *FakeVGTestSuite) ShouldFail(method string, obj runtime.Object) error {
	switch v := obj.(type) {
	case *storagev1alpha2.DellCsiVolumeGroupSnapshot:
		vg := obj.(*storagev1alpha2.DellCsiVolumeGroupSnapshot)
		if method == "Update" && UPDATE_VG_ERROR {
			testLog.Info("ShouldFail", "force vg error", vg.Name)
			testLog.Info("ShouldFail", "force update vg error", v)
			return errors.New("unable to update VG")
		}
	case *s1.VolumeSnapshot:
		vs := obj.(*s1.VolumeSnapshot)
		if method == "Create" && CREATE_VS_ERROR {
			if strings.Contains(vs.Name, "-1-") {
				testLog.Info("ShouldFail", "force vs error", vs.Name)
				testLog.Info("ShouldFail", "force create vs error", v)
				return errors.New("unable to create Volsnap")
			}
		}
	case *s1.VolumeSnapshotContent:
		vsc := obj.(*s1.VolumeSnapshotContent)
		if method == "Create" && CREATE_VC_ERROR {
			snapshotName := vsc.Spec.VolumeSnapshotRef.Name
			if strings.Contains(snapshotName, "-1-") {
				testLog.Info("ShouldFail", "force vsc error", v)
				testLog.Info("ShouldFail", "force vsc error", vsc.Name)
				return errors.New("unable to create VolsnapContent")
			}
		} else if method == "Update" && UPDATE_VC_ERROR {
			testLog.Info("ShouldFail", "force update vs error", v)
			return errors.New("unable to update VolsnapContent")
		}
	default:
	}
	return nil
}

// Given step implements startup csi grpc client , fake client
func (suite *FakeVGTestSuite) aVgsController() error {

	testLog.Info("Init called", "test=", "")

	_ = storagev1alpha2.AddToScheme(scheme.Scheme)
	_ = s1.AddToScheme(scheme.Scheme)

	var obj []runtime.Object
	c, _ := fake_client.NewFakeClient(obj, suite)

	suite.mockUtils = &fake_client.MockUtils{
		FakeClient: c,
		Specs:      common.Common{Namespace: ns},
	}
	suite.driverName = common.DriverName

	// unix://./unix_sock
	var err error
	csiConn, err = connection.Connect(sock)
	if err != nil {
		testLog.Error(err, "failed to connect to CSI driver, make sure start_server.sh has kicked off server")
		os.Exit(1)
	}
	// used to call driver controller.go methods to verify or cleanup powerflex array snapshots
	driverClient = csi.NewControllerClient(csiConn)
	suite.srcVolNames = make([]string, 0)
	suite.errs = make([]error, 0)
	reconcile_vgname = vgname

	LABEL_ERROR = false
	NOPV_ERROR = false
	NOVSC_ERROR = false
	DRIVER_NAME_ERROR = false
	CREATE_VC_ERROR = false
	CREATE_VS_ERROR = false
	UPDATE_VC_ERROR = false
	UPDATE_VG_ERROR = false

	// set default count to 2
	suite.VolCount = 2
	suite.srcVolIDs = make([]string, 0)
	testLog.Info("Init VG controller done", "driverClient", driverClient)

	return nil
}

// call reconcile to test vg snapshotter
func (suite *FakeVGTestSuite) runVGReconcile() error {

	// make a Reconciler object with grpc client
	// notice Client is set to fake client :the k8s mock
	vgReconcile := &controller.DellCsiVolumeGroupSnapshotReconciler{
		Client:        suite.mockUtils.FakeClient,
		Log:           logf.Log.WithName("vg-controller"),
		Scheme:        common.Scheme,
		EventRecorder: fakeRecorder,
		VGClient:      csiclient.New(csiConn, ctrl.Log.WithName("volumegroup-client"), 100*time.Second),
		DriverName:    common.DriverName,
	}

	// make a request object to pass to Reconcile method in controller
	// this does not contain any k8s object , using the name passed in Reconcile can select
	// object from fake-client in memory objects
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: ns,
			Name:      reconcile_vgname,
		},
	}

	// invoke controller Reconcile to test. typically k8s would call this when resource is changed
	_, err := vgReconcile.Reconcile(context.Background(), req)
	if err != nil {
		suite.addError(err)
		return err
	}
	return nil

}

func (suite *FakeVGTestSuite) iCallTestDeleteVG() error {
	vg := &storagev1alpha2.DellCsiVolumeGroupSnapshot{}
	ctx := context.Background()

	if err := suite.mockUtils.FakeClient.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      vgname,
	}, vg); err != nil {
		testLog.Error(err, "vg not found", "name", vgname, "ns", ns)
		suite.addError(err)
	}

	testLog.Info("vg got from icalltestdeleteVG is", "vg", vg)

	suite.mockUtils.FakeClient.SetDeletionTimeStamp(ctx, vg)
	suite.mockUtils.FakeClient.Delete(ctx, vg)

	if err := suite.runVGReconcile(); err != nil {
		suite.addError(err)
	}

	return nil
}

// create pre-reqs and call reconcile
func (suite *FakeVGTestSuite) iCallTestReconcileErrorVGFor(errorType string) error {

	// make a k8s object and save in memory ,
	// Reconcile is called to update this object and we can verify
	// hence there is no need to have a k8s environment

	// user want to make a vg with this src volume

	// if vg exists delete it allowing us to rerun without error
	// if there is a snap on array then cleanup

	// pre-req volume snapshot class must exist
	_ = suite.makeFakeVSC()

	for _, srcId := range suite.srcVolIDs {

		// pre-req pv must  exist
		testLog.Info("Make Fake PV and PVC for", setlabel, srcId)

		_ = suite.makeFakePV(srcId)

		_ = suite.makeFakePVC(srcId)
	}

	// user makes a vg create request to controller to select pvc with this label
	_ = suite.makeFakeVG()

	// run the VG controller Reconcile
	if strings.Compare(errorType, "VS") == 0 {
		CREATE_VS_ERROR = true
	}
	if strings.Compare(errorType, "VC") == 0 {
		CREATE_VC_ERROR = true
	}
	err := suite.runVGReconcile()
	if err != nil {
		CREATE_VS_ERROR = false
		CREATE_VC_ERROR = false
		suite.errs = nil
		if err = suite.runVGReconcile(); err != nil {
			suite.addError(err)
		}
	} else {
		err = errors.New("Forced Reconcile Error did not occur")
		testLog.Error(err, reconcile_vgname)
		suite.addError(err)
	}
	if len(suite.errs) == 0 {
		return suite.verify()
	}
	_ = suite.CleanupVolsOnArray()

	return nil
}

// create pre-reqs and call reconcile
func (suite *FakeVGTestSuite) iCallTestCreateVG() error {

	// make a k8s object and save in memory ,
	// Reconcile is called to update this object and we can verify
	// hence there is no need to have a k8s environment

	// user want to make a vg with this src volume

	// if vg exists delete it allowing us to rerun without error
	// if there is a snap on array then cleanup

	// pre-req volume snapshot class must exist
	_ = suite.makeFakeVSC()

	for _, srcId := range suite.srcVolIDs {

		// pre-req pv must  exist
		testLog.Info("Make Fake PV and PVC for", setlabel, srcId)

		_ = suite.makeFakePV(srcId)

		_ = suite.makeFakePVC(srcId)
	}

	// user makes a vg create request to controller to select pvc with this label
	_ = suite.makeFakeVG()

	// run the VG controller Reconcile
	err := suite.runVGReconcile()
	if err != nil {
		suite.addError(err)
		_ = suite.CleanupVolsOnArray()
	}
	if len(suite.errs) == 0 {
		return suite.verify()
	}

	return nil
}

// create pre-reqs and call reconcile
func (suite *FakeVGTestSuite) iCallTestCreateVGWithBadVsc() error {

	// make a k8s object and save in memory ,
	// Reconcile is called to update this object and we can verify
	// hence there is no need to have a k8s environment

	// user want to make a vg with this src volume

	// if vg exists delete it allowing us to rerun without error
	// if there is a snap on array then cleanup

	//pre-req volume snapshot class must exist
	_ = suite.makeBadVSC("red-rrr")

	for _, srcId := range suite.srcVolIDs {

		// pre-req pv must  exist
		testLog.Info("Make Fake PV and PVC for", setlabel, srcId)

		_ = suite.makeFakePV(srcId)

		_ = suite.makeFakePVC(srcId)
	}

	// user makes a vg create request to controller to select pvc with this label
	_ = suite.makeFakeVG()

	// run the VG controller Reconcile
	err := suite.runVGReconcile()
	if err != nil {
		suite.addError(err)
		_ = suite.CleanupVolsOnArray()
	}
	if len(suite.errs) == 0 {
		return suite.verify()
	}

	return nil
}

func (suite *FakeVGTestSuite) verifyLabel(obj runtime.Object, key string) error {
	var err error
	vsnap := obj.(*s1.VolumeSnapshot)
	testLog.V(1).Info("found fake object ", "Name", vsnap.Name)
	testLog.V(1).Info("found fake object ", "Labels", vsnap.Labels)
	//vsnap.ObjectMeta.Name snap.ObjectMeta.Labels
	lbs := vsnap.ObjectMeta.Labels
	var EXPECTED_LABEL_FOUND bool
	for k, l := range lbs {
		testLog.Info("label match", k, l, "vg", vgname, "reconcile", reconcile_vgname)
		expect := vgname
		if reconcile_vgname != vgname {
			expect = reconcile_vgname
		}
		if k == "snapshotGroup" && l == expect {
			EXPECTED_LABEL_FOUND = true
		}
	}
	if !EXPECTED_LABEL_FOUND {
		err = errors.New("VolumeSnapshot does not have label key=snapshotGroup with proper value")
		testLog.Error(err, vsnap.Name)
		suite.addError(err)
	} else {
		testLog.Info("label matched ok for VolumeSnapshot=", vsnap.Name, vsnap.Labels)
	}
	return err

}

// verify VG object in fake-client has expected updates after reconcile runs
func (suite *FakeVGTestSuite) verify() error {

	// get the fake objects we created before reconcile
	objMap := suite.mockUtils.FakeClient.Objects
	var volGroup *storagev1alpha2.DellCsiVolumeGroupSnapshot

	event := <-fakeRecorder.Events
	fmt.Println("DEBUG event :", event)

	var FOUND_VG bool
	for k, v := range objMap {
		testLog.V(1).Info("found fake object ", k.Name, "created or updated by reconcile")
		// verify label
		if strings.HasPrefix(k.Name, vgname) && strings.Contains(k.Name, "-"+PVC_NAME_PREFIX) {
			// v is of type *s1.VolumeSnapshot
			// check label put by controller is valid
			if reconcile_vgname != vgname && strings.HasPrefix(k.Name, reconcile_vgname) {
				err := suite.verifyLabel(v, "snapshotGroup")
				if err != nil {
					return err
				}
			}
		}
		vg := vgname
		if reconcile_vgname != vgname {
			vg = reconcile_vgname
		}
		if k.Name == vg {
			// assert v is of desired type
			volGroup = v.(*storagev1alpha2.DellCsiVolumeGroupSnapshot)

			testLog.Info("found VG ", "name", volGroup.Name)

			if k.Name == volGroup.Name {
				FOUND_VG = true
				// todo verify
				// compare new changes  volGroup.Status.CreationTime
				// compare new changes  volGroup.Status.Snapshots

				var err error
				// "4d4a2e5a36080e0f-bab0f85900000002"
				if !regexp.MustCompile(`^([a-zA-Z0-9]*)-([a-zA-Z0-9]*)$`).MatchString(volGroup.Status.SnapshotGroupID) {
					err = errors.New("unable to find VG Group ID")
					testLog.Error(err, vg)
					suite.addError(err)
				} else {
					testLog.Info("found VG", " ID ", volGroup.Status.SnapshotGroupID)
				}
				//2021-05-12 10:20:17 -0400 EDT
				stime := volGroup.Status.CreationTime.String()
				if !regexp.MustCompile(`\d{4}-\d{1,2}-\d{1,2} \d{2}:\d{2}:\d{2} -\d{4} EDT`).MatchString(stime) {
					err = errors.New("unable to match time ")
					testLog.Error(err, volGroup.Status.CreationTime.String())
					suite.addError(err)
				} else {
					testLog.Info("found VG", " Time ", volGroup.Status.CreationTime)
				}
				// ReadyToUse status
				ready := volGroup.Status.ReadyToUse
				if !ready {
					err = errors.New("VG Snapshotter is not ready to use ")
					testLog.Error(err, vgname)
					suite.addError(err)
				} else {
					testLog.Info("found VG", "ReadyToUse", volGroup.Status.ReadyToUse)
				}

				// Pending, Error, Completed Status
				complete := volGroup.Status.Status
				testLog.Info("Found VG", "Status", volGroup.Status.Status)

				if complete != "Complete" {
					err = errors.New("VG Snapshotter is not completed ")
					testLog.Error(err, vgname)
					suite.addError(err)
				} else {
					testLog.Info("Found VG", "Completed", volGroup.Status.Status)
				}

				// "vg-snap-timestamp-0-fake-pvc-4d4a2e5a36080e0f-a234c09c00000003,vg-snap-timestamp-1-fake-pvc-4d4a2e5a36080e0f-a234c09d0000000f"
				snames := strings.Split(volGroup.Status.Snapshots, ",")
				for _, sn := range snames {

					// for example vg-snap-090121-152109-0-vg-int-pvc-... get 090121-152109-0
					re := regexp.MustCompile(`-(?P<sdigit>\d+-\d+-\d+)-.*`)
					matches := re.FindStringSubmatch(sn)
					dIndex := re.SubexpIndex("sdigit")
					testLog.V(1).Info("snap name regex match ", "index", matches[dIndex])

					snamePrefix := vg + "-" + matches[dIndex] + "-" + PVC_NAME_PREFIX

					if strings.Contains(sn, snamePrefix) {
						testLog.Info("snap name regex match ", "sname", snamePrefix)
					} else {
						testLog.Info("snap name regex does not match ", "snamePrefix", snamePrefix)
						err = fmt.Errorf("unable to find snap %s", sn)
						testLog.Error(err, sn)
						suite.addError(err)
					}
				}
			}
		}
	}
	if !FOUND_VG {
		testLog.Error(errors.New("unable to find VG"), vgname, reconcile_vgname)
	}
	return nil
}

func (suite *FakeVGTestSuite) debugFakeObjects() {
	objMap := suite.mockUtils.FakeClient.Objects
	if len(objMap) == 0 {
		testLog.V(1).Info("Objects are empty")
	}

	for k, v := range objMap {
		testLog.V(1).Info("found fake object ", "name", k.Name)
		testLog.V(1).Info("found fake object ", "object", fmt.Sprintf("%#v", v))
	}
}

func (suite *FakeVGTestSuite) makeFakeVG() error {

	getlabel := setlabel
	if LABEL_ERROR {
		testLog.Info("vg force label error")
		getlabel = "xxxxx"
		LABEL_ERROR = false

	}
	if NOVSC_ERROR {
		vscname = "no-vsc-for-vg"
	}
	// empty list to be filled by controller
	// snaps := make([]storagev1alpha2.SnapshotVolume, 0)

	// passing ids works , label on pvc also works

	volumeGroup := common.MakeVG(reconcile_vgname, ns, suite.driverName, getlabel, vscname, "Delete", nil)

	// make a k8s object and save in memory, Reconcile is called to update this object and this test can verify
	// hence there is no need to have a k8s environment

	ctx := context.Background()
	return suite.mockUtils.FakeClient.Create(ctx, &volumeGroup)
}

func (suite *FakeVGTestSuite) makeFakePVC(srcId string) error {

	pvcname := PVC_NAME_PREFIX + "-" + srcId
	lbls := labels.Set{
		"volume-group": setlabel,
	}
	pvname := PV_NAME_PREFIX + "-" + srcId
	pvcObj := common.MakePVC(pvcname, ns, scname, pvname, lbls)
	pvcObj.Status.Phase = core_v1.ClaimBound
	ctx := context.Background()
	suite.removeExistingObject(pvcname, ns, "pvc")
	err := suite.mockUtils.FakeClient.Create(ctx, &pvcObj)
	//suite.debugObjects()
	return err
}

func (suite *FakeVGTestSuite) makeFakeVSC() error {
	vsc := common.MakeVSC(vscname, "csi-vxflexos")
	ctx := context.Background()
	err := suite.mockUtils.FakeClient.Create(ctx, &vsc)
	return err
}

func (suite *FakeVGTestSuite) makeBadVSC(drivername string) error {
	vsc := common.MakeVSC(vscname, drivername)
	ctx := context.Background()
	err := suite.mockUtils.FakeClient.Create(ctx, &vsc)
	return err
}

func (suite *FakeVGTestSuite) makeFakePV(srcId string) error {

	pvname := PV_NAME_PREFIX + "-" + srcId
	if NOPV_ERROR {
		objMap := suite.mockUtils.FakeClient.Objects
		for k := range objMap {
			if k.Name == pvname {
				delete(objMap, k)
				testLog.Info("force error delete PV for this pvc ", "PV ", pvname)
			}
		}
		return nil
	}

	suite.srcVolNames = append(suite.srcVolNames, pvname)
	volumeAttributes := map[string]string{
		"fake-CapacityGB": "8.00",
		"StoragePool":     "pool1",
	}

	pvObj := common.MakePV(pvname, suite.driverName, srcId, scname, volumeAttributes)
	ctx := context.Background()
	suite.removeExistingObject(pvname, ns, "pv")
	err := suite.mockUtils.FakeClient.Create(ctx, &pvObj)
	//suite.debugObjects()
	return err
}

func (suite *FakeVGTestSuite) removeExistingObject(objName string, newNS string, objType string) bool {
	objMap := suite.mockUtils.FakeClient.Objects
	for k, v := range objMap {
		if k.Name == objName {
			if v != nil && objType == "pvc" {
				t := v.(*core_v1.PersistentVolumeClaim)
				existingNS := t.ObjectMeta.Namespace
				if existingNS != newNS {
					continue
				}
			}
			testLog.Info("remove current object ", objType, k.Name)
			delete(suite.mockUtils.FakeClient.Objects, k)
		}
	}
	return true
}

// cleanup  helper method to call powerflex csi driver
func (suite *FakeVGTestSuite) iCallDeleteSnapshot(snapID string) error {
	ctx := context.Background()
	req := &csi.DeleteSnapshotRequest{
		SnapshotId: snapID,
	}
	// call csi driver using grpc client
	_, err := driverClient.DeleteSnapshot(ctx, req)
	if err != nil {
		testLog.Error(err, "test DeleteSnapshot returned error")
		return err
	} else {
		testLog.Info("test cleanup DeleteSnapshot ok", "snap", req.SnapshotId)
	}
	return nil
}

// helper method to call powerflex csi driver
func (suite *FakeVGTestSuite) iCallGetSnapshot(srcId string) (string, error) {
	var err error
	ctx := context.Background()
	// SnapshotId: idToQuery
	req := &csi.ListSnapshotsRequest{SourceVolumeId: srcId}

	// call csi-driver to query powerflex array
	snaps, err := driverClient.ListSnapshots(ctx, req)
	if err != nil {
		return "", err
	}
	entries := snaps.GetEntries()
	for j := 0; j < len(entries); j++ {
		entry := entries[j]
		id := entry.GetSnapshot().SnapshotId
		ts := ptypes.TimestampString(entry.GetSnapshot().CreationTime)
		foundsrcID := entry.GetSnapshot().SourceVolumeId
		testLog.V(1).Info("look for volume ", srcId, foundsrcID)
		if id != "" && foundsrcID != "" && strings.Contains(srcId, foundsrcID) {
			testLog.Info("test found snapshot in powerflex ", "id:", id, "source ID", foundsrcID, "time", ts)
			return id, nil
		}
	}
	return "", nil
}

//run once per suite to remove array volumes
func (suite *FakeVGTestSuite) TearDownTestSuite() {
	testLog.Info("Cleaning up resources...")
	_ = suite.CleanupVolsOnArray()
	_ = csiConn.Close()
}
