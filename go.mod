module github.com/dell/csi-volumegroup-snapshotter

replace github.com/dell/csi-volumegroup-snapshotter => ./

go 1.16

require (
	github.com/dell/dell-csi-extensions/volumeGroupSnapshot v0.2.0
	github.com/go-chi/chi v1.5.4
	github.com/go-logr/logr v0.4.0
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0
	github.com/lithammer/fuzzysearch v1.1.2
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20210908191846-a5e095526f91 // indirect
	golang.org/x/oauth2 v0.0.0-20210402161424-2e8d93401602 // indirect
	golang.org/x/sys v0.0.0-20210908233432-aa78b53d3365 // indirect
	google.golang.org/grpc v1.40.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/controller-runtime v0.10.0
)
