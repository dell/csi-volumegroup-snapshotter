module github.com/dell/csi-volumegroup-snapshotter

replace github.com/dell/csi-volumegroup-snapshotter => ./

go 1.16

require (
	github.com/dell/dell-csi-extensions/volumeGroupSnapshot v1.0.0
	github.com/go-chi/chi v1.5.4
	github.com/go-logr/logr v0.4.0
	github.com/google/uuid v1.3.0
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0
	github.com/lithammer/fuzzysearch v1.1.2
	github.com/stretchr/testify v1.7.0
	google.golang.org/grpc v1.42.0
	k8s.io/api v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/client-go v0.22.3
	k8s.io/klog/v2 v2.10.0
	sigs.k8s.io/controller-runtime v0.10.2
)
