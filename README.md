# CSM Volume Group Snapshotter
Many stateful Kubernetes applications use several persistent volumes to store data. 
To create recoverable snapshots of volumes for these applications, it is necessary to have capability to create 
consistent snapshots across all volumes of the application at the same time.  
Dell CSM Volume Group Snapshotter is an operator which extends Kubernetes API to support crash-consistent 
snapshots of groups of volumes. 
This operator consists of VolumeGroupSnapshot CRD and csi-volumegroupsnapshotter controller. The csi-volumegroupsnapshotter
is a sidecar container, which runs in the controller pod of CSI driver.
The csi-volumegroupsnapshotter uses CSI extension, implemented by Dell EMC CSI drivers, to manage volume group snapshots on 
backend arrays. 

CSM Volume Group Snapshotter is currently in a Technical Preview Phase, and should be considered alpha software. 
We are actively seeking feedback from users about its features.
Please provide feedback using <TBD> .
We will take that input, along with our own results from doing extensive testing, 
and incrementally improve the software. We do not recommend or support it for production use at this time.

## Volume Group Snapshot CRD
In Kubernetes volume group snapshot objects are represented as instances of VolumeGroupSnapshot CRD.
Example of VolumeGroupSnapshot instance in Kubernetes:
```yaml
Name:         vg1-snap1
Namespace:    helmtest-vxflexos
Labels:       <none>
Annotations:  <none>
API Version:  volumegroup.storage.dell.com/v1alpha1
Kind:         DellCsiVolumeGroupSnapshot
Metadata:
  Creation Timestamp:  2021-05-07T16:18:15Z
  Generation:          1
  Managed Fields:
    API Version:  volumegroup.storage.dell.com/v1alpha1
    Fields Type:  FieldsV1
    .............
    Manager:         vg-snapshotter
    Operation:       Update
    Time:            2021-05-07T16:18:17Z
  Resource Version:  24607275
  UID:               c2f53f33-1bd6-40ef-b1df-59b85627834d
Spec:
  Driver Name:            csi-vxflexos.dellemc.com
  Member Reclaim Policy:  retain
  Pvc Label:              volumeGroup1
  Volumesnapshotclass:    vxflexos-snapclass
Status:
  Creation Time:      2021-05-07T16:08:32Z
  Snapshot Group ID:  4d4a2e5a36080e0f-bab0ef6900000002
  Snapshots:          vg1-snap1-0-pvol1,vg1-snap1-1-pvol0

```
To create an instance of VolumeGroupSnapshot in Kubernetes cluster, create .yaml file similar to this one VGS.yaml:
```
apiVersion: volumegroup.storage.dell.com/v1alpha1 
kind: DellCsiVolumeGroupSnapshot 
metadata:
  name: "vg1-snap1"
  namespace: "helmtest-vxflexos"
spec:
  driverName: "csi-vxflexos.dellemc.com"
  # defines how to process VolumeSnapshot members when volume group snapshot is deleted
  # "retain" --- keep VolumeSnapshot instances
  # "delete" --- delete VolumeSnapshot instances
  memberReclaimPolicy: "retain"
  # volume snapshot class to use for VolumeSnapshot members in volume group snapshot
  volumesnapshotclass: "vxflexos-snapclass"
  pvcLabel: "volumeGroup1"
```
Run command: `kubectl create -f VGS.yaml`

## csi-volumegroupsnapshotter Controller

The csi-volumegroupsnapshotter controller processes reconcile requests for VolumeGroupSnapshot events. 
#### Reconcile logic for VolumeGroupSnapshot create event 
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
2. Get volumeHandle for all corresponding PersistentVolume instances for this set of PVCs
3. Call CreateVolumeGroupSnapshot() CSI API extension method of CSI driver with list of volume handles and volume group
   snapshot name
4. Once driver responds with list of Snapshot objects create 
   VolumeSnapshot and VolumeSnapshotContent instances for group members in Kubernetes. As a result, VolumeSnapshot and 
   VolumeSnapshotContent instances are created for each snapshot in the group. To associate VolumeSnapshot instances with
   parent group, these objects are labeled with VolumeGroupSnapshot name.
5. Update status of VolumeGroupSnapshot to set groupID, creationTime and list of VolumeSnapshot member names.   

#### Reconcile logic for VolumeGroupSnapshot delete event
Reconciliation steps: 
1. Call DeleteVolumeGroupSnapshot CSI API extension method of CSI driver with volume group snapshot ID
2. Once driver responds, remove volume group snapshot label from VolumeSnapshot members
3. Delete VolumeGroupSnapshot in Kubernetes
3. Process member VolumeSnapshot instances based on the value of 
   `memberReclaimPolicy` in VolumeGroupSnapshot instance. For `delete` call Kubernetes API to delete each
   VolumeSnapshot instance, for `retain` keep VolumeSnapshot instances 

## CSI Extension for Volume Group Snapshot Operations in Drivers
The CSI extension API is defined  [here](https://github.com/dell/dell-csi-extensions) under volumeGroupSnapshot.

## Supported CSI Drivers
Currently, in the initial Technical Preview, CSM Volume Group Snapshotter provides support to create and delete volume group 
snapshots for PowerFlex array. 
Additional array support in CSM Volume Group Snapshotter is planned for the near future.

## Deploying CSM Volume Group Snapshotter
Install Volume Group Snapshot alpha CRD in your cluster:
* wget https://github.com/dell/dell-csi-volumegroup-snapshotter/tree/master/config/crd/bases
* kubectl create -f config/crd/bases

Configure all the helm chart parameters described below before deploying the drivers.

### Helm Chart Installation

These installation instructions apply to the helm chart in the (PowerFlex CSI Driver) https://github.com/dell/csi-powerflex repository
version v1.5.0.  
The drivers that support Helm chart deployment allow CSM Volume Group Snapshotter to be _optionally_ deployed 
by variables in the chart. There is a _vgsnapshotter_ block specified in the _values.yaml_ file of the chart 
that will look similar the text below by default:

```
# Volume Group Snapshotter feature is an optional feature under development and tech preview.
# Enable this feature only after contact support for additional information
vgsnapshotter:
  enabled: false
  image: 
 
```
To deploy CSM Volume Group Snapshotter with the driver, the following changes are required:
1. Enable CSM Volume Group Snapshotter by changing the vgsnapshotter.enabled boolean to true. 
2. Specify the Volume Group Snapshotter image to be used as vgsnapshotter.image .
3. Install PowerFlex driver with `csi_install.sh`

### How to Build Controller Image
```  
git clone https://github.com/dell/dell-csi-volumegroup-snapshotter.git  
cd dell-csi-volumegroup-snapshotter  
make docker-build ( to build image or make podman-build)  
```  

## Testing Approach 
### Unit Tests
  To run unit tests, at top level of repository, run 
  ```make unit-test```  
  
### Integration Tests
  To run integration tests, at top level of repository, run
  ```make int-test```  
  for more information, consult the [integration test README](test/integration-test/README.md) 

### Helm Tests
  To run the helm test, consult the [helm test README](test/helm/README.MD)


