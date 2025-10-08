# Requirements Q&A Session - MachineSet Annotator

## Discovery Questions

### Q1: Will this controller need to handle AWS credentials that are different from those used by the cluster?
**Default if unknown:** No (credentials will come from MachineSet provider spec references, same pattern as machine-api-provider-aws)

### Q2: Should the controller watch MachineSets across all namespaces or be restricted to a specific namespace?
**Default if unknown:** Yes, watch all namespaces (maintains flexibility and matches machine-api-provider-aws pattern)

### Q3: Will the controller need to support custom AWS endpoints (e.g., for GovCloud or custom installations)?
**Default if unknown:** Yes (maintain compatibility with custom endpoint configurations from machine-api-provider-aws)

### Q4: Should the controller continue reconciling MachineSets that are being deleted (have a deletion timestamp)?
**Default if unknown:** No (skip deleted MachineSets as they're being garbage collected)

### Q5: Will the controller need to emit Kubernetes events for important operations (success/failure)?
**Default if unknown:** Yes (helps with observability and debugging)

## Discovery Answers

**Q1:** No - The controller will use AWS credentials from secrets referenced in the MachineSet provider spec, following the same pattern as machine-api-provider-aws.

**Q2:** Yes - The controller should support watching all namespaces by default, with optional namespace restriction via command-line flag.

**Q3:** Yes - The controller needs to support custom AWS endpoints and CA bundles for GovCloud and specialized AWS environments.

**Q4:** No - The controller should skip MachineSets with deletion timestamps as they are being garbage collected.

**Q5:** Yes - The controller should emit Kubernetes events for important operations to aid observability and debugging.

## Context Findings
Research findings from codebase analysis.

### Similar Features Found
- **MachineSet Controller**: `/Users/jjaggars/code/machine-api-provider-aws/pkg/actuators/machineset/controller.go` - Complete implementation of MachineSet reconciliation with annotation logic
- **Instance Types Cache**: `/Users/jjaggars/code/machine-api-provider-aws/pkg/actuators/machineset/ec2_instance_types.go` - Thread-safe cache for EC2 instance type information with 24-hour refresh
- **AWS Client Wrapper**: `/Users/jjaggars/code/machine-api-provider-aws/pkg/client/client.go` - Full AWS client with credential handling, custom endpoints, and CA bundle support

### Implementation Patterns
- **Controller Pattern**: Uses controller-runtime with `Reconcile(ctx, req)` method following standard Kubernetes controller pattern
- **Annotation Keys**: Uses specific keys - `machine.openshift.io/vCPU`, `machine.openshift.io/memoryMb`, `machine.openshift.io/GPU`, `capacity.cluster-autoscaler.kubernetes.io/labels`
- **Feature Gate Handling**: Checks `FeatureGateMachineAPIMigration` and skips reconciliation if `PausedCondition` is true or `AuthoritativeAPI != MachineAPI`
- **Error Handling**: Uses `mapierrors.InvalidMachineConfiguration()` for config errors and returns them without requeuing
- **Provider Spec Parsing**: Uses `ProviderSpecFromRawExtension()` at line 467-480 in utils.go to unmarshal AWSMachineProviderConfig from MachineSet template
- **AWS Session Creation**: `newAWSSession()` at line 357-387 creates sessions with credentials from secrets, custom CA bundles, and custom endpoints
- **Region Validation**: `NewValidatedClient()` validates region before returning client, caching DescribeRegions calls
- **Architecture Normalization**: Converts x86_64→amd64, arm64→arm64 for kubernetes.io/arch labels
- **Event Emission**: Uses `recorder.Eventf()` for "ReconcileError" and "FailedUpdate" events

### Technical Constraints
- **Dependencies**: Requires openshift/api (machinev1beta1, configv1), machine-api-operator, controller-runtime, aws-sdk-go, klog/v2, component-base (for feature gates)
- **Credentials Access**: Must read secrets from same namespace as MachineSet (referenced in providerSpec.CredentialsSecret)
- **ConfigMap Access**: Needs read access to `kube-cloud-config` ConfigMap in `openshift-config-managed` namespace for custom CA bundles
- **Infrastructure Access**: Reads `cluster` Infrastructure object from config.openshift.io/v1 for custom endpoints
- **Cache Behavior**: Instance types cache refreshes after 24 hours, region cache after 30 minutes
- **Deletion Handling**: Must skip MachineSets with non-zero DeletionTimestamp
- **Patch Strategy**: Uses `client.MergeFrom()` to create patch before reconcile, applies with `Client.Patch()` after
- **Concurrent Reconciliation**: Cache is thread-safe with RWMutex to handle concurrent reconciles

## Expert Questions
Detailed technical questions after understanding the codebase.

### Q1: Should the controller support the FeatureGateMachineAPIMigration feature gate and pause reconciliation when enabled?
**Default if unknown:** No (this is a new standalone controller, not part of the migration from machine-api to CAPI, so this feature gate is not applicable)

### Q2: Should instance type information be cached per-region with automatic refresh, or fetched on every reconciliation?
**Default if unknown:** Yes, use caching (following the pattern from machine-api-provider-aws with 24-hour cache refresh to minimize AWS API calls)

### Q3: If an instance type is unknown or not found in AWS, should the controller requeue the MachineSet for retry?
**Default if unknown:** No (following machine-api-provider-aws pattern: log error, emit event, and stop reconciling to prevent infinite retry loops - user intervention required)

### Q4: Should the controller preserve existing values in the `capacity.cluster-autoscaler.kubernetes.io/labels` annotation or replace them?
**Default if unknown:** Yes, preserve and merge (use `util.MergeCommaSeparatedKeyValuePairs` to ensure existing labels are retained alongside architecture label)

### Q5: Should the controller create a separate client cache for the openshift-config-managed namespace to access custom CA bundles?
**Default if unknown:** Yes (maintain compatibility with custom CA bundle support for GovCloud and custom installations)

## Expert Answers

**Q1:** No - The controller does not need to support the FeatureGateMachineAPIMigration feature gate as it's a standalone tool independent of the CAPI migration process.

**Q2:** Yes - Use per-region caching with 24-hour automatic refresh to minimize AWS API calls and avoid throttling.

**Q3:** No - Do not requeue on unknown instance types. Log error, emit warning event, and stop reconciling to prevent infinite retry loops.

**Q4:** Yes - Preserve and merge existing labels using `util.MergeCommaSeparatedKeyValuePairs` to avoid overwriting user-specified capacity labels.

**Q5:** Yes - Create a separate client cache for openshift-config-managed namespace to access custom CA bundles for GovCloud and specialized AWS environments.

## Key Insights

### Architecture Decisions
1. **Standalone Controller**: This is an independent project, not part of machine-api-provider-aws, so it excludes CAPI migration complexity
2. **Faithful Pattern Replication**: Follows machine-api-provider-aws patterns for credentials, caching, error handling, and AWS client setup
3. **Production-Ready Features**: Includes custom CA bundles, custom endpoints, region caching, and thread-safe instance type caching
4. **Smart Error Handling**: No infinite retries on invalid instance types - emit events and stop reconciling

### Critical Implementation Details
1. **Files to Copy**: controller.go, ec2_instance_types.go, client.go, ProviderSpecFromRawExtension from utils.go
2. **Module Path Updates**: Change all internal imports from machine-api-provider-aws to new module path
3. **Package Simplification**: Remove feature gate logic, MachineAPI migration checks, and PausedCondition handling
4. **Simplified Main**: No machine actuator, only MachineSet controller registration
5. **RBAC**: MachineSets (get/list/watch/update/patch), Infrastructures (get/list/watch), Secrets/ConfigMaps (get/list/watch), Events (create/patch)

### Test Coverage
- Instance type cache behavior (thread safety, refresh logic)
- Annotation merging for capacity labels
- Provider spec parsing
- Error handling for unknown instance types
- Deletion timestamp handling
