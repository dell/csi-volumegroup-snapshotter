#!/bin/bash
# used for cleanup after a failed vgs helm test
NS=helmtest-vxflexos

helm delete -n $NS vgs
sleep 10
kubectl get pods -n $NS

# delete pvcs
force=no
if [ "$1" == "--force" ];
then
	force=yes
fi

pvcs=$(kubectl get pvc -n helmtest-vxflexos | awk '/pvol/ { print $1; }')
echo deleting... $pvcs
for pvc in $pvcs
do
if [ $force == "yes" ];
then
	echo kubectl delete --force --grace-period=0 pvc $pvc -n $NS
	kubectl delete --force --grace-period=0 pvc $pvc -n $NS
else
	echo kubectl delete pvc $pvc -n $NS
	kubectl delete pvc $pvc -n $NS
fi
done

kubectl get pvc -o wide

# delete vgs and volumesnapshots
kubectl delete -f vgs-pvc-label.yaml
kubectl delete -f vgs-pvc-names.yaml
kubectl delete volumesnapshots --all -n $NS
