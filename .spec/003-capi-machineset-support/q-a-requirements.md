# Requirements Q&A Session

## Discovery Questions
Questions to understand the problem space and context.

### Q1: Should this migration completely replace MAPI support with CAPI-only support?
**Default if unknown:** No (maintain backward compatibility to avoid breaking existing deployments)

### Q2: Will CAPI MachineSet resources use the same annotation keys as MAPI for cluster-autoscaler compatibility?
**Default if unknown:** Yes (cluster-autoscaler expects standard annotation keys)

### Q3: Should the controller watch both MachineSet and MachineDeployment CAPI resources?
**Default if unknown:** Yes (MachineDeployment is more commonly used in CAPI clusters)

### Q4: Will CAPI resources reference AWS credentials using the same Secret-based approach as MAPI?
**Default if unknown:** No (CAPI typically uses AWSCluster for cluster-wide credentials or separate credential references)

### Q5: Should IRSA (IAM Roles for Service Accounts) authentication continue to work with CAPI resources?
**Default if unknown:** Yes (IRSA is best practice for AWS authentication in Kubernetes)

## Discovery Answers
Consolidated answers from the discovery phase.

**A1:** Yes - Complete migration to CAPI-only support, removing all MAPI code

**A2:** Yes - Keep the same annotation keys for cluster-autoscaler compatibility:
- `machine.openshift.io/vCPU`
- `machine.openshift.io/memoryMb`
- `machine.openshift.io/GPU`
- `capacity.cluster-autoscaler.kubernetes.io/labels`

**A3:** Watch only MachineDeployment resources (cluster.x-k8s.io/v1beta1), not MachineSet

**A4:** Continue using IRSA primarily (no need to extract credentials from AWSCluster or other CAPI patterns)

**A5:** Yes - IRSA remains the primary authentication method using AWS_ROLE_ARN and AWS_WEB_IDENTITY_TOKEN_FILE environment variables

## Context Findings
Research findings from codebase analysis.

### Current MAPI Architecture
**Controller Pattern (`pkg/controller/controller.go`)**:
- Watches `machinev1beta1.MachineSet` resources
- Extracts `AWSMachineProviderConfig` from embedded `providerSpec.value`
- Instance type is directly in the providerSpec: `providerConfig.InstanceType`
- Region is from: `providerConfig.Placement.Region`
- Credentials reference: `providerConfig.CredentialsSecret.Name`

**Key Components**:
1. **Controller reconciliation** (lines 68-130): Standard controller-runtime pattern with MachineSet watching
2. **Provider spec parsing** (`pkg/utils/providerspec.go`): `ProviderSpecFromRawExtension()` unmarshals MAPI provider config
3. **AWS client creation** (`pkg/client/client.go`): Handles IRSA and secret-based auth
4. **Instance type caching** (`pkg/controller/ec2_instance_types.go`): 24-hour cache with thread-safe access
5. **Annotation setting** (lines 182-194 in controller.go): Sets 4 annotation keys

### CAPI Architecture Differences
**Resource Structure**:
```
MAPI: MachineSet.spec.template.spec.providerSpec.value (embedded AWSMachineProviderConfig)
CAPI: MachineDeployment.spec.template.spec.infrastructureRef → AWSMachineTemplate.spec.template.spec
```

**Key CAPI Resources**:
- `cluster.x-k8s.io/v1beta1` MachineDeployment (what we'll watch)
- `infrastructure.cluster.x-k8s.io/v1beta2` AWSMachineTemplate (referenced infrastructure)
- Instance type location: `AWSMachineTemplate.spec.template.spec.instanceType`
- Region location: Typically in AWSCluster or can be inferred from cluster

### Files Requiring Changes

**Must Modify**:
1. **`go.mod`**: Add CAPI dependencies
   - `sigs.k8s.io/cluster-api` for MachineDeployment
   - `sigs.k8s.io/cluster-api-provider-aws/v2` for AWSMachineTemplate

2. **`cmd/controller/main.go`** (lines 169-183):
   - Remove: `machinev1beta1.AddToScheme()` and `machinev1.Install()`
   - Add: CAPI scheme registration (clusterv1, infrav1)
   - Remove: Feature gate setup for MachineAPIMigration

3. **`pkg/controller/controller.go`**:
   - Change watched resource from `machinev1beta1.MachineSet` to `clusterv1.MachineDeployment`
   - Remove: Feature gate logic (lines 90-109)
   - Update: `reconcile()` signature to accept MachineDeployment
   - Add: Infrastructure template resolution logic
   - Keep: Annotation keys (no change needed)
   - Keep: AWS client builder, instance type cache, IRSA auth

4. **`pkg/utils/providerspec.go`**:
   - Replace/add function to resolve AWSMachineTemplate from infrastructureRef
   - New function to extract instance type from AWSMachineTemplate

**Can Remove**:
- `config/crd/` - All MAPI CRDs (will use CAPI CRDs from cluster-api installation)
- Feature gate references throughout

**Keep Unchanged**:
- `pkg/client/client.go` - AWS client and IRSA auth logic works as-is
- `pkg/controller/ec2_instance_types.go` - Instance type caching unchanged
- Annotation key constants - Same keys for compatibility

### Technical Constraints
1. **CAPI installation prerequisite**: Cluster must have CAPI and CAPA controllers installed
2. **AWSMachineTemplate reference resolution**: Need to fetch external resource, not embedded config
3. **Region discovery**: May need to infer region from cluster context or AWSCluster resource
4. **API version compatibility**: CAPI v1beta1 (stable), CAPA v1beta2 (current AWS provider version)

## Expert Questions
Detailed technical questions after understanding the codebase.

### Q1: Should the controller fetch the AWS region from the referenced AWSCluster resource, or require it as an annotation on the MachineDeployment?
**Default if unknown:** Fetch from AWSCluster (follows CAPI patterns where cluster-wide config lives in Cluster resources)

### Q2: If an AWSMachineTemplate resource is not found or cannot be fetched, should we skip reconciliation silently or emit a warning event?
**Default if unknown:** Emit warning event and requeue (same pattern as invalid instance type in current code at line 178)

### Q3: Should we remove all OpenShift-specific dependencies (configv1.Infrastructure for custom endpoints, machine-api-operator error types)?
**Default if unknown:** Yes (clean break from OpenShift, use standard CAPI patterns)

### Q4: Should the controller continue to watch all namespaces by default, or only watch the namespace where CAPI resources typically live?
**Default if unknown:** Watch all namespaces (maintains current behavior, more flexible for multi-tenant clusters)

### Q5: Should we rename the project/binary from "capa-annotator" to something more CAPI-specific, or keep the existing name?
**Default if unknown:** Keep existing name (maintains continuity, "CAPA" already refers to Cluster API Provider AWS)

## Expert Answers
Consolidated answers from the expert phase.

**A1:** Option C - Try fetching region from AWSCluster resource first, fall back to annotation if AWSCluster not found or region not specified

**A2:** Default - Emit warning event and requeue when AWSMachineTemplate is not found (matches existing error handling pattern at controller.go:178)

**A3:** Yes - Remove all OpenShift-specific dependencies:
- Remove `github.com/openshift/api/config/v1` (Infrastructure, custom endpoints)
- Remove `github.com/openshift/machine-api-operator` (error types, utilities)
- Remove custom CA bundle support from OpenShift ConfigMaps

**A4:** Watch all namespaces by default, keep existing `--namespace` flag for optional override (maintains current flexibility)

**A5:** Keep existing "capa-annotator" name (CAPA already means Cluster API Provider AWS, no breaking changes needed)

## Key Insights
Summary of important insights gathered during the Q&A process.

1. **Clean Migration Strategy**: Complete replacement of MAPI with CAPI, no dual-mode support needed
2. **Architectural Shift**: Move from embedded provider config to external infrastructure template references
3. **Resource Type Focus**: Watch MachineDeployment (higher-level) instead of MachineSet (lower-level)
4. **Annotation Compatibility**: Keep existing annotation keys for seamless cluster-autoscaler integration
5. **Authentication Simplification**: IRSA-only approach eliminates need for complex credential resolution
6. **OpenShift Decoupling**: Remove all OpenShift dependencies for a pure CAPI/Kubernetes solution
7. **Region Resolution**: Flexible approach (AWSCluster first, annotation fallback) handles various CAPI setups
8. **Template Resolution**: New critical path - fetch and validate AWSMachineTemplate from infrastructureRef
9. **Namespace Flexibility**: Maintain all-namespace watching for multi-tenant and varied CAPI deployments
10. **Reusable Components**: EC2 instance type caching, AWS client builder, and IRSA auth remain unchanged