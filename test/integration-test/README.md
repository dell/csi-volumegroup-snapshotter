# Integration tests for Volume Group Snapshotter
The integration test is simulated test of the different pieces of snapshotter. Refer to features/vg.feature for scenarios covered in this integration test.

## Pre-requisites
- SDC is installed
- Driver integration tests are passing
- config.json in this folder points to a valid array

## Run Test
Check the go.mod file and make sure all the "replace" directives are as desired.

run.sh : will start driver-server and run snapshot vg create tests
