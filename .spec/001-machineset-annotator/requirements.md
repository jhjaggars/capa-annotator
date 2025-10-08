# Requirements Specification - MachineSet Annotator

## Overview
Create a standalone Kubernetes controller that automatically annotates MachineSet objects with CPU, memory, GPU, and architecture information by querying AWS EC2 instance type data. This enables cluster-autoscaler to scale from zero by providing capacity information before machines are created.

## Problem Statement
The cluster-autoscaler needs to know the capacity (CPU, memory, GPU, architecture) of machines before they're created in order to scale from zero. Currently, this annotation functionality exists within `openshift/machine-api-provider-aws`, but we need it as a standalone, reusable component that can operate independently of the full machine-api-provider-aws stack.

## Solution Overview
Extract the MachineSet annotation functionality from `machine-api-provider-aws` into a new standalone controller. This controller will:
1. Watch MachineSet resources across all namespaces (or a specific namespace)
2. Parse AWS provider configuration from MachineSet specs
3. Query AWS EC2 API for instance type information (with caching)
4. Set annotations on MachineSets with vCPU, memory, GPU, and architecture data
5. Support custom AWS endpoints and CA bundles for specialized environments

## Functional Requirements

### FR1: MachineSet Watching and Reconciliation
- **FR1.1**: Controller must watch MachineSet resources (machine.openshift.io/v1beta1)
- **FR1.2**: Support watching all namespaces by default
- **FR1.3**: Support optional namespace restriction via `--namespace` flag
- **FR1.4**: Skip MachineSets with non-zero deletion timestamps
- **FR1.5**: Use controller-runtime's standard Reconcile pattern

### FR2: AWS Credentials and Authentication
- **FR2.1**: Read AWS credentials from Kubernetes secrets referenced in MachineSet provider spec (`providerSpec.credentialsSecret`)
- **FR2.2**: Credentials secret must be in the same namespace as the MachineSet
- **FR2.3**: Support standard AWS credential keys: `aws_access_key_id` and `aws_secret_access_key`
- **FR2.4**: Return InvalidMachineConfiguration error if credentials secret is nil or not found

### FR3: Instance Type Information Retrieval
- **FR3.1**: Query AWS EC2 `DescribeInstanceTypes` API for instance type details
- **FR3.2**: Extract: vCPU count, memory (MB), GPU count, CPU architecture
- **FR3.3**: Normalize architecture: x86_64 → amd64, arm64 → arm64
- **FR3.4**: Cache instance type information per region with 24-hour refresh
- **FR3.5**: Use thread-safe cache with RWMutex for concurrent reconciliation
- **FR3.6**: Handle AWS API pagination to retrieve all instance types

### FR4: Annotation Setting
- **FR4.1**: Set `machine.openshift.io/vCPU` annotation with vCPU count as string
- **FR4.2**: Set `machine.openshift.io/memoryMb` annotation with memory in MB as string
- **FR4.3**: Set `machine.openshift.io/GPU` annotation with GPU count as string
- **FR4.4**: Set/merge `capacity.cluster-autoscaler.kubernetes.io/labels` annotation with `kubernetes.io/arch=<architecture>`
- **FR4.5**: Preserve existing labels in capacity annotation using `util.MergeCommaSeparatedKeyValuePairs`
- **FR4.6**: Initialize annotations map if nil before setting values

### FR5: Error Handling
- **FR5.1**: For unknown instance types: log error, emit warning event, return without error (no requeue)
- **FR5.2**: For invalid provider config: return InvalidMachineConfiguration error
- **FR5.3**: For AWS client errors: return error to trigger requeue
- **FR5.4**: Emit Kubernetes events for "ReconcileError" and "FailedUpdate"
- **FR5.5**: Use structured logging with MachineSet name and namespace

### FR6: Custom AWS Configuration
- **FR6.1**: Support custom CA bundles from `kube-cloud-config` ConfigMap in `openshift-config-managed` namespace
- **FR6.2**: Support custom AWS endpoints from Infrastructure object (config.openshift.io/v1)
- **FR6.3**: Create separate client cache scoped to `openshift-config-managed` namespace
- **FR6.4**: Validate AWS region before creating client (using region cache)
- **FR6.5**: Cache `DescribeRegions` API calls for 30 minutes

### FR7: Controller Configuration
- **FR7.1**: Support `--metrics-bind-address` flag (default: `:8080`)
- **FR7.2**: Support `--namespace` flag for namespace restriction
- **FR7.3**: Support `--leader-elect` flag for HA deployments
- **FR7.4**: Support `--leader-elect-lease-duration` flag (default: 120s)
- **FR7.5**: Support `--health-addr` flag (default: `:9440`)
- **FR7.6**: Provide health and readiness endpoints

## Technical Requirements

### TR1: Dependencies
- **TR1.1**: `github.com/aws/aws-sdk-go` for AWS API interaction
- **TR1.2**: `github.com/openshift/api` for machine/v1beta1 and config/v1 types
- **TR1.3**: `github.com/openshift/machine-api-operator` for utilities and error types
- **TR1.4**: `sigs.k8s.io/controller-runtime` for controller framework
- **TR1.5**: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go` for Kubernetes types
- **TR1.6**: `k8s.io/klog/v2` for logging
- **TR1.7**: Go module: `github.com/jjaggars/capi-annotator` (or appropriate name)

### TR2: Source Files to Port
- **TR2.1**: `/Users/jjaggars/code/machine-api-provider-aws/pkg/actuators/machineset/controller.go` → `pkg/controller/controller.go`
- **TR2.2**: `/Users/jjaggars/code/machine-api-provider-aws/pkg/actuators/machineset/ec2_instance_types.go` → `pkg/controller/ec2_instance_types.go`
- **TR2.3**: `/Users/jjaggars/code/machine-api-provider-aws/pkg/client/client.go` → `pkg/client/client.go`
- **TR2.4**: `ProviderSpecFromRawExtension` function from `/Users/jjaggars/code/machine-api-provider-aws/pkg/actuators/machine/utils.go` → `pkg/utils/providerspec.go`

### TR3: Code Modifications
- **TR3.1**: Update all internal package imports from `github.com/openshift/machine-api-provider-aws/pkg/*` to new module path
- **TR3.2**: Remove FeatureGateMachineAPIMigration gate checks from controller.go (lines 90-109)
- **TR3.3**: Remove PausedCondition and AuthoritativeAPI logic
- **TR3.4**: Keep all AWS client, region cache, and instance type cache logic intact
- **TR3.5**: Simplify main.go to only register MachineSet controller (no machine actuator)

### TR4: RBAC Permissions
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
rules:
- apiGroups: ["machine.openshift.io"]
  resources: ["machinesets"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["config.openshift.io"]
  resources: ["infrastructures"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["secrets", "configmaps"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

### TR5: Deployment Configuration
- **TR5.1**: Deploy as Kubernetes Deployment with 1-3 replicas
- **TR5.2**: Enable leader election for HA
- **TR5.3**: Set lease duration to 120s, renew deadline to 110s, retry period to 20s
- **TR5.4**: Requires network access to AWS EC2 API endpoints
- **TR5.5**: Typically deployed in `openshift-machine-api` namespace

## Implementation Hints

### Files Requiring Direct Port
1. **pkg/controller/controller.go** (from pkg/actuators/machineset/controller.go):
   - Lines 1-66: Keep struct, interface, setup method
   - Lines 68-88: Keep reconcile entry point and deletion check
   - Lines 90-109: **REMOVE** feature gate and paused condition logic
   - Lines 111-182: Keep provider spec parsing, AWS client creation, instance type query, annotation setting

2. **pkg/controller/ec2_instance_types.go** (from pkg/actuators/machineset/ec2_instance_types.go):
   - Keep entire file unchanged except package name

3. **pkg/client/client.go** (from pkg/client/client.go):
   - Keep entire file unchanged except package imports

4. **pkg/utils/providerspec.go**:
   - Extract only `ProviderSpecFromRawExtension` function (lines 467-480)

### Main Entry Point Pattern
Based on `/Users/jjaggars/code/machine-api-provider-aws/cmd/manager/main.go` lines 150-234:
- Parse flags
- Create controller-runtime Manager with metrics, leader election, health endpoints
- Register schemes: machinev1beta1, configv1, corev1
- Create config-managed client for openshift-config-managed namespace
- Initialize region cache and instance types cache
- Register only MachineSet controller (skip machine actuator)
- Start manager with signal handler

### Provider Spec Parsing
From controller.go line 144:
```go
providerConfig, err := utils.ProviderSpecFromRawExtension(machineSet.Spec.Template.Spec.ProviderSpec.Value)
```

### Instance Type Query
From controller.go line 158:
```go
instanceType, err := r.InstanceTypesCache.GetInstanceType(awsClient, providerConfig.Placement.Region, providerConfig.InstanceType)
```

### Annotation Merging
From controller.go line 178-180:
```go
machineSet.Annotations[labelsKey] = util.MergeCommaSeparatedKeyValuePairs(
    fmt.Sprintf("kubernetes.io/arch=%s", instanceType.CPUArchitecture),
    machineSet.Annotations[labelsKey])
```

## Acceptance Criteria

- [ ] Controller successfully watches MachineSets in all namespaces
- [ ] Controller can be restricted to a single namespace with --namespace flag
- [ ] MachineSets with deletion timestamps are skipped
- [ ] AWS credentials are read from secrets referenced in provider spec
- [ ] Instance type information is cached per region with 24-hour refresh
- [ ] Cache is thread-safe for concurrent reconciliation
- [ ] vCPU, memory, GPU annotations are set correctly
- [ ] Architecture label is merged with existing capacity annotations
- [ ] Unknown instance types log error and emit event without requeuing
- [ ] Custom CA bundles from openshift-config-managed namespace are supported
- [ ] Custom AWS endpoints from Infrastructure object are supported
- [ ] Controller emits Kubernetes events for errors
- [ ] Health and readiness endpoints respond correctly
- [ ] Leader election works for HA deployments
- [ ] Unit tests cover cache behavior, annotation merging, error handling

## Assumptions

1. MachineSets will use AWS provider spec format (machine.openshift.io/v1beta1.AWSMachineProviderConfig)
2. AWS credentials in secrets use standard keys: `aws_access_key_id`, `aws_secret_access_key`
3. Controller runs in OpenShift environment with access to config.openshift.io/v1 Infrastructure CRD
4. The `openshift-config-managed` namespace exists for custom CA bundles
5. Network connectivity to AWS EC2 API endpoints is available
6. This controller does not participate in CAPI migration (no FeatureGateMachineAPIMigration support)
7. Label merging utility (`util.MergeCommaSeparatedKeyValuePairs`) is available from machine-api-operator

## Out of Scope

- **Machine (not MachineSet) reconciliation**: This controller only handles MachineSets
- **Other cloud providers**: Only AWS is supported (Azure, GCP, etc. are out of scope)
- **Machine provisioning**: This controller only sets annotations, does not create/delete machines
- **CAPI migration support**: No FeatureGateMachineAPIMigration feature gate handling
- **Machine status updates**: Only MachineSet annotations are modified
- **Webhook validation**: No admission webhooks for validating MachineSet configurations
- **Custom metrics**: Only standard controller-runtime metrics (no custom Prometheus metrics for cache hits/misses)
