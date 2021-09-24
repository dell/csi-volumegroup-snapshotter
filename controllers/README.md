
# csi-volumegroupsnapshotter Controller

The csi-volumegroupsnapshotter controller processes reconcile requests for VolumeGroupSnapshot events. 
This README describes the basic controller logic.

## Reconcile Logic for VolumeGroupSnapshot Create Event 
Reconciliation steps:
1. Find all PVC instances with volume-group label as defined in `pvcLabel` attribute in volume group snapshot
```
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: pvol0
  namespace: helmtest-vxflexos
  labels:
    volume-group: volumeGroup1
spec:
  accessModes:
  - ReadWriteOnce
  volumeMode: Filesystem
  resources:
    requests:
      storage: 8Gi
  storageClassName: vxflexos
```
2. Get volumeHandle for all corresponding PersistentVolume instances for this set of PVCs.
3. Call CreateVolumeGroupSnapshot() CSI API extension method of CSI driver with list of volume handles and volume group snapshot name.
4. Once driver responds with list of Snapshot objects create VolumeSnapshot and VolumeSnapshotContent instances for group members in Kubernetes. As a result, VolumeSnapshot and VolumeSnapshotContent instances are created for each snapshot in the group. To associate VolumeSnapshot instances with parent group, these objects are labeled with VolumeGroupSnapshot name.
5. Update status of VolumeGroupSnapshot to set groupID, creationTime and list of VolumeSnapshot member names.

## Reconcile Logic for VolumeGroupSnapshot Delete Event
Reconciliation steps: 
1. Process member VolumeSnapshot instances based on the value of `Deletion Policy` in volumesnapshotclass that vgs specifies. For `Delete` call Kubernetes API to delete each
   VolumeSnapshot instance. Kubernetes will also delete the corresponding VolumeSnapshotContent instance. For `Retain` keep VolumeSnapshot instances.

## CSI Extension for Volume Group Snapshot Operations in Drivers
The CSI extension API is defined  [here](https://github.com/dell/dell-csi-extensions) under volumeGroupSnapshot.

