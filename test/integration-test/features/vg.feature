Feature: VxFlex OS CSI interface
  As a consumer of the CSI interface
  I want to run a system test
  So that I know the service functions correctly.

@vg
Scenario: Call Create VolumesGroupSnapshot happy path
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-int" "8"
  When I Call Test Create VG 
  Then There are No Errors
  And I Call Clean up Volumes On Array

@vg
Scenario: Call Create VolumesGroupSnapshot happy path idempotent case
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-int1" "8"
  When I Call Test Create VG
  Then There are No Errors
  When I Call Test Create VG
  Then There are No Errors
  And I Call Clean up Volumes On Array

@vg
Scenario: Call Create VolumesGroupSnapshot happy path same label new vg name
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-int2" "8"
  When I Call Test Create VG
  Then There are No Errors
  And I Set VG name "vg-int-snap-two"
  And I Set PVC Label "vg-int-snap-label"
  When I Call Test Create VG
  Then There are No Errors
  And I Call Clean up Volumes On Array

@vg
Scenario: Call Create VolumesGroupSnapshot same label different namspace label Request is idempotent
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-int3" "8"
  And I Set VG name "vg-int-snap-three"
  When I Call Test Create VG
  Then There are No Errors
  And I Set Namespace "nstwo"
  When I Call Test Create VG
  And I Call Clean up Volumes On Array

@wip
Scenario: Call Create VolumesGroupSnapshot happy path same label different namspace pvc label missing
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-int4" "8"
  When I Call Test Create VG
  Then There are No Errors
  And I Set Namespace "nsthree"
  And I Force PVC Label Error "vg-int-wrong-label"
  When I Call Test Create VG
  Then The Error Message Should Contain "pvc with label missing"
  And I Call Clean up Volumes On Array

@vg
Scenario: Call Create VolumesGroupSnapshot happy path non default array
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Set Another SystemID "altSystem"
  And I Call Create 2 Volumes "vg-int5" "8"
  And I Set PVC Label "vg-int-snap-label"
  When I Call Test Create VG
  Then There are No Errors
  And I Call Clean up Volumes On Array


@vg
Scenario: Call Create VolumesGroupSnapshot volumes in different system with same label
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 1 Volumes "vg-1-int" "8"
  And I Set Another SystemID "altSystem"
  And I Call Create 1 Volumes "vg-2-int" "8"
  When I Call Test Create VG
  Then The Error Message Should Contain "systemIDs are different"
  And I Call Clean up Volumes On Array


@vg
Scenario: Call Create VolumesGroupSnapshot with no matching volumes
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-int6" "8"
  And I Force PVC Label Error "vg-int-wrong-label"
  When I Call Test Create VG
  Then The Error Message Should Contain "pvc with label missing"
  And I Call Clean up Volumes On Array

@vg
Scenario: Call Create VolumesGroupSnapshot with no matching pv for pvc
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-int7" "8"
  And I Force NoPV Error "vg-int-no-pv-for-pvc"
  When I Call Test Create VG
  Then The Error Message Should Contain "unable to find pv"
  And I Call Clean up Volumes On Array

@vg
Scenario: Call Create VolumesGroupSnapshot with no pre-req vsc for snapshots
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-int8" "8"
  And I Force NoVSC Error "vg-int-no-vsc-for-vg"
  When I Call Test Create VG
  Then The Error Message Should Contain "VolumeSnapshotClass.snapshot.storage.k8s.io no-vsc-for-vg not found"
  And I Call Clean up Volumes On Array

@vg
Scenario: Call Create VolumesGroupSnapshot k8s object has name but empty vgname sent in reconcile.
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-int9" "8"
  And I Force Driver Error "vg-int-no-vg-name"
  When I Call Test Create VG
  Then The Error Message Should Contain "CreateVolumeGroupSnapshotRequest needs Name to be set"
  And I Call Clean up Volumes On Array


@vg
Scenario: Call Create VolumesGroupSnapshot k8s object has bad vgname
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-int10" "8"
  And I Force Bad VG Error "-1"
  When I Call Test Create VG
  Then The Error Message Should Contain "CreateVolumeGroupSnapshotRequest needs Name to be set"
  And I Call Clean up Volumes On Array

@vg
Scenario: Call Create VolumesGroupSnapshot response process hit error during create VolumeSnapshotContent
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-vc" "8"
  And I Force Create VC Error "response-create-error-vsc"
  When I Call Test Create VG
  Then The Error Message Should Contain "unable to create VolsnapContent"
  And I Call Clean up Volumes On Array

@vg
Scenario: Call Create VolumesGroupSnapshot response process hit error during update vg
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-vc1" "8"
  And I Force Update VG Error "response-update-error-vg"
  When I Call Test Create VG
  Then The Error Message Should Contain "unable to update VG"
  And I Call Clean up Volumes On Array

@vg
Scenario: Call Create VolumesGroupSnapshot response process hit error during update status for VolumeSnapshotContent
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-vc2" "8"
  And I Force Update VC Error "response-update-error-vc"
  When I Call Test Create VG
  Then The Error Message Should Contain "unable to update VolsnapContent"
  And I Call Clean up Volumes On Array


@vp
Scenario: Call Create VolumesGroupSnapshot response process hit error during create VolumeSnapshot
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-vsc1" "8"
  And I Force Create VS Error "response-create-error-vs"
  When I Call Test Create VG
  Then The Error Message Should Contain "unable to create Volsnap"
  And I Call Clean up Volumes On Array
