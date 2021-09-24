this code is for development only

The purpose is to change controller code and quickly verify without time consuming steps to build image and deploy to k8s

pre req : you need csi driver for vxflexos int test working , that is code is checked out , and configurd ok with powerflex

refer go.mod and pull the csi-vxflexos , goscaleio and dell-csi-extensions as per "replace" directives

edit stat_server.sh to set folder for driver and run it

now edit dev_test.go to set src volume ids  that must exist 

run go test dev_test.go -v  , will create a vg 

