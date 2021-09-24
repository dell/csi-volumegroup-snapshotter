<!--
Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
-->

# Dell Container Storage Modules (CSM) Volume Group Snapshotter

[![License](https://img.shields.io/github/license/dell/csi-volumegroup-snapshotter?style=flat-square&color=blue&label=License)](https://github.com/dell/csi-volumegroup-snapshotter/blob/master/LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/dellemc/csi-volumegroup-snapshotter)](https://hub.docker.com/repository/docker/dellemc/csi-volumegroup-snapshotter)
[![Go version](https://img.shields.io/github/go-mod/go-version/dell/csi-volumegroup-snapshotter)](go.mod)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/dell/csi-volumegroup-snapshotter?include_prereleases&label=latest&style=flat-square)](https://github.com/dell/csi-volumegroup-snapshotter/releases/latest)

Dell CSM Volume Group Snapshotter is part of the [Container Storage Modules (CSM)](https://github.com/dell/csm) open-source suite of Kubernetes storage enablers for Dell EMC products.

This project aims at extending native Kubernetes functionality to support _Disaster Recovery_ workflows by utilizing storage array based volume snapshot.

The Dell CSM Volume Group Snapshotter is an operator which extends Kubernetes API to support crash-consistent snapshots of groups of volumes. 
This operator consists of VolumeGroupSnapshot CRD and _CSM Volumegroupsnapshotter Controller_, which is a sidecar container that runs in the controller pod of the CSI driver.
It uses the `dell-csi-extensions` API, implemented by Dell EMC CSI drivers, to manage volume group snapshots on backend arrays. You can read more about the extensions [here](https://github.com/dell/dell-csi-extensions).

For documentation, please visit [Container Storage Modules documentation](https://dell.github.io/csm-docs/).

CSI Volume Group Snapshotter is currently in a Technical Preview Phase, and should be considered pre-release software. We are actively seeking feedback from users about its features. Please provide feedback using . We will take that input, along with our own results from doing extensive testing, and incrementally improve the software. We do not recommend or support it for production use at this time.

## Supported CSI Drivers
Currently, CSM Volume Group Snapshotter provides support to create and delete volume group snapshots for PowerFlex array. 
Additional array support in CSM Volume Group Snapshotter is planned for the near future.

## Build
This project is a Go module (see golang.org Module information for explanation), and you can build multiple binaries & images with the provided Makefile.

### Dependencies
This project relies on the following tools which have to be installed in order to generate certain manifests.

| Tool | Version |
| --------- | ----------- |
| externalshapshotter-v1 | v4.1.x |

To install the external-snapshotter, please see the [GitHub repository](https://github.com/kubernetes-csi/external-snapshotter/tree/v4.1.1). If you have already installed the correct version of the external-snapshotter with the CSI driver, you don't need to do it again.


### Binaries

To build an image for docker, run `make docker-build`
To build an image for podman, run `make podman-build`

## Installation
To install and use the Volume Group Snapshotter, you need to install the CRD in your cluster and also deploy it with the driver.

### Custom Resource Definitions

Run the command `make install` to install the Custom Resource Definitions in your Kubernetes cluster.
Configure all the helm chart parameters described below before deploying the drivers.

### Deploy VGS in Driver with Helm Chart Parameters

The drivers that support Helm chart deployment allow the CSM Volume Group Snapshotter to be _optionally_ deployed 
by variables in the chart. There is a _vgsnapshotter_ block specified in the _values.yaml_ file of the chart that will look similar the text below by default:

```
# volume group snapshotter(vgsnapshotter) details
# These options control the running of the vgsnapshotter container
vgsnapshotter:
  enabled: false
  image: 
 
```
To deploy CSM Volume Group Snapshotter with the driver, the following changes are required:
1. Enable CSM Volume Group Snapshotter by changing the vgsnapshotter.enabled boolean to true. 
2. In the image field, put the location of the image you created in the above steps, or link to one already built.
3. Install driver with `csi_install.sh` script

## Volume Group Snapshot CRD Create
In Kubernetes, volume group snapshot objects are represented as instances of VolumeGroupSnapshot CRD.
Here is an example of a VolumeGroupSnapshot instance in Kubernetes:

```yaml
Name:         demo-vg
Namespace:    helmtest-vxflexos
API Version:  volumegroup.storage.dell.com/v1alpha2
Kind:         DellCsiVolumeGroupSnapshot
Spec:
  Driver Name:            csi-vxflexos.dellemc.com
  Member Reclaim Policy:  Retain
  Pvc Label:              vgs-snap-label
  Volumesnapshotclass:    vxflexos-snapclass
Status:
  Creation Time:        2021-08-20T08:56:16Z
  Ready To Use:         true
  Snapshot Group ID:    1235e15806d1ec0f-b0eec05600000003
  Snapshot Group Name:  demo-vg-20082021-142241
  Snapshots:            demo-vg-20082021-142241-0-pvol0,demo-vg-20082021-142241-1-pvol2,demo-vg-20082021-142241-2-pvol1
  Status:               Complete
Events:
  Type    Reason   Age   From                              Message
  ----    ------   ----  ----                              -------
  Normal  Updated  41s   dell-csi-volumegroup-snapshotter  Created VG snapshotter vg with name: demo-vg
  Normal  Updated  41s   dell-csi-volumegroup-snapshotter  Updated VG snapshotter vg with name: demo-vg
```
Some of the above fields are described here:

| Field               | Description                                                         |
| :--                 | :--                                                                 |
| **Status**          | Group of field defining the status of the VGS.                      |
| Creation Time       | Time that the snapshot was taken.                                   |
| Ready To Use        | Field defining whether the VGS is ready to use.                     |
| Snapshot Group ID   | Group ID for the VGS.                                               |
| Snapshot Group Name | Unique, timestamped name for the VGS (date format: DDMMYYYY-HHMMSS) |
| Snapshots           | List of Volume Snapshots contained in the group.                    |
| Status              | Overall status of the VGS.                                          |

To create an instance of VolumeGroupSnapshot in Kubernetes cluster, create .yaml file similar to vg.yaml in the top-level folder. Below is a sample .yaml followed by a table of parameters:
```
apiVersion: volumegroup.storage.dell.com/v1alpha2
kind: DellCsiVolumeGroupSnapshot
metadata:
  # Name must be 13 characters or less in length
  name: "vg-snaprun1"
  namespace: "helmtest-vxflexos"
spec:
  # Add fields here
  driverName: "csi-vxflexos.dellemc.com"
  memberReclaimPolicy: "Retain"
  volumesnapshotclass: "vxflexos-snapclass"
  pvcLabel: "vgs-snap-label"
  # pvcList:
  #   - "pvcName1"
  #   - "pvcName2"
```
| Parameter           | Description                                                  | Required | Default |
| :------------------ | :----------------------------------------------------------- | :------- | :------ |
| name                | Name of snapshot to be taken. Each name must be in the namespace, and each name must be 13 characters or less. | true     | -       |
| namespace           | The namespace that the snapshot lives in.                    | true     | -       |
| driverName          | Name of the Powerflex CSI driver.                            | true     | -       |
| memberReclaimPolicy | Currently Not used. Instead `Deletion Policy` from specified VolumeSnapshotClass defines how to process individual VolumeSnapshot instances when the snapshot group is deleted. There are two options -- "Retain" to retain the snapshots, and "Delete" to delete the snapshots. | true     | -       |
| volumesnapshotclass | Volume snapshot class name for VGS members.                  | true     | -       |
| pvcLabel            | A label that can also be included in PVC yamls to specify a set of PVCs for the VGS. Either this or pvcList is required, but not both. | false    | -       |
| pvcList             | A list of PVCs for the VGS. Either this or pvcLabel is required, but not both. | false    | -       |

Run the command `kubectl create -f vg.yaml` to take the specified snapshot.

### How to create policy based Volume Group Snapshots  
Currently, array based policies are not supported. This will be addressed in an upcoming release. For a temporary solution, cronjob can be used to mimic policy based Volume Group Snapshots. The only supported policy is how often the group should be created. To create a cronjob that creates a volume group snapshot periodically, use the template found in samples/ directory. Once the template is filled out, use the command `kubectl create -f samples/cron-template.yaml` to create the configmap and cronjob. 
>Note: Cronjob is only supported on Kubernetes versions 1.21 or higher 

### Deleting policy based Volume Group Snapshots  
Currently, automatic deletion of Volume Group Snapshots is not supported. All deletion must be done manually.

## Testing
To build the unit tests, run `make unit-test`

To build the integration tests, run `make int-test`
  Additional steps are required to run integration tests.
  For more information, consult the [integration test README](test/integration-test/README.md) 

To run the helm tests, consult the [helm test README](test/helm/README.MD)

## Troubleshooting
To get the logs from VGS, do `kubectl logs POD-NAME vg-snapshotter`

| Symptoms | Prevention, Resolution or Workaround |
|------------|--------------|
| Snapshot creation fails with `VG name exceedes 13 characters` | In the VolumeGroup.yaml file, change the name to something with 13 characters or less |
| Snapshot creation fails with `no matches for kind \"DellCsiVolumeGroupSnapshot\" in version \"volumegroup.storage.dell.com/` | CRD is not created. Run `make install` to install CRD for Volume Group Snapshotter |
| `kubectl describe vgs VGS-NAME` shows its Status.Status is `error` | Driver fails to create VG on array |
| `kubectl describe vgs VGS-NAME` shows its Status.Status is `incomplete` | VG is created ok on array, controller fails to create snapshot/snapshotcontent |
| Snapshot deletion fails with `VG Snapshotter status is incomplete` | Ensure you use the latest CRDs available in config/crd/bases/ | 


