/*
Copyright 2018 The Kubernetes Authors.
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
	"fmt"
	"os"
	"time"

	awsclient "github.com/jhjaggars/capa-annotator/pkg/client"
	machinesetcontroller "github.com/jhjaggars/capa-annotator/pkg/controller"
	"github.com/jhjaggars/capa-annotator/pkg/version"
	corev1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// The default durations for the leader election operations.
var (
	leaseDuration = 120 * time.Second
	renewDeadline = 110 * time.Second
	retryPeriod   = 20 * time.Second
)

func main() {
	printVersion := flag.Bool(
		"version",
		false,
		"print version and exit",
	)

	metricsAddress := flag.String(
		"metrics-bind-address",
		":8080",
		"Address for hosting metrics",
	)

	watchNamespace := flag.String(
		"namespace",
		"",
		"Namespace that the controller watches to reconcile machine-api objects. If unspecified, the controller watches for machine-api objects across all namespaces.",
	)

	leaderElectResourceNamespace := flag.String(
		"leader-elect-resource-namespace",
		"",
		"The namespace of resource object that is used for locking during leader election. If unspecified and running in cluster, defaults to the service account namespace for the controller. Required for leader-election outside of a cluster.",
	)

	leaderElect := flag.Bool(
		"leader-elect",
		false,
		"Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.",
	)

	leaderElectLeaseDuration := flag.Duration(
		"leader-elect-lease-duration",
		leaseDuration,
		"The duration that non-leader candidates will wait after observing a leadership renewal until attempting to acquire leadership of a led but unrenewed leader slot. This is effectively the maximum duration that a leader can be stopped before it is replaced by another candidate. This is only applicable if leader election is enabled.",
	)

	healthAddr := flag.String(
		"health-addr",
		":9440",
		"The address for health checking.",
	)

	klog.InitFlags(nil)
	if err := flag.Set("logtostderr", "true"); err != nil {
		klog.Fatalf("Error setting logtostderr flag: %v", err)
	}
	flag.Parse()

	if *printVersion {
		fmt.Println(version.String)
		os.Exit(0)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		klog.Fatalf("Error getting configuration: %v", err)
	}

	// Setup a Manager
	syncPeriod := 10 * time.Minute
	opts := manager.Options{
		LeaderElection:          *leaderElect,
		LeaderElectionNamespace: *leaderElectResourceNamespace,
		LeaderElectionID:        "capa-annotator-leader",
		LeaseDuration:           leaderElectLeaseDuration,
		HealthProbeBindAddress:  *healthAddr,
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		Metrics: server.Options{
			BindAddress: *metricsAddress,
		},
		// Slow the default retry and renew election rate to reduce etcd writes at idle: BZ 1858400
		RetryPeriod:   &retryPeriod,
		RenewDeadline: &renewDeadline,
	}

	if *watchNamespace != "" {
		opts.Cache.DefaultNamespaces = map[string]cache.Config{
			*watchNamespace: {},
		}
		klog.Infof("Watching CAPI objects only in namespace %q for reconciliation.", *watchNamespace)
	}

	mgr, err := manager.New(cfg, opts)
	if err != nil {
		klog.Fatalf("Error creating manager: %v", err)
	}

	// Setup Scheme for all resources
	if err := clusterv1.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatalf("Error setting up CAPI scheme: %v", err)
	}

	if err := infrav1.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatalf("Error setting up CAPA scheme: %v", err)
	}

	if err := corev1.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatal(err)
	}

	describeRegionsCache := awsclient.NewRegionCache()

	ctrl.SetLogger(textlogger.NewLogger(textlogger.NewConfig()))
	setupLog := ctrl.Log.WithName("setup")

	if err := (&machinesetcontroller.Reconciler{
		Client:             mgr.GetClient(),
		Log:                ctrl.Log.WithName("controllers").WithName("MachineDeployment"),
		AwsClientBuilder:   awsclient.NewValidatedClient,
		RegionCache:        describeRegionsCache,
		InstanceTypesCache: machinesetcontroller.NewInstanceTypesCache(),
	}).SetupWithManager(mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MachineDeployment")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		klog.Fatal(err)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.Fatal(err)
	}

	// Start the Cmd
	err = mgr.Start(ctrl.SetupSignalHandler())
	if err != nil {
		klog.Fatalf("Error starting manager: %v", err)
	}
}
