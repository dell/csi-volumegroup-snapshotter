module github.com/dell/dell-csi-volumegroup-snapshotter/test/integration-test

replace github.com/dell/dell-csi-extensions/volumeGroupSnapshot => ../../dell-csi-extensions/volumeGroupSnapshot

replace github.com/dell/dell-csi-volumegroup-snapshotter => ../../

replace github.com/dell/csi-vxflexos => ./csi-vxflexos

// replace github.com/dell/goscaleio => ./csi-vxflexos/goscaleio

go 1.16

require (
	github.com/container-storage-interface/spec v1.5.0
	github.com/cucumber/godog v0.12.1
	github.com/dell/csi-vxflexos v1.5.1
	github.com/dell/dell-csi-volumegroup-snapshotter v0.2.0
	github.com/dell/gocsi v1.4.0
	github.com/golang/protobuf v1.5.2
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0
	go.uber.org/zap v1.19.1
	google.golang.org/grpc v1.40.0
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	sigs.k8s.io/controller-runtime v0.10.0
)
