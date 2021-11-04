/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	csiclient "github.com/dell/csi-volumegroup-snapshotter/pkg/csiclient"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"

	volumegroupv1 "github.com/dell/csi-volumegroup-snapshotter/api/v1"

	// latest v1 from external does compile ok
	s1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"

	"github.com/dell/csi-volumegroup-snapshotter/controllers"
	"github.com/dell/csi-volumegroup-snapshotter/pkg/common"
	"github.com/dell/csi-volumegroup-snapshotter/pkg/connection"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(s1.AddToScheme(scheme))
	utilruntime.Must(volumegroupv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	//var probeAddr string
	var csiAddress string
	var workerThreads int
	var retryIntervalStart time.Duration
	var retryIntervalMax time.Duration
	var csiOperationTimeout time.Duration
	//var vgContextKeyPrefix   string
	var domain string
	flag.StringVar(&csiAddress, "csi-address", "/var/run/csi/csi.sock", "Address for the csi driver socket")
	flag.IntVar(&workerThreads, "worker-threads", 2, "Number of concurrent reconcilers for each of the controllers")
	flag.DurationVar(&retryIntervalStart, "retry-interval-start", time.Second, "Initial retry interval of failed reconcile request. It doubles with each failure, upto retry-interval-max")
	flag.DurationVar(&retryIntervalMax, "retry-interval-max", 5*time.Minute, "Maximum retry interval of failed reconcile request")
	flag.DurationVar(&csiOperationTimeout, "timeout", 10*time.Second, "Timeout of waiting for response for CSI Driver")
	//flag.StringVar(&vgContextKeyPrefix, "context-prefix", "", "All the volume-group-attribute-keys with this prefix are added as annotation to the DellCSIVolumeGroup")
	flag.Parse()
	//controllers.InitLabelsAndAnnotations(domain)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	//flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	// todo : remove later
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	setupLog.V(1).Info("Prefix", "Domain", domain)
	//setupLog.V(1).Info(common.DellCSIVolumegroup, "Version", "1.0.0")
	// Connect to csi
	csiConn, err := connection.Connect(csiAddress)

	if err != nil {
		setupLog.Error(err, "failed to connect to CSI driver")
		os.Exit(1)
	}
	leaderElectionID := common.DellCSIVolumegroup

	csiclientconn := csiclient.New(csiConn, ctrl.Log.WithName("csiclient"), csiOperationTimeout)

	driverName, err := csiclientconn.ProbeDriver()
	if err != nil {
		setupLog.Error(err, "error waiting for the CSI driver to be ready")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   leaderElectionID,
	})
	if err != nil {
		setupLog.Error(err, "unable to start controller")
		os.Exit(1)
	}

	expRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(retryIntervalStart, retryIntervalMax)
	if err = (&controllers.DellCsiVolumeGroupSnapshotReconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("DellCsiVolumeGroupSnapshot"),
		VGClient:      csiclient.New(csiConn, ctrl.Log.WithName("volumegroup-client"), csiOperationTimeout),
		EventRecorder: mgr.GetEventRecorderFor(common.DellCSIVolumegroup),
		DriverName:    driverName,
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr, expRateLimiter, workerThreads); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DellCsiVolumeGroupSnapshot")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	setupLog.Info("starting controller")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
