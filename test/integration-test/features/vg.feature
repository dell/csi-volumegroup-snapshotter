Feature: VxFlex OS CSI interface
  As a consumer of the CSI interface
  I want to run a system test
  So that I know the service functions correctly.

Scenario: Call Create and Delete Volume happy path
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-int1" "8"
  When I Call Test Create VG
  Then There are No Errors
  When I Call Test Delete VG
  Then There are No Errors
  And I Call Clean up Volumes On Array

Scenario: Call Create and Delete Volume reconcile retry VS error
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-int2" "8"
  When I Call Test Reconcile Error VG For "VS"
  Then There are No Errors
  When I Call Test Delete VG
  Then There are No Errors
  And I Call Clean up Volumes On Array

Scenario: Call Create and Delete Volume reconcile retry VC error
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-int3" "8"
  When I Call Test Reconcile Error VG For "VC"
  Then There are No Errors
  When I Call Test Delete VG
  Then There are No Errors
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot happy path idempotent case
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-ok" "8"
  When I Call Test Create VG
  Then There are No Errors
  When I Call Test Create VG
  Then There are No Errors
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot happy path same label new vg name
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-label" "8"
  When I Call Test Create VG
  Then There are No Errors
  And I Set VG name "vg-int-two"
  And I Set PVC Label "vg-int-snap-label"
  When I Call Test Create VG
  Then There are No Errors
  And I Call Clean up Volumes On Array

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
   
Scenario: Call Create VolumesGroupSnapshot happy path same name different namspace
   Given a Vgs Controller
   And I Call Clean up Volumes On Array
   And I Set Namespace "nsone-test"
   And I Call Create 2 Volumes "vg-int4" "8"
   When I Call Test Create VG
   Then There are No Errors
   And I Set Namespace "nstwo-test"
   And I Call Create 2 Volumes "vg-int4" "8"
   When I Call Test Create VG
   Then There are No Errors

Scenario: Call Create VolumesGroupSnapshot same label different namspace label Request is idempotent
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 2 Volumes "vg-label-diff" "8"
  And I Set VG name "vg-int-three"
  When I Call Test Create VG
  Then There are No Errors
  And I Set Namespace "nstwo"
  When I Call Test Create VG
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot happy path non default array
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Set Another SystemID "altSystem"
  And I Call Create 2 Volumes "vg-int5" "8"
  And I Set PVC Label "vg-int-snap-label"
  When I Call Test Create VG
  Then There are No Errors
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot volumes in different system with same label
  Given a Vgs Controller
  And I Call Clean up Volumes On Array
  And I Call Create 1 Volumes "vg-1-int" "8"
  And I Set Another SystemID "altSystem"
  And I Call Create 1 Volumes "vg-2-int" "8"
  When I Call Test Create VG
  Then The Error Message Should Contain "systemIDs are different"
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot with no matching volumes
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-int6" "8"
  And I Force PVC Label Error "vg-int-wronglabel"
  When I Call Test Create VG
  Then The Error Message Should Contain "pvc with label missing"
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot with no matching pv for pvc
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-int7" "8"
  And I Force NoPV Error "vg-int-no-pv-for-pvc"
  When I Call Test Create VG
  Then The Error Message Should Contain "unable to find pv"
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot with no pre-req vsc for snapshots
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-int8" "8"
  And I Force NoVSC Error "vg-int-no-vsc-for-vg"
  When I Call Test Create VG
  Then The Error Message Should Contain "VolumeSnapshotClass.snapshot.storage.k8s.io no-vsc-for-vg not found"
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot response process hit error during create VolumeSnapshotContent
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-vc" "8"
  And I Force Create VC Error "response-create-error-vsc"
  When I Call Test Create VG
  Then The Error Message Should Contain "unable to create VolsnapContent"
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot response process hit error during update vg
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-vc1" "8"
  And I Force Update VG Error "response-update-error-vg"
  When I Call Test Create VG
  Then The Error Message Should Contain "unable to update VG"
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot response process hit error during update status for VolumeSnapshotContent
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-vc2" "8"
  And I Force Update VC Error "response-update-error-vc"
  When I Call Test Create VG
  Then The Error Message Should Contain "unable to update VolsnapContent"
  And I Call Clean up Volumes On Array

Scenario: Call Create VolumesGroupSnapshot response process hit error during create VolumeSnapshot
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-vsc1" "8"
  And I Force Create VS Error "response-create-error-vs"
  When I Call Test Create VG
  Then The Error Message Should Contain "unable to create Volsnap"
  And I Call Clean up Volumes On Array

Scenario: Call Driver Verification for probing driver name and verifying it for creating vg
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-wer1" "8"
  When I Call Test Create VG With BadVsc
  Then The Error Message Should Contain "VG Snapshotter vg create failed, VolumeSnapshotClass driver name does not match volumegroupsnapshotter"
  And I Call Clean up Volumes On Array

Scenario: Call HandleSnapContentDelete twice to clean up finalizer
  Given a Vgs Controller
  And I Call Create 2 Volumes "vg-snap-watch" "8"
  When I Call Test Create VG And HandleSnapContentDelete
  Then There are No Errors
  And I Call Clean up Volumes On Array
