
refer features/vg.feature for scenarios covered in this integration test

pre req : must have sdc installed and driver int test pass , config.json in this folder must point to valid array

also look at go.mod and pull the "replace" source for csi-vxflexos , goscaleio and dell-csi-extensions to the appropriate location

run.sh : will start driver-server and run snapshot vg create tests

