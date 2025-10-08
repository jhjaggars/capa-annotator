# Implementation Plan - MachineSet Annotator

## Overview
This implementation plan details the step-by-step process for creating a standalone MachineSet annotation controller by porting code from `machine-api-provider-aws`. The controller will watch MachineSets, query AWS EC2 instance type information, and set capacity annotations for cluster-autoscaler.

**Module Path:** `github.com/jhjaggars/capi-annotator`
**Source Project:** `/Users/jjaggars/code/machine-api-provider-aws/`
**Target Project:** `/Users/jjaggars/code/capi-annotator/`

## Technical Architecture

### High-Level Architecture
```
┌─────────────────────────────────────────────────────────────┐
│              MachineSet Annotator Controller                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐      ┌──────────────┐                    │
│  │  Controller  │─────>│ AWS Client   │───> AWS EC2 API    │
│  │  Reconciler  │      │              │                     │
│  └──────────────┘      └──────────────┘                    │
│         │                      │                            │
│         │                      │                            │
│         v                      v                            │
│  ┌──────────────┐      ┌──────────────┐                    │
│  │  Provider    │      │  Instance    │                     │
│  │  Spec Parser │      │  Types Cache │                     │
│  └──────────────┘      └──────────────┘                    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
         │                          │
         v                          v
  ┌─────────────┐          ┌──────────────┐
  │ MachineSet  │          │  Kubernetes  │
  │  (watch)    │          │   Secrets    │
  └─────────────┘          └──────────────┘
```

### Component Breakdown

1. **Controller Reconciler** (`pkg/controller/controller.go`):
   - Watches MachineSet resources
   - Orchestrates reconciliation logic
   - Sets annotations on MachineSets

2. **Instance Types Cache** (`pkg/controller/ec2_instance_types.go`):
   - Thread-safe cache with RWMutex
   - 24-hour refresh cycle per region
   - Handles AWS API pagination

3. **AWS Client** (`pkg/client/client.go`):
   - Wraps AWS SDK clients (EC2, ELB, ELBv2)
   - Manages credentials from Kubernetes secrets
   - Supports custom CA bundles and endpoints
   - Region validation and caching

4. **Provider Spec Parser** (`pkg/utils/providerspec.go`):
   - Unmarshals AWSMachineProviderConfig from RawExtension
   - Validates configuration

5. **Main Entry Point** (`cmd/controller/main.go`):
   - Flag parsing and configuration
   - Manager setup with schemes
   - Controller registration
   - Health/metrics endpoints

## Implementation Steps

### Phase 1: Project Initialization (Day 1)

#### Step 1.1: Initialize Go Module
```bash
cd /Users/jjaggars/code/capi-annotator
go mod init github.com/jhjaggars/capi-annotator
```

#### Step 1.2: Create Directory Structure
```bash
mkdir -p cmd/controller
mkdir -p pkg/controller
mkdir -p pkg/client
mkdir -p pkg/utils
mkdir -p hack
```

Expected structure:
```
/Users/jjaggars/code/capi-annotator/
├── cmd/
│   └── controller/
│       └── main.go
├── pkg/
│   ├── controller/
│   │   ├── controller.go
│   │   └── ec2_instance_types.go
│   ├── client/
│   │   └── client.go
│   └── utils/
│       └── providerspec.go
├── hack/
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
└── README.md
```

#### Step 1.3: Copy and Update go.mod Dependencies
Based on machine-api-provider-aws/go.mod, add core dependencies:

```go
module github.com/jhjaggars/capi-annotator

go 1.23

require (
	github.com/aws/aws-sdk-go v1.55.7
	github.com/go-logr/logr v1.4.2
	github.com/openshift/api v0.0.0-20250108101736-59f72e9e2bf6
	github.com/openshift/machine-api-operator v0.2.1-0.20241212154333-7eb961d3d7e3
	k8s.io/api v0.33.3
	k8s.io/apimachinery v0.33.3
	k8s.io/client-go v0.33.3
	k8s.io/component-base v0.33.3
	k8s.io/klog/v2 v2.130.1
	sigs.k8s.io/controller-runtime v0.21.0
)
```

Run: `go mod tidy`

### Phase 2: Port Core Files (Day 1-2)

#### Step 2.1: Port pkg/controller/ec2_instance_types.go

**Source:** `/Users/jjaggars/code/machine-api-provider-aws/pkg/actuators/machineset/ec2_instance_types.go`
**Target:** `/Users/jjaggars/code/capi-annotator/pkg/controller/ec2_instance_types.go`

**Changes Required:**
1. Line 14: Change package from `package machineset` to `package controller`
2. Line 23: Update import `awsclient "github.com/openshift/machine-api-provider-aws/pkg/client"` to `awsclient "github.com/jhjaggars/capi-annotator/pkg/client"`
3. Keep everything else unchanged (lines 1-216)

**Key Components to Preserve:**
- `InstanceType` struct (lines 38-44)
- `InstanceTypesCache` interface (lines 47-49)
- `NewInstanceTypesCache()` function (lines 64-69)
- `GetInstanceType()` method with caching logic (lines 73-96)
- `fetchEC2InstanceTypes()` with pagination (lines 126-164)
- `normalizeArchitecture()` function (lines 205-215)

#### Step 2.2: Port pkg/client/client.go

**Source:** `/Users/jjaggars/code/machine-api-provider-aws/pkg/client/client.go`
**Target:** `/Users/jjaggars/code/capi-annotator/pkg/client/client.go`

**Changes Required:**
1. Line 29: Update import `"github.com/openshift/machine-api-provider-aws/pkg/version"` to `"github.com/jhjaggars/capi-annotator/pkg/version"` (need to create version package)
2. Keep all other code unchanged (lines 1-517)

**Key Components to Preserve:**
- `Client` interface (lines 75-98)
- `awsClient` struct and all methods (lines 100-188)
- `NewClient()` function (lines 194-205)
- `NewValidatedClient()` with region validation (lines 317-355)
- `newAWSSession()` with credentials, CA bundles, endpoints (lines 357-413)
- `RegionCache` interface and implementation (lines 244-292)
- `NewRegionCache()` function (lines 250-255)

**Additional File Needed:**
Create `pkg/version/version.go`:
```go
package version

var (
	String = "v0.1.0"
)
```

#### Step 2.3: Port pkg/utils/providerspec.go

**Source:** `/Users/jjaggars/code/machine-api-provider-aws/pkg/actuators/machine/utils.go` (function only)
**Target:** `/Users/jjaggars/code/capi-annotator/pkg/utils/providerspec.go`

**Content:**
```go
package utils

import (
	"encoding/json"
	"fmt"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

// ProviderSpecFromRawExtension unmarshals a raw extension into an AWSMachineProviderSpec type
func ProviderSpecFromRawExtension(rawExtension *runtime.RawExtension) (*machinev1beta1.AWSMachineProviderConfig, error) {
	if rawExtension == nil {
		return &machinev1beta1.AWSMachineProviderConfig{}, nil
	}

	spec := new(machinev1beta1.AWSMachineProviderConfig)
	if err := json.Unmarshal(rawExtension.Raw, &spec); err != nil {
		return nil, fmt.Errorf("error unmarshalling providerSpec: %v", err)
	}

	klog.V(5).Infof("Got provider Spec from raw extension: %+v", spec)
	return spec, nil
}
```

#### Step 2.4: Port pkg/controller/controller.go

**Source:** `/Users/jjaggars/code/machine-api-provider-aws/pkg/actuators/machineset/controller.go`
**Target:** `/Users/jjaggars/code/capi-annotator/pkg/controller/controller.go`

**Changes Required:**

1. **Package and Imports (lines 1-26):**
   - Change package to `controller`
   - Update import on line 15: `utils "github.com/openshift/machine-api-provider-aws/pkg/actuators/machine"` to `utils "github.com/jhjaggars/capi-annotator/pkg/utils"`
   - Update import on line 16: `awsclient "github.com/openshift/machine-api-provider-aws/pkg/client"` to `awsclient "github.com/jhjaggars/capi-annotator/pkg/client"`
   - Remove line 9 import: `openshiftfeatures "github.com/openshift/api/features"` (not needed)
   - Remove line 21 import: `"k8s.io/component-base/featuregate"` (not needed)

2. **Constants (lines 28-36):**
   - Keep unchanged

3. **Reconciler Struct (lines 38-50):**
   - Remove line 46: `Gate                featuregate.MutableFeatureGate` (not needed)
   - Keep all other fields

4. **SetupWithManager (lines 52-66):**
   - Keep unchanged

5. **Reconcile Method (lines 68-130):**
   - Keep lines 68-88 (entry point, get MachineSet, handle deletion)
   - **REMOVE lines 90-109** (feature gate and paused condition logic)
   - Keep lines 111-130 (patch setup, reconcile call, error handling)

6. **isInvalidConfigurationError (lines 132-140):**
   - Keep unchanged

7. **reconcile Method (lines 142-182):**
   - Keep unchanged

**Final controller.go structure:**
```go
package controller

// imports (updated paths)

const (
	cpuKey    = "machine.openshift.io/vCPU"
	memoryKey = "machine.openshift.io/memoryMb"
	gpuKey    = "machine.openshift.io/GPU"
	labelsKey = "capacity.cluster-autoscaler.kubernetes.io/labels"
)

type Reconciler struct {
	Client              client.Client
	Log                 logr.Logger
	AwsClientBuilder    awsclient.AwsClientBuilderFuncType
	RegionCache         awsclient.RegionCache
	ConfigManagedClient client.Client
	InstanceTypesCache  InstanceTypesCache

	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	// unchanged
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("machineset", req.Name, "namespace", req.Namespace)
	logger.V(3).Info("Reconciling")

	machineSet := &machinev1beta1.MachineSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, machineSet); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Skip deleted MachineSets
	if !machineSet.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// REMOVED: Feature gate and paused condition logic (lines 90-109)

	originalMachineSetToPatch := client.MergeFrom(machineSet.DeepCopy())

	result, err := r.reconcile(machineSet)
	if err != nil {
		logger.Error(err, "Failed to reconcile MachineSet")
		r.recorder.Eventf(machineSet, corev1.EventTypeWarning, "ReconcileError", "%v", err)
	}

	if err := r.Client.Patch(ctx, machineSet, originalMachineSetToPatch); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch machineSet: %v", err)
	}

	if isInvalidConfigurationError(err) {
		return result, nil
	}
	return result, err
}

func isInvalidConfigurationError(err error) bool {
	// unchanged
}

func (r *Reconciler) reconcile(machineSet *machinev1beta1.MachineSet) (ctrl.Result, error) {
	// unchanged - full implementation from lines 142-182
}
```

### Phase 3: Create Main Entry Point (Day 2)

#### Step 3.1: Create cmd/controller/main.go

**Source:** `/Users/jjaggars/code/machine-api-provider-aws/cmd/manager/main.go`
**Target:** `/Users/jjaggars/code/capi-annotator/cmd/controller/main.go`

**Simplified version (no machine actuator, no feature gates):**

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"github.com/openshift/machine-api-operator/pkg/metrics"
	machinesetcontroller "github.com/jhjaggars/capi-annotator/pkg/controller"
	awsclient "github.com/jhjaggars/capi-annotator/pkg/client"
	"github.com/jhjaggars/capi-annotator/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	leaseDuration = 120 * time.Second
	renewDeadline  = 110 * time.Second
	retryPeriod   = 20 * time.Second
)

func main() {
	printVersion := flag.Bool("version", false, "print version and exit")
	metricsAddress := flag.String("metrics-bind-address", metrics.DefaultMachineMetricsAddress, "Address for hosting metrics")
	watchNamespace := flag.String("namespace", "", "Namespace that the controller watches to reconcile machine-api objects")
	leaderElectResourceNamespace := flag.String("leader-elect-resource-namespace", "", "The namespace of resource object that is used for locking during leader election")
	leaderElect := flag.Bool("leader-elect", false, "Start a leader election client and gain leadership before executing the main loop")
	leaderElectLeaseDuration := flag.Duration("leader-elect-lease-duration", leaseDuration, "The duration that non-leader candidates will wait after observing a leadership renewal")
	healthAddr := flag.String("health-addr", ":9440", "The address for health checking")

	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()

	if *printVersion {
		fmt.Println(version.String)
		os.Exit(0)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		klog.Fatalf("Error getting configuration: %v", err)
	}

	syncPeriod := 10 * time.Minute
	opts := manager.Options{
		LeaderElection:          *leaderElect,
		LeaderElectionNamespace: *leaderElectResourceNamespace,
		LeaderElectionID:        "machineset-annotator-leader",
		LeaseDuration:           leaderElectLeaseDuration,
		HealthProbeBindAddress:  *healthAddr,
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		Metrics: server.Options{
			BindAddress: *metricsAddress,
		},
		RetryPeriod:   &retryPeriod,
		RenewDeadline: &renewDeadline,
	}

	if *watchNamespace != "" {
		opts.Cache.DefaultNamespaces = map[string]cache.Config{
			*watchNamespace: {},
		}
		klog.Infof("Watching machine-api objects only in namespace %q", *watchNamespace)
	}

	mgr, err := manager.New(cfg, opts)
	if err != nil {
		klog.Fatalf("Error creating manager: %v", err)
	}

	// Setup Schemes
	if err := machinev1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatalf("Error setting up scheme: %v", err)
	}
	if err := machinev1.Install(mgr.GetScheme()); err != nil {
		klog.Fatalf("Error setting up scheme: %v", err)
	}
	if err := configv1.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatal(err)
	}
	if err := corev1.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatal(err)
	}

	configManagedClient, startCache, err := newConfigManagedClient(mgr)
	if err != nil {
		klog.Fatal(err)
	}
	mgr.Add(startCache)

	describeRegionsCache := awsclient.NewRegionCache()

	ctrl.SetLogger(klogr.New())
	setupLog := ctrl.Log.WithName("setup")

	if err := (&machinesetcontroller.Reconciler{
		Client:              mgr.GetClient(),
		Log:                 ctrl.Log.WithName("controllers").WithName("MachineSet"),
		AwsClientBuilder:    awsclient.NewValidatedClient,
		RegionCache:         describeRegionsCache,
		ConfigManagedClient: configManagedClient,
		InstanceTypesCache:  machinesetcontroller.NewInstanceTypesCache(),
	}).SetupWithManager(mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MachineSet")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		klog.Fatal(err)
	}
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.Fatal(err)
	}

	klog.Info("Starting MachineSet Annotator Controller")
	err = mgr.Start(ctrl.SetupSignalHandler())
	if err != nil {
		klog.Fatalf("Error starting manager: %v", err)
	}
}

func newConfigManagedClient(mgr manager.Manager) (runtimeclient.Client, manager.Runnable, error) {
	cacheOpts := cache.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
		DefaultNamespaces: map[string]cache.Config{
			awsclient.KubeCloudConfigNamespace: {},
		},
	}
	cache, err := cache.New(mgr.GetConfig(), cacheOpts)
	if err != nil {
		return nil, nil, err
	}

	c, err := runtimeclient.New(mgr.GetConfig(), runtimeclient.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
		Cache: &runtimeclient.CacheOptions{
			Reader: cache,
		},
	})
	return c, cache, err
}
```

### Phase 4: Build Infrastructure (Day 3)

#### Step 4.1: Create Makefile

```makefile
# Module path
MODULE = github.com/jhjaggars/capi-annotator

# Binary name
BINARY = machineset-annotator

# Image settings
IMAGE_TAG ?= latest
IMAGE_REPO ?= quay.io/jhjaggars/machineset-annotator

# Go settings
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: all
all: build

.PHONY: build
build:
	@echo "Building $(BINARY)..."
	go build -o bin/$(BINARY) ./cmd/controller

.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...

.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run

.PHONY: tidy
tidy:
	@echo "Tidying go.mod..."
	go mod tidy

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf bin/

.PHONY: image
image:
	@echo "Building container image..."
	docker build -t $(IMAGE_REPO):$(IMAGE_TAG) .

.PHONY: push
push:
	@echo "Pushing container image..."
	docker push $(IMAGE_REPO):$(IMAGE_TAG)

.PHONY: run
run:
	@echo "Running controller locally..."
	go run ./cmd/controller/main.go

.PHONY: verify
verify: fmt vet test
	@echo "Verification complete"
```

#### Step 4.2: Create Dockerfile

```dockerfile
# Build stage
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.23-openshift-4.18 AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Cache dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o machineset-annotator ./cmd/controller

# Runtime stage
FROM registry.ci.openshift.org/ocp/4.18:base-rhel9

WORKDIR /
COPY --from=builder /workspace/machineset-annotator .
USER 65532:65532

ENTRYPOINT ["/machineset-annotator"]
```

### Phase 5: Kubernetes Manifests (Day 3)

#### Step 5.1: Create deploy/ directory structure

```bash
mkdir -p deploy
```

#### Step 5.2: Create deploy/rbac.yaml

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: machineset-annotator
  namespace: openshift-machine-api
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: machineset-annotator
rules:
- apiGroups:
  - machine.openshift.io
  resources:
  - machinesets
  verbs:
  - get
  - list
  - watch
  - update
  - patch
- apiGroups:
  - config.openshift.io
  resources:
  - infrastructures
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - secrets
  - configmaps
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: machineset-annotator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: machineset-annotator
subjects:
- kind: ServiceAccount
  name: machineset-annotator
  namespace: openshift-machine-api
```

#### Step 5.3: Create deploy/deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: machineset-annotator
  namespace: openshift-machine-api
  labels:
    app: machineset-annotator
spec:
  replicas: 2
  selector:
    matchLabels:
      app: machineset-annotator
  template:
    metadata:
      labels:
        app: machineset-annotator
    spec:
      serviceAccountName: machineset-annotator
      containers:
      - name: controller
        image: quay.io/jhjaggars/machineset-annotator:latest
        imagePullPolicy: Always
        command:
        - /machineset-annotator
        args:
        - --leader-elect=true
        - --leader-elect-lease-duration=120s
        - --metrics-bind-address=:8080
        - --health-addr=:9440
        ports:
        - name: metrics
          containerPort: 8080
          protocol: TCP
        - name: health
          containerPort: 9440
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: health
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: health
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: machineset-annotator-metrics
  namespace: openshift-machine-api
  labels:
    app: machineset-annotator
spec:
  ports:
  - name: metrics
    port: 8080
    targetPort: metrics
  selector:
    app: machineset-annotator
```

### Phase 6: Documentation (Day 4)

#### Step 6.1: Create README.md

```markdown
# MachineSet Annotator

A standalone Kubernetes controller that automatically annotates MachineSet objects with CPU, memory, GPU, and architecture information by querying AWS EC2 instance type data.

## Overview

This controller enables cluster-autoscaler to scale from zero by providing capacity information before machines are created. It watches MachineSets, queries AWS EC2 API for instance type information, and sets annotations that the autoscaler uses for scaling decisions.

## Annotations Set

- `machine.openshift.io/vCPU` - Number of vCPUs for the instance type
- `machine.openshift.io/memoryMb` - Memory in MB for the instance type
- `machine.openshift.io/GPU` - Number of GPUs for the instance type
- `capacity.cluster-autoscaler.kubernetes.io/labels` - Architecture label (e.g., `kubernetes.io/arch=amd64`)

## Features

- ✅ Watches MachineSets across all namespaces (or specific namespace)
- ✅ Queries AWS EC2 instance type information with caching (24-hour refresh)
- ✅ Supports custom AWS endpoints (GovCloud, C2S, etc.)
- ✅ Supports custom CA bundles
- ✅ Thread-safe caching for concurrent reconciliation
- ✅ Leader election for HA deployments
- ✅ Prometheus metrics endpoint
- ✅ Health and readiness probes

## Installation

### Prerequisites

- Kubernetes 1.28+
- OpenShift 4.14+ (for MachineSet CRD)
- AWS credentials in Kubernetes secrets

### Deploy to OpenShift

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
```

## Configuration

### Command-line Flags

- `--metrics-bind-address` - Metrics endpoint (default: `:8080`)
- `--namespace` - Watch specific namespace (default: all namespaces)
- `--leader-elect` - Enable leader election (default: false)
- `--leader-elect-lease-duration` - Lease duration (default: 120s)
- `--health-addr` - Health check address (default: `:9440`)

### AWS Credentials

AWS credentials are read from Kubernetes secrets referenced in the MachineSet provider spec:

```yaml
spec:
  template:
    spec:
      providerSpec:
        value:
          credentialsSecret:
            name: aws-credentials
```

The secret must contain:
- `aws_access_key_id`
- `aws_secret_access_key`

## Development

### Build

```bash
make build
```

### Run Locally

```bash
make run
```

### Run Tests

```bash
make test
```

### Build Container Image

```bash
make image IMAGE_TAG=v0.1.0
```

### Format and Verify

```bash
make verify
```

## Architecture

The controller consists of:

1. **Controller Reconciler**: Watches MachineSets and orchestrates annotation logic
2. **Instance Types Cache**: Thread-safe cache with 24-hour refresh per region
3. **AWS Client**: Wraps AWS SDK with credential and endpoint support
4. **Provider Spec Parser**: Unmarshals AWSMachineProviderConfig

## License

Apache 2.0

## Contributing

Contributions welcome! Please open an issue or pull request.
```

## Files to Modify/Create Summary

| File | Action | Source | Notes |
|------|--------|--------|-------|
| `go.mod` | Create | New | Module path: `github.com/jhjaggars/capi-annotator` |
| `pkg/controller/ec2_instance_types.go` | Port | `machine-api-provider-aws/pkg/actuators/machineset/ec2_instance_types.go` | Update package, imports |
| `pkg/controller/controller.go` | Port | `machine-api-provider-aws/pkg/actuators/machineset/controller.go` | Remove lines 90-109 (feature gates) |
| `pkg/client/client.go` | Port | `machine-api-provider-aws/pkg/client/client.go` | Update imports |
| `pkg/utils/providerspec.go` | Extract | `machine-api-provider-aws/pkg/actuators/machine/utils.go` | Lines 467-480 only |
| `pkg/version/version.go` | Create | New | Simple version string |
| `cmd/controller/main.go` | Adapt | `machine-api-provider-aws/cmd/manager/main.go` | Simplified (no machine actuator) |
| `Makefile` | Create | New | Build, test, image targets |
| `Dockerfile` | Create | New | Multi-stage build |
| `deploy/rbac.yaml` | Create | New | RBAC manifests |
| `deploy/deployment.yaml` | Create | New | Deployment and Service |
| `README.md` | Create | New | Documentation |

## Dependencies

### Go Module Dependencies

```
github.com/aws/aws-sdk-go v1.55.7
github.com/go-logr/logr v1.4.2
github.com/openshift/api v0.0.0-20250108101736-59f72e9e2bf6
github.com/openshift/machine-api-operator v0.2.1-0.20241212154333-7eb961d3d7e3
k8s.io/api v0.33.3
k8s.io/apimachinery v0.33.3
k8s.io/client-go v0.33.3
k8s.io/component-base v0.33.3
k8s.io/klog/v2 v2.130.1
sigs.k8s.io/controller-runtime v0.21.0
```

## Testing Strategy

### Unit Tests to Port/Create

1. **Instance Type Cache Tests** (`pkg/controller/ec2_instance_types_test.go`):
   - Test cache initialization
   - Test cache refresh logic
   - Test thread safety with concurrent access
   - Test architecture normalization
   - Test pagination handling

2. **Controller Tests** (`pkg/controller/controller_test.go`):
   - Test MachineSet reconciliation
   - Test annotation setting
   - Test deletion timestamp handling
   - Test error handling for unknown instance types
   - Test provider spec parsing errors

3. **Client Tests** (`pkg/client/client_test.go`):
   - Test AWS session creation
   - Test credential loading from secrets
   - Test custom CA bundle support
   - Test region validation

### Integration Tests

- End-to-end test with fake AWS client
- Test full reconciliation flow
- Test cache refresh behavior

## Deployment Considerations

### Namespace
Deploy to `openshift-machine-api` namespace alongside other machine-api components.

### Replicas
- Development: 1 replica
- Production: 2-3 replicas with leader election enabled

### Resource Requirements
- CPU: 100m (request), 500m (limit)
- Memory: 128Mi (request), 512Mi (limit)

### Network Requirements
- Outbound access to AWS EC2 API endpoints
- Access to Kubernetes API server

### RBAC Requirements
- Read: MachineSets, Infrastructures, Secrets, ConfigMaps
- Write: MachineSets (update/patch), Events (create/patch)

## Risks and Mitigation

### Risk 1: AWS API Rate Limiting
**Description:** Frequent DescribeInstanceTypes calls could hit AWS API rate limits.
**Mitigation:** Instance type cache with 24-hour refresh cycle minimizes API calls.

### Risk 2: Unknown Instance Types
**Description:** New AWS instance types might not be in cache.
**Mitigation:** Cache auto-refreshes every 24 hours. Controller logs warnings and emits events.

### Risk 3: Credential Access
**Description:** Controller needs access to AWS credentials in secrets.
**Mitigation:** RBAC strictly limits secret access to get/list/watch only.

### Risk 4: Concurrent Reconciliation
**Description:** Multiple controller replicas could race on cache updates.
**Mitigation:** Thread-safe cache with RWMutex ensures consistency.

## Implementation Timeline

- **Day 1**: Project initialization, port ec2_instance_types.go, port client.go
- **Day 2**: Port controller.go, create main.go, port utils
- **Day 3**: Create Makefile, Dockerfile, Kubernetes manifests
- **Day 4**: Documentation, unit tests, integration tests
- **Day 5**: E2E testing, bug fixes, final validation

## Validation Checklist

- [ ] Go module initializes successfully
- [ ] All dependencies resolve
- [ ] Code compiles without errors
- [ ] Unit tests pass
- [ ] Container image builds
- [ ] Deployment applies to cluster
- [ ] Controller starts and becomes ready
- [ ] MachineSets get annotated correctly
- [ ] Unknown instance types handled gracefully
- [ ] Custom CA bundles work
- [ ] Leader election functions in HA setup
- [ ] Metrics endpoint responds
- [ ] Health probes succeed
