module github.com/dell/dell-csi-volumegroup-snapshotter/test/integration-test

//replace github.com/dell/dell-csi-extensions/volumeGroupSnapshot => ../../dell-csi-extensions/volumeGroupSnapshot

replace github.com/dell/dell-csi-volumegroup-snapshotter => ../../

//replace github.com/dell/csi-vxflexos => ./csi-vxflexos

//replace github.com/dell/goscaleio => ./csi-vxflexos/goscaleio

go 1.16

require (
	github.com/container-storage-interface/spec v1.4.0
	github.com/cucumber/godog v0.11.0
	github.com/dell/csi-vxflexos v1.5.0
	github.com/dell/dell-csi-volumegroup-snapshotter v0.1.0
	github.com/dell/gocsi v1.3.0
	github.com/golang/protobuf v1.4.3
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.1.0
	go.uber.org/zap v1.16.0
	google.golang.org/grpc v1.29.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)
