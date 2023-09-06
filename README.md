<!--
Copyright (c) 2021 - 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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

## Table of Contents

* [Code of Conduct](https://github.com/dell/csm/blob/main/docs/CODE_OF_CONDUCT.md)
* [Maintainer Guide](https://github.com/dell/csm/blob/main/docs/MAINTAINER_GUIDE.md)
* [Committer Guide](https://github.com/dell/csm/blob/main/docs/COMMITTER_GUIDE.md)
* [Contributing Guide](https://github.com/dell/csm/blob/main/docs/CONTRIBUTING.md)
* [List of Adopters](https://github.com/dell/csm/blob/main/docs/ADOPTERS.md)
* [Support](https://github.com/dell/csm/blob/main/docs/SUPPORT.md)
* [Security](https://github.com/dell/csm/blob/main/docs/SECURITY.md)
* [Supported CSI Drivers](#supported-csi-drivers)
* [Build](#build)
* [Installation](#installation)
* [Testing](#testing)
* [Troubleshooting](#troubleshooting)

## Supported CSI Drivers
Currently, CSM Volume Group Snapshotter provides support to create and delete volume group snapshots on Dell PowerFlex and Dell PowerStore storage platforms. 
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
To build an image, run `make docker`  
Image will be built with Podman if it is avaialble; otherwise, Docker will be used. 
> Note: Buildah is needed to build the image

## Installation
To install and use the Volume Group Snapshotter, you need to install pre-requisites in your cluster, then install the CRD in your cluster and deploy it with the driver.

### 1. Install Pre-Requisites
The only pre-requisite required is the external-snapshotter, which is available [here](https://github.com/kubernetes-csi/external-snapshotter/tree/v4.1.1). Version 4.1+ is required. This is also required for the driver, so if the driver has already been installed, this pre-requisite should already be fulfilled as well.

The external-snapshotter is split into two controllers, the common snapshot controller and a CSI external-snapshotter sidecar. The common snapshot controller must be installed only once per cluster.

Here are sample instructions on installing the external-snapshotter CRDs:
```
git clone https://github.com/kubernetes-csi/external-snapshotter/
cd ./external-snapshotter
git checkout release-<your-version>
kubectl create -f client/config/crd
kubectl create -f deploy/kubernetes/snapshot-controller
```

### 2. Install VGS CRD

```
IMPORTANT: delete previous v1aplha2 version of CRD and vgs resources created using alpha version.
	   Snapshots on array will remain if memberReclaimPolicy=retain was used.
```
If you want to install the VGS CRD from a pre-generated yaml, you can do so with the following command (run in top-level folder):
```
kubectl apply -f config/crd/vgs-install.yaml
```

If you want to create your own CRD for installation with Kustomize, then the command `make install` can be used to create and install the Custom Resource Definitions in your Kubernetes cluster.

### 3. Deploy VGS in CSI Driver with Helm Chart Parameters
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
2. In the vgsnapshotter.image field, put the location of the image you created following the steps in the build section, or link to one already built (such as the one on DockerHub, `dellemc/csi-volumegroup-snapshotter:v1.3.0`).
3. Install/upgrade the driver normally. You should now have VGS successfully deployed with the driver!

## Volume Group Snapshot CRD Create
In Kubernetes, volume group snapshot objects are represented as instances of VolumeGroupSnapshot CRD.
Here is an example of a VolumeGroupSnapshot instance in Kubernetes:

```yaml
Name:         demo-vg
Namespace:    helmtest-vxflexos
API Version:  volumegroup.storage.dell.com/v1
Kind:         DellCsiVolumeGroupSnapshot
Spec:
  Driver Name:            csi-vxflexos.dellemc.com
  Member Reclaim Policy:  Retain
  Pvc Label:              vgs-snap-label
  Timeout:                90
  Volumesnapshotclass:    vxflexos-snapclass
Status:
  Creation Time:        2021-08-20T08:56:16Z
  Ready To Use:         true
  Snapshot Group ID:    4d4a2e5a36080e0f-b0eec05600000003
  Snapshot Group Name:  
  Snapshots:            c685412100000013-snap-0-pvol2,c685412200000014-snap-1-pvol0,c685412300000015-snap-2-pvol1
  Status:               Complete
Events:
  Type    Reason   Age   From                              Message
  ----    ------   ----  ----                              -------
  Normal  Updated  41s   csi-volumegroup-snapshotter  reated snapshot consistency group on array with id: 4d4a2e5a36080e0f-bab2657800000003
  Normal  Updated  41s   csi-volumegroup-snapshotter  Created volume group with id: 4d4a2e5a36080e0f-bab2657800000003
```
Some of the above fields are described here:

| Field               | Description                                                         |
| :--                 | :--                                                                 |
| **Status**          | Group of field defining the status of the VGS.                      |
| Creation Time       | Time that the snapshot was taken.                                   |
| Ready To Use        | Field defining whether the VGS is ready to use.                     |
| Snapshot Group ID   | Group ID for the VGS.                                               |
| Snapshot Group Name | snapshot group name on array. Note for Powerflex there is no name   |
| Snapshots           | List of Volume Snapshots contained in the group.                    |
| Status              | Overall status of the VGS.                                          |

To create an instance of VolumeGroupSnapshot in Kubernetes cluster, create .yaml file similar to vg.yaml in the top-level folder. Below is a sample .yaml followed by a table of parameters:
```
apiVersion: volumegroup.storage.dell.com/v1
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
  timeout: 90
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
| timeout             | defines timeout value for snapshots to be created            | false    | 90       |
| pvcList             | A list of PVCs for the VGS. Either this or pvcLabel is required, but not both. | false    | -       |

Run the command `kubectl create -f vg.yaml` to take the specified snapshot.

### How to create policy based Volume Group Snapshots  
Currently, array based policies are not supported. This will be addressed in an upcoming release. For a temporary solution, cronjob can be used to mimic policy based Volume Group Snapshots. The only supported policy is how often the group should be created. To create a cronjob that creates a volume group snapshot periodically, use the template found in samples/ directory. Once the template is filled out, use the command `kubectl create -f samples/cron-template.yaml` to create the configmap and cronjob. 
>Note: Cronjob is only supported on Kubernetes versions 1.21 or higher 

### VolumeSnapshotContent watcher
A VolumeSnapshotContent watcher is implemented to watch for VG's managing VolumeSnapshotContent. When any of the VolumeSnapshotContents get deleted, its managing VG, if there is one, will update `Status.Snapshots` to remove that snapshot. If all the snapshots are deleted, the VG will be also deleted automatically. 

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
