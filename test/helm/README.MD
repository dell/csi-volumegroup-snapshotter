# Helm Test for Volume Group Snapshotter
Helm test for volume group snapshotter(VGS) creates three volumes and a storage class. Then a VGS will be created from vgs.yaml. It will snapshot the three volumes. 
The test then checks all volumesnapshots and volumesnapshotcontents are in ready status. If everything is ok, it will clean up itself.


## pre-requesite
This is an end-to-end test that uses real array, driver, and kubernetes. To allow it to work, make sure:
- CSI driver is installed
- Volume snapshot CRD and controller are installed
- VGS CRD is installed

## Run Test
To run the test, first fill in the system/array ID and storage pool that you want to run test on. Then Run `./vgstest.sh` to start. If it finishs ok, you should see `vgs test passed. Clean up resources...` in output.
