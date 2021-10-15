# Script to verify Volumegroup-snapshotter installation

## Description

This directory provides scripts to verify the Kubernetes environment for vgs instalation.


### Verify A Kubernetes Environment

The `verify.sh` script is run by itself. This provides a handy means to validate a Kubernetes system with vgs-controller installed as a sidecar to the driver. To verify an environment, run `verify.sh`.
The verify script validates snapshots version installed, vgs crds version  and vgs-image running in the driver.

For usage information:
```
[scripts]# ./verify.sh 
