# Helm Test for Volume Group Snapshotter
Helm test for volume group snapshotter (VGS) creates three volumes and a storage class. Then a VGS will be created from vgs.yaml. It will snapshot the three volumes. 
The test then checks all volumesnapshots and volumesnapshotcontents are in ready status. If everything is ok, it will clean up itself.


## Pre-requesites
This is an end-to-end test that uses real array, driver, and kubernetes. To allow it to work, make sure:
- CSI driver is installed
- Volume snapshot CRD and controller are installed
- VGS CRD is installed
- helmtest.yaml in this folder points to a valid array

## Run Test
To run the test, run `./vgstest.sh <argument>` to start. The argument can be either `label`, `names`, or `ns`. If it finishs ok, you should see `vgs test passed. Clean up resources...` in output.

# Helm Test for CronJob
Helm test for CronJob creates three volumes and a storage class. Then a CronJob that creates a Volume Group Snapshot every minute is created. 

## Pre-requesites
This is an end-to-end test that uses real array, driver, and kubernetes. To allow it to work, make sure:
- CSI driver is installed
- Volume snapshot CRD and controller are installed
- VGS CRD is installed
- Test is running on K8s 1.21 or higher

## Run Test
To run the test, fill in helmtest.yaml with the values required, and run ```./cronTest.sh```, located in the cron/ directory. The test should take around 3 minutes to complete.  
