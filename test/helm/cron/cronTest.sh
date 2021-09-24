#!/bin/bash
NS=helmtest-vxflexos

# check DellCsiVolumeGroupSnapshot CRD exists
kubectl api-resources | grep -q DellCsiVolumeGroupSnapshot
if [ $? != 0 ]; then
  echo "CRD DellCsiVolumeGroupSnapshot needs to be installed to run vgs test"
  exit 1
fi

echo "creating 3 volumes and sc"
helm install -n ${NS} vgs ../vgs --values ../helmtest.yaml
echo "done creating 3 volumes and sc"
echo "creating configmap with vgs yaml and CronJob to create a vgs every minute"
kubectl create -f cron.yaml

limit=3

numOfGroups=$(kubectl get -n $NS dellcsivolumegroupsnapshot.volumegroup.storage.dell.com | grep "vgs-test-" | wc -l)
echo ""

while [ $numOfGroups -le $limit ]
do
	echo "Waiting for 3 volume group snapshots to be created..." 
	echo ""
	echo "kubectl get jobs -A"
	kubectl get jobs -A 
	echo ""
	echo "kubectl get pods -n vxflexos"
	kubectl get pods -n vxflexos
	echo ""
	echo "kubectl get -n $NS dellcsivolumegroupsnapshot.volumegroup.storage.dell.com"
	kubectl get -n $NS dellcsivolumegroupsnapshot.volumegroup.storage.dell.com 
	echo ""
	numOfGroups=$(kubectl get -n $NS dellcsivolumegroupsnapshot.volumegroup.storage.dell.com | grep "vgs-test-" | wc -l)
	
	if [ $numOfGroups -eq $limit ]; then
		echo "3 volume group snapshots created"
		echo "" 
		break
	fi 

	echo "sleep 60"
	sleep 60
	echo ""
done 
echo "Created these snapshots: "
kubectl get volumesnapshots -n $NS
echo ""

numOfSnaps=$(kubectl get -n $NS volumesnapshots | grep "vgs-test-" | wc -l)
if [ $numOfSnaps -eq 0 ]; then
	echo "Test Failed: Groups were created but snaps were not.Check vg-snapshotter logs."
	exit 2 
fi


echo "Test passed. Cleaning up." 
kubectl delete -f cron.yaml
sleep 5 
kubectl delete -n $NS dellcsivolumegroupsnapshot.volumegroup.storage.dell.com --all
kubectl delete volumesnapshots --all -n $NS
helm delete -n $NS vgs

