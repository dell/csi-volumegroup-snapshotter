#!/bin/bash
NS=helmtest-vxflexos

# check DellCsiVolumeGroupSnapshot CRD exists
kubectl api-resources | grep -q DellCsiVolumeGroupSnapshot
if [ $? != 0 ]; then
  echo "CRD DellCsiVolumeGroupSnapshot needs to be installed to run vgs test"
  exit 1
fi

if [ "$1" != "names" ] && [ "$1" != "label" ] && [ "$1" != "ns" ]; then
  echo "first argument is required. Must be 'names', 'label', or 'ns'"
  exit 1
fi

echo "creating 3 volumes and sc"
helm install -n ${NS} vgs vgs --values helmtest.yaml
sleep 5
echo "done creating 3 volumes and sc"
echo "create vgs"
kubectl create -f vgs-pvc-$1.yaml

sleep 10
snapCount=$(kubectl get volumesnapshot -n $NS | grep vgs-helm-test | wc -l)
readySnapCount=$(kubectl get volumesnapshot -n $NS | grep vgs-helm-test | grep true | wc -l)

vscCount=$(kubectl get volumesnapshotcontent | grep vgs-helm-test | wc -l)
vscReadyCount=$(kubectl get volumesnapshotcontent | grep vgs-helm-test | grep true | wc -l)

if [[ $snapCount != 3 || $readySnapCount != 3 ]]; then
  echo "volumesnapshots are not ready"
  exit 2
fi

if [[ $vscCount != 3 || $vscReadyCount != 3 ]]; then
  echo "volumesnapshotcontents are not ready"
  exit 2
fi

echo "describe vgs:"
echo "k describe vgs -n $NS vgs-helm-test"
kubectl describe vgs -n $NS vgs-helm-test

snapshotGroupID=$(kubectl get vgs -n $NS vgs-helm-test -o=jsonpath='{ .status.snapshotGroupID }')
snapshots=$(kubectl get vgs -n $NS vgs-helm-test -o=jsonpath='{ .status.snapshots }')
if [[ -z "$snapshotGroupID" ]]; then
  echo "status.snapshotGroupID is empty"
  exit 2
fi

if [[ -z "$snapshots" ]]; then
  echo "status.snapshots is empty"
  exit 2
fi

echo "vgs test passed. Clean up resources..."
kubectl delete -f vgs-pvc-$1.yaml
kubectl delete volumesnapshots --all -n $NS
helm delete -n $NS vgs
