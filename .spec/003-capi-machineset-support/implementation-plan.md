# Implementation Plan - CAPI MachineDeployment Support

## Overview
This implementation plan provides a step-by-step guide for migrating the capa-annotator controller from OpenShift Machine API (MAPI) to Kubernetes Cluster API (CAPI). The migration involves replacing MAPI MachineSet resources with CAPI MachineDeployment resources, removing OpenShift dependencies, and implementing infrastructure template resolution.

## Technical Architecture

### Current Architecture (MAPI)
```
MachineSet (machine.openshift.io/v1beta1)
  └─> providerSpec.value (embedded AWSMachineProviderConfig)
      ├─> instanceType: "m5.large"
      ├─> placement.region: "us-east-1"
      └─> credentialsSecret.name: "aws-creds"
```

### Target Architecture (CAPI)
```
MachineDeployment (cluster.x-k8s.io/v1beta1)
  ├─> spec.clusterName → Cluster
  │                       └─> infrastructureRef → AWSCluster
  │                                                └─> spec.region: "us-east-1"
  └─> spec.template.spec.infrastructureRef → AWSMachineTemplate
                                               └─> spec.template.spec.instanceType: "m5.large"

Authentication: IRSA (AWS_ROLE_ARN + AWS_WEB_IDENTITY_TOKEN_FILE)
```

### Component Responsibilities

**Unchanged Components:**
- `pkg/controller/ec2_instance_types.go` - EC2 instance type caching
- `pkg/client/client.go` (partial) - AWS session creation, IRSA auth, region cache

**Modified Components:**
- `pkg/controller/controller.go` - Watch MachineDeployment, orchestrate reconciliation
- `pkg/utils/providerspec.go` - Resolve CAPI templates instead of MAPI provider specs
- `pkg/client/client.go` - Remove OpenShift-specific code
- `cmd/controller/main.go` - Update scheme registration

**New Logic:**
- Infrastructure template resolution (AWSMachineTemplate fetch)
- Region resolution with AWSCluster fallback
- Simplified error handling without OpenShift error types

## Implementation Steps

### Phase 1: Dependency Management (30 minutes)

#### Step 1.1: Update go.mod with CAPI dependencies
```bash
# Add CAPI dependencies
go get sigs.k8s.io/cluster-api@v1.8.5
go get sigs.k8s.io/cluster-api-provider-aws/v2@v2.6.1

# Remove OpenShift dependencies
go mod edit -droprequire github.com/openshift/api
go mod edit -droprequire github.com/openshift/machine-api-operator
go mod edit -droprequire github.com/openshift/library-go

# Clean up
go mod tidy
```

**Verification:**
- Check `go.mod` contains `sigs.k8s.io/cluster-api v1.8.5`
- Check `go.mod` contains `sigs.k8s.io/cluster-api-provider-aws/v2 v2.6.1`
- Check no `github.com/openshift` dependencies remain (except in vendor if applicable)
- Run `go mod verify` to ensure integrity

---

### Phase 2: Create CAPI Template Resolution Logic (1 hour)

#### Step 2.1: Rewrite pkg/utils/providerspec.go

**File:** `pkg/utils/providerspec.go`

**Action:** Replace entire file content with:

```go
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

package utils

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/klog/v2"
)

const (
	// RegionAnnotation is the fallback annotation for AWS region
	RegionAnnotation = "capa.infrastructure.cluster.x-k8s.io/region"
)

// ResolveAWSMachineTemplate fetches the AWSMachineTemplate referenced by the MachineDeployment
func ResolveAWSMachineTemplate(ctx context.Context, c client.Client, machineDeployment *clusterv1.MachineDeployment) (*infrav1.AWSMachineTemplate, error) {
	// Extract infrastructureRef
	infraRef := machineDeployment.Spec.Template.Spec.InfrastructureRef
	if infraRef.Name == "" {
		return nil, fmt.Errorf("infrastructureRef.name is empty")
	}

	// Validate it's an AWSMachineTemplate
	if infraRef.Kind != "AWSMachineTemplate" {
		return nil, fmt.Errorf("expected AWSMachineTemplate, got %s", infraRef.Kind)
	}

	// Fetch the template
	template := &infrav1.AWSMachineTemplate{}
	key := client.ObjectKey{
		Name:      infraRef.Name,
		Namespace: infraRef.Namespace,
	}
	// Use same namespace as MachineDeployment if not specified
	if key.Namespace == "" {
		key.Namespace = machineDeployment.Namespace
	}

	if err := c.Get(ctx, key, template); err != nil {
		return nil, fmt.Errorf("failed to fetch AWSMachineTemplate %s/%s: %w", key.Namespace, key.Name, err)
	}

	klog.V(3).Infof("Resolved AWSMachineTemplate %s/%s for MachineDeployment %s", key.Namespace, key.Name, machineDeployment.Name)
	return template, nil
}

// ExtractInstanceType gets the instance type from AWSMachineTemplate
func ExtractInstanceType(template *infrav1.AWSMachineTemplate) (string, error) {
	if template.Spec.Template.Spec.InstanceType == "" {
		return "", fmt.Errorf("instanceType is empty in AWSMachineTemplate")
	}
	return template.Spec.Template.Spec.InstanceType, nil
}

// ResolveRegion attempts to get AWS region from AWSCluster, falls back to annotation
func ResolveRegion(ctx context.Context, c client.Client, machineDeployment *clusterv1.MachineDeployment) (string, error) {
	// Try to get region from AWSCluster
	if machineDeployment.Spec.ClusterName != "" {
		region, err := getRegionFromAWSCluster(ctx, c, machineDeployment)
		if err == nil {
			return region, nil
		}
		klog.V(3).Infof("Failed to get region from AWSCluster: %v, trying annotation fallback", err)
	}

	// Fallback to annotation
	if region, ok := machineDeployment.Annotations[RegionAnnotation]; ok && region != "" {
		klog.V(3).Infof("Using region %s from annotation %s", region, RegionAnnotation)
		return region, nil
	}

	return "", fmt.Errorf("unable to determine AWS region from AWSCluster or annotation %s", RegionAnnotation)
}

// getRegionFromAWSCluster fetches region from the AWSCluster resource
func getRegionFromAWSCluster(ctx context.Context, c client.Client, machineDeployment *clusterv1.MachineDeployment) (string, error) {
	// Fetch the Cluster resource
	cluster := &clusterv1.Cluster{}
	clusterKey := client.ObjectKey{
		Name:      machineDeployment.Spec.ClusterName,
		Namespace: machineDeployment.Namespace,
	}

	if err := c.Get(ctx, clusterKey, cluster); err != nil {
		return "", fmt.Errorf("failed to fetch Cluster %s/%s: %w", clusterKey.Namespace, clusterKey.Name, err)
	}

	// Fetch AWSCluster
	if cluster.Spec.InfrastructureRef == nil {
		return "", fmt.Errorf("cluster %s has nil infrastructureRef", cluster.Name)
	}

	awsCluster := &infrav1.AWSCluster{}
	awsClusterKey := client.ObjectKey{
		Name:      cluster.Spec.InfrastructureRef.Name,
		Namespace: cluster.Spec.InfrastructureRef.Namespace,
	}
	if awsClusterKey.Namespace == "" {
		awsClusterKey.Namespace = cluster.Namespace
	}

	if err := c.Get(ctx, awsClusterKey, awsCluster); err != nil {
		return "", fmt.Errorf("failed to fetch AWSCluster %s/%s: %w", awsClusterKey.Namespace, awsClusterKey.Name, err)
	}

	if awsCluster.Spec.Region == "" {
		return "", fmt.Errorf("AWSCluster %s has empty region", awsCluster.Name)
	}

	klog.V(3).Infof("Resolved region %s from AWSCluster %s", awsCluster.Spec.Region, awsClusterKey.Name)
	return awsCluster.Spec.Region, nil
}
```

**Verification:**
- File compiles without errors
- All imports are from CAPI packages
- No MAPI references remain

---

### Phase 3: Update AWS Client (Remove OpenShift Dependencies) (45 minutes)

#### Step 3.1: Simplify pkg/client/client.go

**File:** `pkg/client/client.go`

**Changes:**

1. **Remove imports (lines 44-45):**
```go
// DELETE these lines:
configv1 "github.com/openshift/api/config/v1"
machineapiapierrors "github.com/openshift/machine-api-operator/pkg/controller/machine"
```

2. **Remove constants (lines 57-68):**
```go
// DELETE these constants:
GlobalInfrastuctureName = "cluster"
KubeCloudConfigNamespace = "openshift-config-managed"
kubeCloudConfigName = "kube-cloud-config"
cloudCABundleKey = "ca-bundle.pem"
awsRegionsCacheExpirationDuration = time.Minute * 30
```

**Keep:** `awsRegionsCacheExpirationDuration = time.Minute * 30` (it's used by region cache)

3. **Update AwsClientBuilderFuncType (line 71):**
```go
// CHANGE FROM:
type AwsClientBuilderFuncType func(client client.Client, secretName, namespace, region string, configManagedClient client.Client, regionCache RegionCache) (Client, error)

// CHANGE TO:
type AwsClientBuilderFuncType func(client client.Client, secretName, namespace, region string, regionCache RegionCache) (Client, error)
```

4. **Update NewClient function signature (line 193):**
```go
// CHANGE FROM:
func NewClient(ctrlRuntimeClient client.Client, secretName, namespace, region string, configManagedClient client.Client) (Client, error) {

// CHANGE TO:
func NewClient(ctrlRuntimeClient client.Client, secretName, namespace, region string) (Client, error) {
	s, err := newAWSSession(ctrlRuntimeClient, secretName, namespace, region)
	// ... rest unchanged
}
```

5. **Update NewValidatedClient function signature (line 316):**
```go
// CHANGE FROM:
func NewValidatedClient(ctrlRuntimeClient client.Client, secretName, namespace, region string, configManagedClient client.Client, regionCache RegionCache) (Client, error) {

// CHANGE TO:
func NewValidatedClient(ctrlRuntimeClient client.Client, secretName, namespace, region string, regionCache RegionCache) (Client, error) {
	s, err := newAWSSession(ctrlRuntimeClient, secretName, namespace, region)
	// ... rest unchanged
}
```

6. **Simplify newAWSSession function (lines 356-431):**

**REPLACE ENTIRE FUNCTION WITH:**
```go
func newAWSSession(ctrlRuntimeClient client.Client, secretName, namespace, region string) (*session.Session, error) {
	sessionOptions := session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}

	// Check for IRSA environment variables (only auth method)
	roleARN := os.Getenv("AWS_ROLE_ARN")
	tokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")

	// Validate IRSA configuration - both variables must be present
	if roleARN == "" || tokenFile == "" {
		return nil, fmt.Errorf("IRSA not configured: AWS_ROLE_ARN and AWS_WEB_IDENTITY_TOKEN_FILE environment variables required")
	}

	klog.Infof("Using IRSA authentication with role: %s", roleARN)
	// AWS SDK v1 will automatically detect and use web identity credentials
	// from the environment variables - no explicit configuration needed

	// Create AWS session with the configured options
	s, err := session.NewSessionWithOptions(sessionOptions)
	if err != nil {
		return nil, err
	}

	s.Handlers.Build.PushBackNamed(addProviderVersionToUserAgent)

	return s, nil
}
```

7. **Delete these entire functions:**
- `resolveEndpoints()` (lines 440-473)
- `buildCustomEndpointsMap()` (lines 475-484)
- `sharedCredentialsFileFromSecret()` (lines 486-519)
- `newConfigForStaticCreds()` (line 513-519)
- `useCustomCABundle()` (lines 521-544)

**Verification:**
- File compiles without errors
- No OpenShift imports remain
- IRSA-only authentication
- No secret-based credential handling

---

### Phase 4: Update Controller (1.5 hours)

#### Step 4.1: Update pkg/controller/controller.go

**File:** `pkg/controller/controller.go`

**Changes:**

1. **Update imports (lines 13-15):**
```go
// REMOVE:
machinev1beta1 "github.com/openshift/api/machine/v1beta1"
openshiftfeatures "github.com/openshift/api/features"
mapierrors "github.com/openshift/machine-api-operator/pkg/controller/machine"
"github.com/openshift/machine-api-operator/pkg/util"
"github.com/openshift/machine-api-operator/pkg/util/conditions"

// ADD:
clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
```

2. **Keep annotation constants unchanged (lines 32-35):**
```go
// NO CHANGES - these remain for cluster-autoscaler compatibility
const (
	cpuKey    = "machine.openshift.io/vCPU"
	memoryKey = "machine.openshift.io/memoryMb"
	gpuKey    = "machine.openshift.io/GPU"
	labelsKey = "capacity.cluster-autoscaler.kubernetes.io/labels"
)
```

3. **Update Reconciler struct (lines 38-50):**
```go
// CHANGE FROM:
type Reconciler struct {
	Client              client.Client
	Log                 logr.Logger
	AwsClientBuilder    awsclient.AwsClientBuilderFuncType
	RegionCache         awsclient.RegionCache
	ConfigManagedClient client.Client
	InstanceTypesCache  InstanceTypesCache
	Gate                featuregate.MutableFeatureGate

	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

// CHANGE TO:
type Reconciler struct {
	Client             client.Client
	Log                logr.Logger
	AwsClientBuilder   awsclient.AwsClientBuilderFuncType
	RegionCache        awsclient.RegionCache
	InstanceTypesCache InstanceTypesCache

	recorder record.EventRecorder
	scheme   *runtime.Scheme
}
```

4. **Update SetupWithManager (line 55):**
```go
// CHANGE FROM:
For(&machinev1beta1.MachineSet{}).

// CHANGE TO:
For(&clusterv1.MachineDeployment{}).
```

5. **Update Reconcile function (lines 68-130):**
```go
// CHANGE resource type (line 73):
// FROM:
machineSet := &machinev1beta1.MachineSet{}

// TO:
machineDeployment := &clusterv1.MachineDeployment{}

// UPDATE Get call (line 74):
if err := r.Client.Get(ctx, req.NamespacedName, machineDeployment); err != nil {

// UPDATE deletion check (line 86):
if !machineDeployment.DeletionTimestamp.IsZero() {

// DELETE entire feature gate block (lines 90-109) - REMOVE COMPLETELY

// UPDATE patch setup (line 111):
originalMachineDeploymentToPatch := client.MergeFrom(machineDeployment.DeepCopy())

// UPDATE reconcile call (line 113):
result, err := r.reconcile(machineDeployment)

// UPDATE patch call (line 120):
if err := r.Client.Patch(ctx, machineDeployment, originalMachineDeploymentToPatch); err != nil {
	return ctrl.Result{}, fmt.Errorf("failed to patch machineDeployment: %v", err)
}

// DELETE isInvalidConfigurationError function (lines 132-140) - REMOVE COMPLETELY
```

6. **Rewrite reconcile function (lines 142-196):**

**REPLACE ENTIRE FUNCTION WITH:**
```go
func (r *Reconciler) reconcile(machineDeployment *clusterv1.MachineDeployment) (ctrl.Result, error) {
	klog.V(3).Infof("%v: Reconciling MachineDeployment", machineDeployment.Name)

	// Resolve AWSMachineTemplate
	awsMachineTemplate, err := utils.ResolveAWSMachineTemplate(context.Background(), r.Client, machineDeployment)
	if err != nil {
		klog.Errorf("Failed to resolve AWSMachineTemplate: %v", err)
		r.recorder.Eventf(machineDeployment, corev1.EventTypeWarning, "FailedUpdate", "Failed to resolve AWSMachineTemplate: %v", err)
		return ctrl.Result{}, err
	}

	// Extract instance type
	instanceType, err := utils.ExtractInstanceType(awsMachineTemplate)
	if err != nil {
		klog.Errorf("Failed to extract instance type: %v", err)
		r.recorder.Eventf(machineDeployment, corev1.EventTypeWarning, "FailedUpdate", "Failed to extract instance type: %v", err)
		return ctrl.Result{}, err
	}

	// Resolve AWS region
	region, err := utils.ResolveRegion(context.Background(), r.Client, machineDeployment)
	if err != nil {
		klog.Errorf("Failed to resolve AWS region: %v", err)
		r.recorder.Eventf(machineDeployment, corev1.EventTypeWarning, "FailedUpdate", "Failed to resolve AWS region: %v", err)
		return ctrl.Result{}, err
	}

	// Validate IRSA is configured
	roleARN := os.Getenv("AWS_ROLE_ARN")
	tokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	if roleARN == "" || tokenFile == "" {
		err := fmt.Errorf("IRSA not configured: AWS_ROLE_ARN and AWS_WEB_IDENTITY_TOKEN_FILE environment variables required")
		klog.Error(err)
		r.recorder.Eventf(machineDeployment, corev1.EventTypeWarning, "FailedUpdate", "%v", err)
		return ctrl.Result{}, err
	}

	// Create AWS client (secretName is empty string for IRSA)
	awsClient, err := r.AwsClientBuilder(r.Client, "", machineDeployment.Namespace, region, r.RegionCache)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating aws client: %w", err)
	}

	// Get instance type information
	instanceTypeInfo, err := r.InstanceTypesCache.GetInstanceType(awsClient, region, instanceType)
	if err != nil {
		klog.Errorf("Unable to set scale from zero annotations: unknown instance type %s: %v", instanceType, err)
		klog.Errorf("Autoscaling from zero will not work. To fix this, manually populate machine annotations for your instance type: %v", []string{cpuKey, memoryKey, gpuKey})

		r.recorder.Eventf(machineDeployment, corev1.EventTypeWarning, "FailedUpdate", "Failed to set autoscaling from zero annotations, instance type unknown")
		return ctrl.Result{}, nil
	}

	// Set annotations
	if machineDeployment.Annotations == nil {
		machineDeployment.Annotations = make(map[string]string)
	}

	machineDeployment.Annotations[cpuKey] = strconv.FormatInt(instanceTypeInfo.VCPU, 10)
	machineDeployment.Annotations[memoryKey] = strconv.FormatInt(instanceTypeInfo.MemoryMb, 10)
	machineDeployment.Annotations[gpuKey] = strconv.FormatInt(instanceTypeInfo.GPU, 10)

	// Merge architecture label with existing labels
	archLabel := fmt.Sprintf("kubernetes.io/arch=%s", instanceTypeInfo.CPUArchitecture)
	if existingLabels, ok := machineDeployment.Annotations[labelsKey]; ok && existingLabels != "" {
		// Simple concatenation - preserve existing labels
		machineDeployment.Annotations[labelsKey] = existingLabels + "," + archLabel
	} else {
		machineDeployment.Annotations[labelsKey] = archLabel
	}

	return ctrl.Result{}, nil
}
```

7. **Add missing import:**
```go
import (
	// ... existing imports ...
	"os"
	"strconv"
)
```

**Verification:**
- File compiles without errors
- No MAPI types remain
- No feature gate references
- IRSA validation present

---

### Phase 5: Update Main Entry Point (30 minutes)

#### Step 5.1: Update cmd/controller/main.go

**File:** `cmd/controller/main.go`

**Changes:**

1. **Update imports (lines 23-26):**
```go
// REMOVE:
configv1 "github.com/openshift/api/config/v1"
apifeatures "github.com/openshift/api/features"
machinev1 "github.com/openshift/api/machine/v1"
machinev1beta1 "github.com/openshift/api/machine/v1beta1"
"github.com/openshift/library-go/pkg/features"
"github.com/openshift/machine-api-operator/pkg/metrics"

// ADD:
clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
```

2. **Keep corev1 import:**
```go
// KEEP:
corev1 "k8s.io/api/core/v1"
```

3. **Update metrics address default (line 64):**
```go
// CHANGE FROM:
metricsAddress := flag.String(
	"metrics-bind-address",
	metrics.DefaultMachineMetricsAddress,
	"Address for hosting metrics",
)

// CHANGE TO:
metricsAddress := flag.String(
	"metrics-bind-address",
	":8080",
	"Address for hosting metrics",
)
```

4. **Remove feature gate setup (lines 98-106):**
```go
// DELETE entire block:
defaultMutableGate := feature.DefaultMutableFeatureGate
gateOpts, err := features.NewFeatureGateOptions(defaultMutableGate, apifeatures.SelfManaged, apifeatures.FeatureGateMachineAPIMigration)
if err != nil {
	klog.Fatalf("Error setting up feature gates: %v", err)
}

// Add the --feature-gates flag
gateOpts.AddFlagsToGoFlagSet(nil)
```

5. **Remove feature gate logging (lines 151-161):**
```go
// DELETE:
// Sets feature gates from flags
klog.Infof("Initializing feature gates: %s", strings.Join(defaultMutableGate.KnownFeatures(), ", "))
warnings, err := gateOpts.ApplyTo(defaultMutableGate)
if err != nil {
	klog.Fatalf("Error setting feature gates from flags: %v", err)
}
if len(warnings) > 0 {
	klog.Infof("Warnings setting feature gates from flags: %v", warnings)
}

klog.Infof("FeatureGateMachineAPIMigration initialised: %t", defaultMutableGate.Enabled(featuregate.Feature(apifeatures.FeatureGateMachineAPIMigration)))
```

6. **Update scheme registration (lines 169-183):**
```go
// REPLACE:
if err := machinev1beta1.AddToScheme(mgr.GetScheme()); err != nil {
	klog.Fatalf("Error setting up scheme: %v", err)
}

if err := machinev1.Install(mgr.GetScheme()); err != nil {
	klog.Fatalf("Error setting up scheme: %v", err)
}

if err := configv1.AddToScheme(mgr.GetScheme()); err != nil {
	klog.Fatal(err)
}

// WITH:
if err := clusterv1.AddToScheme(mgr.GetScheme()); err != nil {
	klog.Fatalf("Error setting up CAPI scheme: %v", err)
}

if err := infrav1.AddToScheme(mgr.GetScheme()); err != nil {
	klog.Fatalf("Error setting up CAPA scheme: %v", err)
}
```

7. **Remove configManagedClient (lines 185-191):**
```go
// DELETE entire function call and variables:
configManagedClient, startCache, err := newConfigManagedClient(mgr)
if err != nil {
	klog.Fatal(err)
}
if err := mgr.Add(startCache); err != nil {
	klog.Fatalf("Error adding start cache to manager: %v", err)
}

// Also DELETE the newConfigManagedClient function at the bottom (lines 226-255)
```

8. **Update Reconciler initialization (lines 198-206):**
```go
// CHANGE FROM:
if err := (&machinesetcontroller.Reconciler{
	Client:              mgr.GetClient(),
	Log:                 ctrl.Log.WithName("controllers").WithName("MachineSet"),
	AwsClientBuilder:    awsclient.NewValidatedClient,
	RegionCache:         describeRegionsCache,
	ConfigManagedClient: configManagedClient,
	InstanceTypesCache:  machinesetcontroller.NewInstanceTypesCache(),
	Gate:                defaultMutableGate,
}).SetupWithManager(mgr, controller.Options{}); err != nil {
	setupLog.Error(err, "unable to create controller", "controller", "MachineSet")
	os.Exit(1)
}

// CHANGE TO:
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
```

**Verification:**
- File compiles without errors
- No OpenShift imports
- No feature gate code
- CAPI schemes registered

---

### Phase 6: Update Tests (2 hours)

#### Step 6.1: Update pkg/controller/controller_test.go

**File:** `pkg/controller/controller_test.go`

**Major Changes:**

1. **Update imports:**
```go
// REMOVE:
machinev1beta1 "github.com/openshift/api/machine/v1beta1"
openshiftfeatures "github.com/openshift/api/features"
"github.com/openshift/library-go/pkg/features"

// ADD:
clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
```

2. **Remove feature gate setup in BeforeEach:**
```go
// DELETE:
gate, err := newDefaultMutableFeatureGate()
Expect(err).NotTo(HaveOccurred())

// And remove Gate field from Reconciler:
r := Reconciler{
	// ... remove Gate: gate,
}
```

3. **Create helper functions for CAPI test fixtures:**

Add these new helper functions:

```go
// newTestMachineDeployment creates a test CAPI MachineDeployment
func newTestMachineDeployment(namespace, instanceType string, existingAnnotations map[string]string) (*clusterv1.MachineDeployment, *infrav1.AWSMachineTemplate, *clusterv1.Cluster, *infrav1.AWSCluster, error) {
	annotations := make(map[string]string)
	for k, v := range existingAnnotations {
		annotations[k] = v
	}

	// Create AWSMachineTemplate
	awsMachineTemplate := &infrav1.AWSMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-aws-template",
			Namespace: namespace,
		},
		Spec: infrav1.AWSMachineTemplateSpec{
			Template: infrav1.AWSMachineTemplateResource{
				Spec: infrav1.AWSMachineSpec{
					InstanceType: instanceType,
				},
			},
		},
	}

	// Create AWSCluster
	awsCluster := &infrav1.AWSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-aws",
			Namespace: namespace,
		},
		Spec: infrav1.AWSClusterSpec{
			Region: "us-east-1",
		},
	}

	// Create Cluster
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
				Kind:       "AWSCluster",
				Name:       awsCluster.Name,
				Namespace:  awsCluster.Namespace,
			},
		},
	}

	// Create MachineDeployment
	replicas := int32(1)
	machineDeployment := &clusterv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:  annotations,
			GenerateName: "test-md-",
			Namespace:    namespace,
		},
		Spec: clusterv1.MachineDeploymentSpec{
			ClusterName: cluster.Name,
			Replicas:    &replicas,
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					ClusterName: cluster.Name,
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
						Kind:       "AWSMachineTemplate",
						Name:       awsMachineTemplate.Name,
						Namespace:  awsMachineTemplate.Namespace,
					},
				},
			},
		},
	}

	return machineDeployment, awsMachineTemplate, cluster, awsCluster, nil
}
```

4. **Update test cases:**

Replace test table entries to create CAPI resources instead of MAPI:

```go
// In DescribeTable, update test cases to:
Entry("with a a1.2xlarge", reconcileTestCase{
	instanceType:        "a1.2xlarge",
	existingAnnotations: make(map[string]string),
	expectedAnnotations: map[string]string{
		cpuKey:    "8",
		memoryKey: "16384",
		gpuKey:    "0",
		labelsKey: "kubernetes.io/arch=amd64",
	},
	expectedEvents: []string{},
}),
// ... repeat for other test cases
```

5. **Update test execution in DescribeTable callback:**

```go
DescribeTable("when reconciling MachineDeployments", func(rtc reconcileTestCase) {
	machineDeployment, awsMachineTemplate, cluster, awsCluster, err := newTestMachineDeployment(namespace.Name, rtc.instanceType, rtc.existingAnnotations)
	Expect(err).ToNot(HaveOccurred())

	// Create infrastructure resources first
	Expect(c.Create(ctx, awsCluster)).To(Succeed())
	Expect(c.Create(ctx, cluster)).To(Succeed())
	Expect(c.Create(ctx, awsMachineTemplate)).To(Succeed())
	Expect(c.Create(ctx, machineDeployment)).To(Succeed())

	Eventually(func() map[string]string {
		md := &clusterv1.MachineDeployment{}
		key := client.ObjectKey{Namespace: machineDeployment.Namespace, Name: machineDeployment.Name}
		err := c.Get(ctx, key, md)
		if err != nil {
			return nil
		}
		annotations := md.GetAnnotations()
		if annotations != nil {
			return annotations
		}
		return make(map[string]string)
	}, timeout).Should(Equal(rtc.expectedAnnotations))

	// ... rest of test validation
},
```

6. **Remove feature gate test helper:**
```go
// DELETE entire function:
func newDefaultMutableFeatureGate() (featuregate.MutableFeatureGate, error) {
	// ... delete
}
```

7. **Update TestReconcile unit tests:**

Similar pattern - create CAPI fixtures instead of MAPI.

**Verification:**
- All tests compile
- `go test ./pkg/controller -v` passes
- No MAPI references in tests

---

### Phase 7: Clean Up (15 minutes)

#### Step 7.1: Remove MAPI CRD files

```bash
rm -rf config/crd/
```

#### Step 7.2: Update README.md

Update documentation to reflect CAPI usage:

1. Change "OpenShift Machine API" → "Kubernetes Cluster API (CAPI)"
2. Change "MachineSet" → "MachineDeployment"
3. Update deployment examples to show CAPI resources
4. Update RBAC examples (use new ClusterRole from TR5 in requirements.md)
5. Update prerequisites section:
   - Remove "OpenShift cluster" requirement
   - Add "CAPI and CAPA controllers installed" requirement

#### Step 7.3: Verify clean build

```bash
go mod tidy
go build ./...
go test ./... -short
make build
```

**Verification:**
- All commands succeed without errors
- Binary runs: `./bin/capa-annotator --version`
- No OpenShift imports in codebase: `grep -r "github.com/openshift" pkg/ cmd/ --exclude-dir=vendor`

---

### Phase 8: Integration Testing (1 hour)

#### Step 8.1: Setup test environment

Create test CAPI resources for manual testing:

```yaml
# test-fixtures/aws-cluster.yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AWSCluster
metadata:
  name: test-cluster-aws
  namespace: default
spec:
  region: us-east-1

---
# test-fixtures/cluster.yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test-cluster
  namespace: default
spec:
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
    kind: AWSCluster
    name: test-cluster-aws

---
# test-fixtures/aws-machine-template.yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AWSMachineTemplate
metadata:
  name: test-worker-template
  namespace: default
spec:
  template:
    spec:
      instanceType: m5.large

---
# test-fixtures/machine-deployment.yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: test-workers
  namespace: default
spec:
  clusterName: test-cluster
  replicas: 3
  template:
    spec:
      clusterName: test-cluster
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
        kind: AWSMachineTemplate
        name: test-worker-template
```

#### Step 8.2: Run integration tests

```bash
# Set IRSA environment variables for testing
export AWS_ROLE_ARN="arn:aws:iam::123456789012:role/capa-annotator-test"
export AWS_WEB_IDENTITY_TOKEN_FILE="/var/run/secrets/eks.amazonaws.com/serviceaccount/token"

# Run integration tests
make test-integration
```

**Verification:**
- Integration tests pass
- Reconciliation creates annotations on MachineDeployment
- AWS EC2 API calls succeed (with valid IRSA credentials)

---

## Files to Modify/Create

### Modified Files
1. **`go.mod`** - Update dependencies (add CAPI, remove OpenShift)
2. **`pkg/utils/providerspec.go`** - Complete rewrite for CAPI template resolution
3. **`pkg/client/client.go`** - Remove OpenShift code, simplify to IRSA-only
4. **`pkg/controller/controller.go`** - Update to watch MachineDeployment, use CAPI types
5. **`cmd/controller/main.go`** - Update scheme registration, remove feature gates
6. **`pkg/controller/controller_test.go`** - Update all tests for CAPI fixtures
7. **`README.md`** - Update documentation for CAPI

### Unchanged Files
- **`pkg/controller/ec2_instance_types.go`** - No changes (instance type caching logic)
- **`pkg/version/version.go`** - No changes
- **`Makefile`** - No changes

### Deleted Files/Directories
- **`config/crd/`** - All MAPI CRD files

### Created Files
- **`test-fixtures/`** - CAPI test resource YAMLs for integration testing

---

## Dependencies

### External Dependencies Added
- `sigs.k8s.io/cluster-api` v1.8.5 - CAPI core types
- `sigs.k8s.io/cluster-api-provider-aws/v2` v2.6.1 - CAPA infrastructure types

### External Dependencies Removed
- `github.com/openshift/api` - OpenShift API types
- `github.com/openshift/machine-api-operator` - MAPI error types and utilities
- `github.com/openshift/library-go` - Feature gate support

### External Dependencies Unchanged
- `github.com/aws/aws-sdk-go` - AWS SDK for EC2 API
- `sigs.k8s.io/controller-runtime` - Controller framework
- `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go` - Kubernetes core
- `k8s.io/klog/v2` - Logging

### Internal Dependencies
- `pkg/client` - Used by controller (modified to remove OpenShift code)
- `pkg/utils` - Used by controller (rewritten for CAPI)
- `pkg/controller/ec2_instance_types.go` - Used by controller (unchanged)

---

## Testing Strategy

### Unit Tests

**Controller Tests (`pkg/controller/controller_test.go`):**
- Test MachineDeployment reconciliation with various instance types
- Test annotation setting on MachineDeployment
- Test AWSMachineTemplate resolution
  - Template found successfully
  - Template not found (error case)
  - Invalid infrastructureRef
- Test region resolution
  - From AWSCluster (primary path)
  - From annotation (fallback path)
  - Neither available (error case)
- Test IRSA validation
  - Both env vars set (success)
  - Missing AWS_ROLE_ARN (error)
  - Missing AWS_WEB_IDENTITY_TOKEN_FILE (error)
- Test existing annotation preservation
- Test architecture normalization (x86_64, arm64)
- Test GPU instance types

**Client Tests (`pkg/client/client_test.go`):**
- Test IRSA session creation
- Test region cache functionality
- Test AWS client creation with IRSA

**Utils Tests (`pkg/utils/providerspec_test.go` - new file):**
- Test ResolveAWSMachineTemplate
  - Valid template
  - Invalid Kind
  - Template not found
- Test ExtractInstanceType
  - Valid instance type
  - Empty instance type
- Test ResolveRegion
  - AWSCluster path
  - Annotation fallback
  - Neither available

### Integration Tests

**Integration Test Scenarios:**
1. **Full reconciliation flow:**
   - Create Cluster, AWSCluster, AWSMachineTemplate, MachineDeployment
   - Verify controller annotates MachineDeployment
   - Verify annotations contain correct CPU, memory, GPU, arch values

2. **Template not found:**
   - Create MachineDeployment with invalid infrastructureRef
   - Verify warning event emitted
   - Verify reconciliation requeues

3. **Region resolution:**
   - Test with AWSCluster containing region
   - Test with region annotation only
   - Test with neither (verify error)

4. **IRSA authentication:**
   - Test with valid IRSA env vars
   - Test with missing env vars (verify error)

### Manual Testing Scenarios

1. **Deploy to CAPI cluster:**
   - Install controller in CAPI cluster
   - Create MachineDeployment
   - Verify annotations appear
   - Verify cluster-autoscaler can scale from zero

2. **Different instance types:**
   - Test with standard instances (m5.large)
   - Test with ARM instances (m6g.xlarge)
   - Test with GPU instances (p3.2xlarge)
   - Verify correct architecture annotation

3. **Error handling:**
   - Invalid instance type
   - Missing AWSMachineTemplate
   - Missing region
   - Invalid IRSA credentials

4. **Multi-namespace:**
   - Deploy with `--namespace` flag
   - Verify only watches specified namespace
   - Deploy without flag
   - Verify watches all namespaces

---

## Deployment Considerations

### Prerequisites
- Kubernetes cluster with CAPI and CAPA installed
- CAPI version 1.8+ required
- CAPA version 2.6+ required
- IRSA configured for the controller's ServiceAccount
- IAM role with `ec2:DescribeInstanceTypes` and `ec2:DescribeRegions` permissions

### RBAC Updates
Deploy new ClusterRole with CAPI permissions (see TR5 in requirements.md):
```bash
kubectl apply -f config/rbac/capi-clusterrole.yaml
```

### Deployment Configuration
Update Deployment manifest:
- Add IRSA annotations to ServiceAccount
- Set environment variables for IRSA (AWS_ROLE_ARN, AWS_WEB_IDENTITY_TOKEN_FILE)
- Mount service account token with correct audience

### Migration from MAPI Version
**No migration path provided** - this is a clean cutover:
1. Uninstall old MAPI-based controller
2. Install new CAPI-based controller
3. Existing MAPI clusters will NOT be supported
4. Only CAPI clusters are supported going forward

### Configuration Changes
- Remove `--feature-gates` flag if present in deployment
- Verify `--namespace` flag usage (optional)
- Verify metrics and health endpoints (:8080, :9440)

---

## Risks and Mitigation

### Risk 1: Breaking change for existing users
**Impact:** High - Completely incompatible with MAPI clusters

**Mitigation:**
- Clear communication in release notes
- Version bump to v2.0.0 (major version)
- Maintain MAPI version on separate branch if needed
- Document migration path: MAPI → CAPI cluster migration required

### Risk 2: CAPI/CAPA API changes
**Impact:** Medium - CAPI v1beta1 and CAPA v1beta2 APIs could change

**Mitigation:**
- Pin to specific CAPI/CAPA versions
- Monitor CAPI/CAPA release notes for API changes
- Test against multiple CAPI/CAPA versions
- Use stable v1beta1/v1beta2 APIs (less likely to break)

### Risk 3: Region resolution complexity
**Impact:** Medium - Multiple failure points (Cluster, AWSCluster, annotation)

**Mitigation:**
- Clear logging at each resolution step
- Detailed error messages indicating which path failed
- Fallback to annotation for maximum compatibility
- Document recommended region configuration patterns

### Risk 4: IRSA configuration errors
**Impact:** High - Cannot function without proper IRSA setup

**Mitigation:**
- Validate env vars early in reconciliation
- Clear error messages for missing/invalid IRSA config
- Comprehensive IRSA setup documentation
- Example IRSA deployment manifests

### Risk 5: Test coverage gaps
**Impact:** Medium - Complex CAPI resource relationships may have edge cases

**Mitigation:**
- Comprehensive unit test coverage for all resolution paths
- Integration tests with real CAPI resources
- Manual testing against real CAPI clusters
- Test with multiple CAPI/CAPA versions

### Risk 6: Annotation compatibility
**Impact:** Low - Using OpenShift annotation keys in non-OpenShift cluster

**Mitigation:**
- Document that annotation keys remain for cluster-autoscaler compatibility
- Cluster-autoscaler already accepts these annotation keys
- Consider adding CAPI-native annotation keys in future version

---

## Implementation Checklist

### Phase 1: Dependencies ✓
- [ ] Add CAPI dependencies to go.mod
- [ ] Remove OpenShift dependencies from go.mod
- [ ] Run `go mod tidy` and verify

### Phase 2: Template Resolution ✓
- [ ] Rewrite pkg/utils/providerspec.go
- [ ] Add ResolveAWSMachineTemplate function
- [ ] Add ExtractInstanceType function
- [ ] Add ResolveRegion function with fallback
- [ ] Verify file compiles

### Phase 3: AWS Client ✓
- [ ] Remove OpenShift imports from pkg/client/client.go
- [ ] Update AwsClientBuilderFuncType signature
- [ ] Simplify newAWSSession to IRSA-only
- [ ] Remove resolveEndpoints, useCustomCABundle, credential secret functions
- [ ] Verify file compiles

### Phase 4: Controller ✓
- [ ] Update imports in pkg/controller/controller.go
- [ ] Update Reconciler struct (remove OpenShift fields)
- [ ] Update SetupWithManager (watch MachineDeployment)
- [ ] Update Reconcile function (use MachineDeployment type)
- [ ] Remove feature gate block
- [ ] Rewrite reconcile function with CAPI logic
- [ ] Verify file compiles

### Phase 5: Main Entry Point ✓
- [ ] Update imports in cmd/controller/main.go
- [ ] Remove feature gate setup
- [ ] Update scheme registration (CAPI/CAPA)
- [ ] Remove configManagedClient
- [ ] Update Reconciler initialization
- [ ] Verify file compiles

### Phase 6: Tests ✓
- [ ] Update test imports
- [ ] Remove feature gate test setup
- [ ] Create newTestMachineDeployment helper
- [ ] Update all test cases to use CAPI fixtures
- [ ] Remove feature gate test function
- [ ] Run tests and verify they pass

### Phase 7: Cleanup ✓
- [ ] Remove config/crd/ directory
- [ ] Update README.md for CAPI
- [ ] Run `go mod tidy`
- [ ] Verify clean build (`make build`)
- [ ] Verify no OpenShift imports remain

### Phase 8: Integration Testing ✓
- [ ] Create test CAPI fixtures
- [ ] Run integration tests
- [ ] Manual testing with real CAPI cluster
- [ ] Verify annotations set correctly
- [ ] Verify cluster-autoscaler compatibility

---

## Important Notes

**DO NOT include time estimates, resource requirements, or team size recommendations in this implementation plan.**

**Implementation Order:**
Follow the phases sequentially as dependencies exist between them. Each phase should be completed and verified before moving to the next.

**Backwards Compatibility:**
This is a **breaking change**. No MAPI support will remain. Existing MAPI-based deployments must migrate to CAPI clusters to use this version.

**Testing:**
Comprehensive testing is critical due to the architectural changes. Both unit tests and integration tests must pass before considering implementation complete.

**Documentation:**
Update all documentation (README, deployment guides, RBAC examples) to reflect CAPI usage patterns and remove OpenShift references.
