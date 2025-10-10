# Requirements Specification - CAPI MachineDeployment Support

## Overview
Migrate the capa-annotator controller from OpenShift Machine API (MAPI) MachineSet resources to Kubernetes Cluster API (CAPI) MachineDeployment resources. The controller will continue to automatically annotate machine resources with CPU, memory, GPU, and architecture information by querying AWS EC2 instance type data, enabling cluster-autoscaler to scale from zero.

## Problem Statement
The current capa-annotator only supports OpenShift Machine API (MAPI) resources, which limits its use to OpenShift clusters. To support standard Kubernetes Cluster API (CAPI) deployments, the controller must be migrated to watch and annotate CAPI MachineDeployment resources instead. This requires:
- Changing the watched resource type from MAPI MachineSet to CAPI MachineDeployment
- Resolving external infrastructure templates instead of parsing embedded provider specs
- Removing OpenShift-specific dependencies
- Adapting to CAPI's different resource structure and patterns

## Solution Overview
Replace MAPI support with CAPI support by:
1. Watching `cluster.x-k8s.io/v1beta1` MachineDeployment resources
2. Resolving `infrastructure.cluster.x-k8s.io/v1beta2` AWSMachineTemplate from infrastructureRef
3. Extracting instance type from the external template instead of embedded provider spec
4. Fetching AWS region from AWSCluster resource with annotation fallback
5. Maintaining IRSA authentication and existing annotation keys for compatibility
6. Removing all OpenShift-specific dependencies (config/v1, machine-api-operator)
7. Keeping AWS client, instance type caching, and annotation logic unchanged

## Functional Requirements

### FR1: CAPI MachineDeployment Watching
- **FR1.1**: Controller must watch MachineDeployment resources (`cluster.x-k8s.io/v1beta1`)
- **FR1.2**: Support watching all namespaces by default (current behavior)
- **FR1.3**: Support optional namespace restriction via existing `--namespace` flag
- **FR1.4**: Skip MachineDeployments with non-zero deletion timestamps
- **FR1.5**: Use controller-runtime's standard Reconcile pattern

### FR2: Infrastructure Template Resolution
- **FR2.1**: Extract `infrastructureRef` from MachineDeployment.spec.template.spec.infrastructureRef
- **FR2.2**: Fetch referenced AWSMachineTemplate resource using client.Get()
- **FR2.3**: Validate infrastructureRef points to `infrastructure.cluster.x-k8s.io/v1beta2` AWSMachineTemplate
- **FR2.4**: Extract instance type from AWSMachineTemplate.spec.template.spec.instanceType
- **FR2.5**: If AWSMachineTemplate not found: emit warning event, log error, requeue reconciliation
- **FR2.6**: If infrastructureRef is nil or invalid: emit warning event, skip reconciliation

### FR3: AWS Region Resolution
- **FR3.1**: Primary: Fetch MachineDeployment.spec.clusterName → Get Cluster resource → Extract cluster.spec.infrastructureRef → Fetch AWSCluster → Read spec.region
- **FR3.2**: Fallback: If AWSCluster fetch fails, look for region annotation on MachineDeployment (e.g., `capa.infrastructure.cluster.x-k8s.io/region`)
- **FR3.3**: If region cannot be determined: emit warning event, skip reconciliation
- **FR3.4**: Cache region information per MachineDeployment to avoid repeated lookups

### FR4: AWS Authentication (Unchanged)
- **FR4.1**: Use IRSA (IAM Roles for Service Accounts) as primary authentication method
- **FR4.2**: Require both environment variables: `AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE`
- **FR4.3**: Return error if IRSA environment variables are not configured
- **FR4.4**: AWS SDK automatically handles web identity token authentication

### FR5: Instance Type Information Retrieval (Unchanged)
- **FR5.1**: Query AWS EC2 `DescribeInstanceTypes` API for instance type details
- **FR5.2**: Extract: vCPU count, memory (MB), GPU count, CPU architecture
- **FR5.3**: Normalize architecture: x86_64 → amd64, arm64 → arm64
- **FR5.4**: Cache instance type information per region with 24-hour refresh
- **FR5.5**: Use thread-safe cache with RWMutex for concurrent reconciliation
- **FR5.6**: Handle AWS API pagination to retrieve all instance types

### FR6: Annotation Setting (Unchanged Keys)
- **FR6.1**: Set `machine.openshift.io/vCPU` annotation with vCPU count as string
- **FR6.2**: Set `machine.openshift.io/memoryMb` annotation with memory in MB as string
- **FR6.3**: Set `machine.openshift.io/GPU` annotation with GPU count as string
- **FR6.4**: Set/merge `capacity.cluster-autoscaler.kubernetes.io/labels` annotation with `kubernetes.io/arch=<architecture>`
- **FR6.5**: Preserve existing labels in capacity annotation (no merging utility needed - use manual string concatenation)
- **FR6.6**: Initialize annotations map if nil before setting values

### FR7: Error Handling
- **FR7.1**: For unknown instance types: log error, emit warning event, return without error (no requeue)
- **FR7.2**: For missing AWSMachineTemplate: emit warning event, return error to requeue
- **FR7.3**: For AWS client errors: return error to trigger requeue
- **FR7.4**: Emit Kubernetes events for "ReconcileError" and "FailedUpdate"
- **FR7.5**: Use structured logging with MachineDeployment name and namespace

### FR8: Controller Configuration (Simplified)
- **FR8.1**: Support `--metrics-bind-address` flag (default: `:8080`)
- **FR8.2**: Support `--namespace` flag for namespace restriction
- **FR8.3**: Support `--leader-elect` flag for HA deployments
- **FR8.4**: Support `--leader-elect-lease-duration` flag (default: 120s)
- **FR8.5**: Support `--health-addr` flag (default: `:9440`)
- **FR8.6**: Remove `--feature-gates` flag (no longer needed)

## Technical Requirements

### TR1: Dependencies
**Add:**
- **TR1.1**: `sigs.k8s.io/cluster-api` v1.8+ for cluster.x-k8s.io/v1beta1 types (Cluster, MachineDeployment)
- **TR1.2**: `sigs.k8s.io/cluster-api-provider-aws/v2` v2.6+ for infrastructure.cluster.x-k8s.io/v1beta2 types (AWSCluster, AWSMachineTemplate)

**Remove:**
- **TR1.3**: `github.com/openshift/api` - No longer needed (was for machinev1beta1, configv1)
- **TR1.4**: `github.com/openshift/machine-api-operator` - No longer needed (was for error types and utilities)
- **TR1.5**: `github.com/openshift/library-go` - No longer needed (was for feature gates)

**Keep:**
- **TR1.6**: `github.com/aws/aws-sdk-go` for AWS API interaction
- **TR1.7**: `sigs.k8s.io/controller-runtime` for controller framework
- **TR1.8**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go` for Kubernetes types
- **TR1.9**: `k8s.io/klog/v2` for logging

### TR2: Files to Modify

**`go.mod`:**
- Add CAPI and CAPA dependencies as specified in TR1
- Remove OpenShift dependencies as specified in TR1

**`cmd/controller/main.go`:**
- **Line 23-26**: Remove imports for `configv1`, `machinev1`, `machinev1beta1`, `apifeatures`
- **Line 24-25**: Add imports for CAPI: `clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"`
- **Line 24-25**: Add imports for CAPA: `infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"`
- **Line 98-106**: Remove feature gate setup (openshiftfeatures, FeatureGateMachineAPIMigration)
- **Line 169-179**: Replace scheme registration:
  - Remove: `machinev1beta1.AddToScheme()`, `machinev1.Install()`, `configv1.AddToScheme()`
  - Add: `clusterv1.AddToScheme(mgr.GetScheme())`, `infrav1.AddToScheme(mgr.GetScheme())`
- **Line 185-191**: Remove configManagedClient creation (no longer needed for OpenShift ConfigMap access)
- **Line 198-206**: Update Reconciler initialization - remove ConfigManagedClient and Gate fields

**`pkg/controller/controller.go`:**
- **Line 13**: Replace import `machinev1beta1 "github.com/openshift/api/machine/v1beta1"` with `clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"`
- **Line 14**: Add import `infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"`
- **Line 14**: Remove imports for `mapierrors`, `util`, `conditions`, `openshiftfeatures`
- **Line 32-35**: Keep annotation key constants (no changes)
- **Line 44-46**: Remove fields from Reconciler struct: `ConfigManagedClient`, `Gate`
- **Line 55**: Change watched resource from `&machinev1beta1.MachineSet{}` to `&clusterv1.MachineDeployment{}`
- **Line 73**: Change resource type from `machineSet := &machinev1beta1.MachineSet{}` to `machineDeployment := &clusterv1.MachineDeployment{}`
- **Line 90-109**: Remove entire feature gate block (FeatureGateMachineAPIMigration, PausedCondition, AuthoritativeAPI checks)
- **Line 142**: Update signature: `func (r *Reconciler) reconcile(machineDeployment *clusterv1.MachineDeployment) (ctrl.Result, error)`
- **Line 144-147**: Replace provider config extraction with infrastructure template resolution (see TR3)
- **Line 149-165**: Replace credentials checking logic with IRSA-only validation (see TR4)
- **Line 167-170**: Update AWS client creation to remove configManagedClient parameter

**`pkg/utils/providerspec.go`:**
- Replace entire file with new CAPI template resolution functions (see TR3)

**`pkg/client/client.go`:**
- **Line 44**: Remove import for `configv1`
- **Line 45**: Remove import for `machineapiapierrors`
- **Line 57-68**: Remove constants: `GlobalInfrastuctureName`, `KubeCloudConfigNamespace`, `kubeCloudConfigName`, `cloudCABundleKey`
- **Line 71**: Remove `configManagedClient client.Client` parameter from AwsClientBuilderFuncType
- **Line 193**: Remove `configManagedClient client.Client` parameter from NewClient function
- **Line 316**: Remove `configManagedClient client.Client` parameter from NewValidatedClient function
- **Line 356**: Remove `configManagedClient client.Client` parameter from newAWSSession function
- **Line 370-377**: Replace `machineapiapierrors.InvalidMachineConfiguration` with standard `fmt.Errorf`
- **Line 383-404**: Remove entire secret-based authentication block (lines checking secretName)
- **Line 401-404**: Update error message for missing IRSA to just return standard error
- **Line 406-409**: Remove `resolveEndpoints()` call (OpenShift-specific custom endpoints)
- **Line 411-414**: Remove `useCustomCABundle()` call (OpenShift-specific CA bundle)
- **Line 440-473**: Remove `resolveEndpoints()` function entirely
- **Line 475-484**: Remove `buildCustomEndpointsMap()` function entirely
- **Line 486-519**: Remove `sharedCredentialsFileFromSecret()` and `newConfigForStaticCreds()` functions entirely
- **Line 521-544**: Remove `useCustomCABundle()` function entirely

**`pkg/controller/controller_test.go`:**
- Update all test cases to use CAPI MachineDeployment and AWSMachineTemplate instead of MAPI MachineSet
- Replace `newTestMachineSet()` with `newTestMachineDeployment()`
- Remove feature gate test setup
- Update imports

### TR3: Infrastructure Template Resolution Logic
New function in `pkg/utils/providerspec.go`:
```go
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

    return template, nil
}

// ExtractInstanceType gets the instance type from AWSMachineTemplate
func ExtractInstanceType(template *infrav1.AWSMachineTemplate) (string, error) {
    if template.Spec.Template.Spec.InstanceType == "" {
        return "", fmt.Errorf("instanceType is empty in AWSMachineTemplate")
    }
    return template.Spec.Template.Spec.InstanceType, nil
}
```

### TR4: Region Resolution Logic
New function in `pkg/utils/providerspec.go`:
```go
const RegionAnnotation = "capa.infrastructure.cluster.x-k8s.io/region"

// ResolveRegion attempts to get AWS region from AWSCluster, falls back to annotation
func ResolveRegion(ctx context.Context, c client.Client, machineDeployment *clusterv1.MachineDeployment) (string, error) {
    // Try to get region from AWSCluster
    if machineDeployment.Spec.ClusterName != "" {
        // Fetch the Cluster resource
        cluster := &clusterv1.Cluster{}
        clusterKey := client.ObjectKey{
            Name:      machineDeployment.Spec.ClusterName,
            Namespace: machineDeployment.Namespace,
        }

        if err := c.Get(ctx, clusterKey, cluster); err == nil {
            // Try to fetch AWSCluster
            if cluster.Spec.InfrastructureRef != nil {
                awsCluster := &infrav1.AWSCluster{}
                awsClusterKey := client.ObjectKey{
                    Name:      cluster.Spec.InfrastructureRef.Name,
                    Namespace: cluster.Spec.InfrastructureRef.Namespace,
                }
                if awsClusterKey.Namespace == "" {
                    awsClusterKey.Namespace = cluster.Namespace
                }

                if err := c.Get(ctx, awsClusterKey, awsCluster); err == nil {
                    if awsCluster.Spec.Region != "" {
                        klog.V(3).Infof("Resolved region %s from AWSCluster %s", awsCluster.Spec.Region, awsClusterKey.Name)
                        return awsCluster.Spec.Region, nil
                    }
                }
            }
        }
    }

    // Fallback to annotation
    if region, ok := machineDeployment.Annotations[RegionAnnotation]; ok && region != "" {
        klog.V(3).Infof("Using region %s from annotation", region)
        return region, nil
    }

    return "", fmt.Errorf("unable to determine AWS region from AWSCluster or annotation")
}
```

### TR5: RBAC Requirements
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: capa-annotator
rules:
# CAPI MachineDeployment access
- apiGroups: ["cluster.x-k8s.io"]
  resources: ["machinedeployments"]
  verbs: ["get", "list", "watch", "update", "patch"]
# CAPI Cluster access (for region resolution)
- apiGroups: ["cluster.x-k8s.io"]
  resources: ["clusters"]
  verbs: ["get", "list", "watch"]
# CAPA infrastructure template access
- apiGroups: ["infrastructure.cluster.x-k8s.io"]
  resources: ["awsmachinetemplates", "awsclusters"]
  verbs: ["get", "list", "watch"]
# Events
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
# Leader election
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
```

### TR6: Files to Remove
- **TR6.1**: `config/crd/` - All MAPI CRD files (will use CAPI CRDs installed by cluster-api)
- **TR6.2**: Any OpenShift-specific RBAC or deployment manifests if they exist

## Implementation Hints

### Files Requiring Changes (Priority Order)
1. **`go.mod`** - Update dependencies first
2. **`pkg/utils/providerspec.go`** - Add CAPI template resolution functions
3. **`pkg/client/client.go`** - Remove OpenShift dependencies and secret-based auth
4. **`pkg/controller/controller.go`** - Update to watch MachineDeployment and use new resolution logic
5. **`cmd/controller/main.go`** - Update scheme registration and remove feature gates
6. **`pkg/controller/controller_test.go`** - Update test fixtures and cases
7. **`README.md`** - Update documentation for CAPI usage

### Patterns to Follow
- **Template resolution pattern**: Similar to how CAPI controllers resolve infrastructure templates (see cluster-api codebase)
- **Region caching**: Add simple in-memory cache map[string]string for MachineDeployment UID → region
- **Error handling**: Maintain existing pattern from line 178 (emit event, log, return nil for terminal errors)
- **AWS client reuse**: Keep all existing AWS client, caching, and EC2 API interaction logic

### Key Implementation Notes
1. **No credential secret handling**: With IRSA-only, remove all Secret fetching logic from client.go
2. **No custom endpoints**: Remove Infrastructure resource fetching and endpoint resolution
3. **No CA bundle**: Remove ConfigMap-based CA bundle support
4. **Simplified error types**: Replace `mapierrors.InvalidMachineConfiguration` with standard `fmt.Errorf`
5. **Annotation merging**: Since we removed OpenShift utilities, use simple string concatenation for labels annotation

### Testing Considerations
- Create test CAPI MachineDeployment and AWSMachineTemplate fixtures
- Mock region resolution (both AWSCluster and annotation paths)
- Test template not found scenario
- Test IRSA environment variable validation
- Integration tests will require CAPI CRDs (can use envtest with CRD installation)

## Acceptance Criteria
- [ ] Controller successfully watches CAPI MachineDeployment resources across all namespaces
- [ ] Controller correctly resolves AWSMachineTemplate from infrastructureRef
- [ ] Controller extracts instance type from AWSMachineTemplate.spec.template.spec.instanceType
- [ ] Controller resolves AWS region from AWSCluster.spec.region (primary method)
- [ ] Controller falls back to region annotation when AWSCluster is not accessible
- [ ] Controller uses IRSA for AWS authentication (environment variables)
- [ ] Controller queries EC2 DescribeInstanceTypes API and caches results for 24 hours
- [ ] Controller sets all 4 annotation keys on MachineDeployment with correct values
- [ ] Controller handles missing AWSMachineTemplate by emitting warning event and requeuing
- [ ] Controller handles invalid instance types by emitting warning event without requeuing
- [ ] All OpenShift dependencies removed from go.mod
- [ ] All tests updated to use CAPI fixtures and pass successfully
- [ ] README and documentation updated to reflect CAPI usage
- [ ] Binary name remains "capa-annotator" (no breaking changes)
- [ ] Health and metrics endpoints continue to function
- [ ] Leader election works correctly in HA deployments

## Assumptions
1. **CAPI installed**: Target Kubernetes cluster has Cluster API and Cluster API Provider AWS (CAPA) controllers already installed
2. **CAPI version**: Using stable CAPI v1beta1 API and CAPA v1beta2 API (minimum versions: cluster-api v1.8+, cluster-api-provider-aws v2.6+)
3. **IRSA configured**: Deployment environment has IRSA properly configured with AWS_ROLE_ARN and AWS_WEB_IDENTITY_TOKEN_FILE
4. **IAM permissions**: IRSA role has ec2:DescribeInstanceTypes and ec2:DescribeRegions permissions
5. **MachineDeployment usage**: CAPI clusters use MachineDeployment resources (not raw MachineSet)
6. **AWSMachineTemplate naming**: Infrastructure templates follow standard CAPI naming conventions
7. **Cluster reference**: MachineDeployment.spec.clusterName is set and references a valid Cluster resource
8. **Same namespace**: AWSMachineTemplate, Cluster, and AWSCluster resources are in the same namespace as MachineDeployment (standard CAPI pattern)
9. **Annotation compatibility**: Cluster-autoscaler continues to recognize existing annotation keys (machine.openshift.io/*)
10. **No migration path needed**: Clean cutover from MAPI to CAPI, no migration of existing MAPI clusters

## Out of Scope
1. **Dual MAPI/CAPI support**: No backward compatibility with MAPI resources
2. **Migration tooling**: No tools to migrate from MAPI to CAPI annotations
3. **Secret-based authentication**: Only IRSA supported, no AWS credential Secrets
4. **Custom AWS endpoints**: No support for custom EC2 endpoints (GovCloud, China regions, etc.)
5. **Custom CA bundles**: No support for custom certificate authorities
6. **OpenShift compatibility**: No OpenShift-specific features or integrations
7. **MachineSet watching**: Only MachineDeployment resources supported, not lower-level MachineSet
8. **Multi-cloud support**: AWS-only, no Azure, GCP, or other cloud providers
9. **Non-AWS CAPI**: No support for CAPI clusters on other infrastructure providers
10. **Annotation customization**: Annotation keys are fixed for cluster-autoscaler compatibility
11. **Historic MAPI data**: No preservation or migration of existing MAPI MachineSet annotations
