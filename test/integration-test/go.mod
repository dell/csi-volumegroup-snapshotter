module github.com/dell/csi-volumegroup-snapshotter/test/integration-test

//replace github.com/dell/dell-csi-extensions/volumeGroupSnapshot => ../../dell-csi-extensions/volumeGroupSnapshot

replace github.com/dell/csi-volumegroup-snapshotter => ../../

//replace github.com/dell/vxflexos => ./csi-powerflex

// replace github.com/dell/goscaleio => ./csi-vxflexos/goscaleio

go 1.16

require (
	github.com/container-storage-interface/spec v1.5.0
	github.com/cucumber/godog v0.12.2
	github.com/dell/csi-volumegroup-snapshotter v0.3.0
	github.com/dell/csi-vxflexos/v2 v2.0.0-20211014194653-b2cf36dab234
	github.com/dell/gocsi v1.4.0
	github.com/dell/goscaleio v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0
	go.uber.org/zap v1.19.1
	google.golang.org/grpc v1.40.0
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	sigs.k8s.io/controller-runtime v0.10.2
)
