# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CAPA Annotator is a standalone Kubernetes controller that assigns CPU, memory, GPU, and architecture annotations to CAPI (Cluster API) MachineDeployment objects by querying AWS EC2 instance type information. This enables cluster-autoscaler to scale from zero by providing instance capacity information via annotations.

The controller watches MachineDeployment resources and:
1. Resolves the associated AWSMachineTemplate to extract instance type
2. Determines the AWS region from the AWSCluster or annotation fallback
3. Queries AWS EC2 API for instance type details (cached for 24 hours)
4. Sets annotations: `machine.openshift.io/vCPU`, `machine.openshift.io/memoryMb`, `machine.openshift.io/GPU`, and `capacity.cluster-autoscaler.kubernetes.io/labels`

## Spec-Driven Development

This project uses [specware](https://github.com/tiwillia/specware) for planning and specification. **The `.spec/` directory is treated as code** and changes to spec files must be committed and included in pull requests.

### Specware Commands
```bash
# Create a new feature specification
specware feature new-requirements <short-name>

# Add implementation planning
specware feature new-implementation-plan <short-name>
```

### Using Specifications
- Use the `/specify` command in Claude Code to gather requirements interactively
- Read existing specifications in `.spec/NNN-feature-name/` directories:
  - `requirements.md`: Feature requirements and details
  - `implementation-plan.md`: Implementation approach
  - `q-a-requirements.md` and `q-a-implementation-plan.md`: Q&A context
  - `.spec-status`: Current phase of the specification
- When implementing features, follow the spec-driven workflow and commit all .spec changes with code changes

## Build and Test Commands

### Building
```bash
# Build the binary
make build

# Build container image
make image IMAGE_NAME=quay.io/username/capa-annotator IMAGE_TAG=v0.1.0

# Push container image
make push IMAGE_NAME=quay.io/username/capa-annotator IMAGE_TAG=v0.1.0
```

### Testing
```bash
# Run unit tests only (recommended for local development, no external dependencies)
make test-unit

# Run integration tests (uses envtest, automatically downloads K8s 1.33.0 binaries if needed)
make test-integration

# Run all tests (unit + integration)
make test

# Generate coverage report
make test-coverage
# Opens coverage.html

# Run tests with race detector
make test-race
```

### Code Quality
```bash
# Format code
make fmt

# Run go vet
make vet

# Run linter (if golangci-lint installed)
make lint

# Tidy dependencies
make tidy
```

### Running Locally
```bash
# Run controller against a cluster
export KUBECONFIG=/path/to/kubeconfig
./bin/capa-annotator --leader-elect=false
```

## Architecture

### Core Components

**Controller** (`pkg/controller/controller.go`):
- Reconciles `MachineDeployment` resources from Cluster API
- Main reconciliation loop in `Reconcile()` method
- Uses `utils.ResolveAWSMachineTemplate()` to get the AWSMachineTemplate
- Uses `utils.ExtractInstanceType()` to get instance type from template
- Uses `utils.ResolveRegion()` to determine AWS region (from AWSCluster or annotation)
- Queries AWS EC2 API via cached `InstanceTypesCache`
- Sets annotations on MachineDeployment with instance capacity information

**AWS Client** (`pkg/client/client.go`):
- Wrapper around AWS SDK clients (EC2, ELB, ELBv2)
- Authentication methods (in priority order):
  1. IRSA (IAM Roles for Service Accounts) via `AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE` env vars
  2. Default AWS credential chain (env vars, ~/.aws/credentials, EC2 metadata)
- `NewValidatedClient()` validates region before returning client
- `RegionCache` caches DescribeRegions API calls for 30 minutes

**Utils** (`pkg/utils/providerspec.go`):
- `ResolveAWSMachineTemplate()`: Fetches AWSMachineTemplate from MachineDeployment's infrastructureRef
- `ExtractInstanceType()`: Extracts instance type from AWSMachineTemplate spec
- `ResolveRegion()`: Gets AWS region from AWSCluster or falls back to `capa.infrastructure.cluster.x-k8s.io/region` annotation

**Instance Types Cache** (`pkg/controller/ec2_instance_types.go`):
- Caches EC2 instance type information for 24 hours per region
- Thread-safe with mutex-protected cache
- Returns `InstanceTypeInfo` with VCPU, MemoryMb, GPU, and CPUArchitecture

### Data Flow
1. Controller watches MachineDeployment resources
2. On reconcile, resolves AWSMachineTemplate → extract instance type
3. Resolves AWS region from AWSCluster or annotation
4. Creates/validates AWS client for region
5. Queries instance type info (cached)
6. Updates MachineDeployment annotations
7. Patches MachineDeployment back to API server

### Key Design Decisions

**CAPI Migration**: The controller was migrated from watching OpenShift MachineSets to CAPI MachineDeployments. See `.spec/003-capi-machineset-support/` for the spec that guided this migration.

**Authentication**: Supports IRSA (preferred) and falls back to default credential chain. IRSA configuration is detected via environment variables, no explicit code changes needed.

**Caching**: Two-tier caching strategy:
- RegionCache: 30-minute TTL for AWS region data
- InstanceTypesCache: 24-hour TTL for instance type information

**Label Preservation**: When setting the `capacity.cluster-autoscaler.kubernetes.io/labels` annotation, the controller preserves existing user-provided labels and only updates the architecture label.

## Dependencies

- Go 1.24+
- Cluster API v1.10.3
- Cluster API Provider AWS v2.9.0
- controller-runtime v0.20.4
- AWS SDK Go v1.55.7

## Testing Notes

- Unit tests use `-short` flag to skip integration tests
- Integration tests require envtest (setup-envtest@release-0.20)
- Envtest automatically downloads Kubernetes binaries (etcd, kube-apiserver) for version 1.33.0
- Tests use Ginkgo/Gomega framework
- Fake AWS client available in `pkg/client/fake/fake.go`

## Project Origin

Based on code from [openshift/machine-api-provider-aws](https://github.com/openshift/machine-api-provider-aws), extracted and adapted for standalone CAPI usage.
