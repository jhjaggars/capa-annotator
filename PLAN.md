# MachineSet Annotator - Implementation Plan

## Overview

Create a standalone Kubernetes controller that assigns CPU, memory, GPU, and architecture annotations to MachineSet objects by querying AWS EC2 instance type information.

## Purpose

This controller replicates the annotation functionality from `openshift/machine-api-provider-aws` but as a completely independent project. It enables cluster-autoscaler to scale from zero by providing instance capacity information via annotations.

## Annotations Set

The controller will set these annotations on MachineSet objects:

- `machine.openshift.io/vCPU` - Number of vCPUs for the instance type
- `machine.openshift.io/memoryMb` - Memory in MB for the instance type
- `machine.openshift.io/GPU` - Number of GPUs for the instance type
- `capacity.cluster-autoscaler.kubernetes.io/labels` - Architecture label (e.g., `kubernetes.io/arch=amd64`)

## Project Structure

```
/Users/jjaggars/code/capa-annotator/
├── cmd/
│   └── controller/
│       └── main.go                    # Main entry point
├── pkg/
│   ├── controller/
│   │   ├── controller.go              # MachineSet reconciler
│   │   └── ec2_instance_types.go      # Instance type cache
│   ├── client/
│   │   └── client.go                  # AWS client wrapper
│   └── utils/
│       └── providerspec.go            # Provider spec parsing
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── README.md
└── PLAN.md                            # This file
```

## Implementation Steps

### 1. Initialize Go Module

Create `go.mod` with:
- Module name: `github.com/jjaggars/capa-annotator`
- Required dependencies:
  - `github.com/aws/aws-sdk-go`
  - `github.com/openshift/api` (for `machine/v1beta1`, `config/v1`)
  - `github.com/openshift/machine-api-operator` (for utilities and error types)
  - `sigs.k8s.io/controller-runtime`
  - `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`
  - `k8s.io/klog/v2`
  - `k8s.io/component-base` (for feature gates)

### 2. Copy Core Controller Logic

**Source:** `/Users/jjaggars/machine-api-provider-aws/pkg/actuators/machineset/controller.go`

**Target:** `pkg/controller/controller.go`

**Changes needed:**
- Update package from `machineset` to `controller`
- Update imports to use new module path for internal packages
- Keep all reconciliation logic intact:
  - Fetch MachineSet
  - Handle deletion timestamps
  - Handle feature gate for MachineAPI migration
  - Parse provider config
  - Query instance type info
  - Set annotations

### 3. Copy Instance Type Cache

**Source:** `/Users/jjaggars/machine-api-provider-aws/pkg/actuators/machineset/ec2_instance_types.go`

**Target:** `pkg/controller/ec2_instance_types.go`

**Changes needed:**
- Update package to `controller`
- Update imports for new module path
- Keep all cache logic:
  - `InstanceType` struct (vCPU, memory, GPU, architecture)
  - `InstanceTypesCache` interface
  - Thread-safe cache with 24-hour refresh
  - EC2 `DescribeInstanceTypes` pagination
  - Architecture normalization (x86_64 → amd64, arm64 → arm64)

### 4. Copy AWS Client Code

**Source:** `/Users/jjaggars/machine-api-provider-aws/pkg/client/client.go`

**Target:** `pkg/client/client.go`

**Keep all functionality:**
- `Client` interface with EC2/ELB methods
- `AwsClientBuilderFuncType` function type
- `NewClient()` - Create client from secret credentials
- `NewValidatedClient()` - Create client with region validation
- `RegionCache` for caching DescribeRegions calls
- Custom CA bundle support
- Custom endpoint resolution
- Session creation with credentials from Kubernetes secrets

### 5. Copy Provider Spec Utilities

**Source:** `/Users/jjaggars/machine-api-provider-aws/pkg/actuators/machine/utils.go`

**Target:** `pkg/utils/providerspec.go`

**Extract only needed functions:**
- `ProviderSpecFromRawExtension()` - Parse AWSMachineProviderConfig from RawExtension
- Any helper functions it depends on

### 6. Create Main Entry Point

**Target:** `cmd/controller/main.go`

**Based on:** `/Users/jjaggars/machine-api-provider-aws/cmd/manager/main.go`

**Simplified version that:**
- Parses flags (metrics address, namespace, leader election, health address, feature gates)
- Sets up controller-runtime Manager
- Registers schemes (machinev1beta1, machinev1, configv1, corev1)
- Creates config-managed client for openshift-config-managed namespace
- Initializes region cache and instance types cache
- Creates and registers only the MachineSet controller (no machine actuator)
- Sets up health/readiness checks
- Starts the manager

### 7. Create Makefile

**Targets:**
- `build` - Build the controller binary
- `test` - Run unit tests
- `image` - Build container image
- `clean` - Clean build artifacts
- `fmt` - Format code
- `vet` - Run go vet
- `lint` - Run golangci-lint (optional)

### 8. Create Dockerfile

**Multi-stage build:**
- Stage 1: Build the Go binary
- Stage 2: Create minimal runtime image with binary
- Use distroless or ubi-minimal as base

### 9. Create README.md

**Sections:**
- Overview and purpose
- How it works
- Deployment instructions
- Configuration (flags, environment variables)
- Required RBAC permissions
- Development setup
- Building and testing

## Key Dependencies to Copy

From the original codebase, we need:

1. **Controller logic** (`controller.go`):
   - Reconcile loop
   - Feature gate handling
   - Provider spec parsing
   - Instance type querying
   - Annotation setting

2. **Instance type cache** (`ec2_instance_types.go`):
   - Cache implementation
   - AWS API pagination
   - Architecture normalization

3. **AWS client** (`client.go`):
   - Client interface
   - Session creation
   - Credential handling
   - Region validation
   - Custom CA bundle support

4. **Utilities** (`utils.go`):
   - Provider spec parsing

## RBAC Requirements

The controller will need these permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: machineset-annotator
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

## Testing Strategy

1. **Unit tests:**
   - Instance type cache behavior
   - Annotation logic
   - Provider spec parsing

2. **Integration tests (optional):**
   - Full reconciliation with fake AWS client
   - Cache refresh behavior

## Configuration

**Command-line flags:**
- `--metrics-bind-address` - Metrics endpoint (default: `:8080`)
- `--namespace` - Watch specific namespace (default: all namespaces)
- `--leader-elect` - Enable leader election (default: false)
- `--leader-elect-resource-namespace` - Namespace for leader election
- `--leader-elect-lease-duration` - Lease duration (default: 120s)
- `--health-addr` - Health check address (default: `:9440`)
- `--feature-gates` - Feature gate configuration

**Environment variables:**
- AWS credentials from Kubernetes secrets (referenced in MachineSet provider spec)

## Deployment Notes

1. Deploy as a Deployment with 1-3 replicas (with leader election)
2. Requires access to AWS credentials (via referenced secrets in MachineSet specs)
3. Needs network access to AWS EC2 API endpoints
4. Should run in the same namespace as other machine-api components (typically `openshift-machine-api`)

## Future Enhancements

- Support for other cloud providers (Azure, GCP, etc.)
- Metrics for cache hit/miss rates
- Prometheus metrics for reconciliation success/failure
- Webhook for validating MachineSet annotations
