# CAPA Annotator

A standalone Kubernetes controller that assigns CPU, memory, GPU, and architecture annotations to MachineSet objects by querying AWS EC2 instance type information.

## Overview

This controller replicates the annotation functionality from `openshift/machine-api-provider-aws` but as a completely independent project. It enables cluster-autoscaler to scale from zero by providing instance capacity information via annotations.

## How It Works

The controller watches MachineSet resources in your cluster and:

1. Extracts the instance type from the MachineSet's AWS provider spec
2. Queries the AWS EC2 API for instance type details (CPU, memory, GPU, architecture)
3. Caches the instance type information for 24 hours
4. Sets the following annotations on the MachineSet:
   - `machine.openshift.io/vCPU` - Number of vCPUs for the instance type
   - `machine.openshift.io/memoryMb` - Memory in MB for the instance type
   - `machine.openshift.io/GPU` - Number of GPUs for the instance type
   - `capacity.cluster-autoscaler.kubernetes.io/labels` - Architecture label (e.g., `kubernetes.io/arch=amd64`)

## Deployment

### Prerequisites

- Kubernetes cluster with OpenShift Machine API installed
- AWS credentials configured (via secrets referenced in MachineSet specs)
- Network access to AWS EC2 API endpoints

### Building

```bash
# Build the binary
make build

# Build the container image
make image IMAGE_NAME=quay.io/yourusername/capa-annotator IMAGE_TAG=v0.1.0

# Push the container image
make push IMAGE_NAME=quay.io/yourusername/capa-annotator IMAGE_TAG=v0.1.0
```

### Deploying to Kubernetes

The controller should be deployed as a Deployment with 1-3 replicas (with leader election enabled for high availability).

Example deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: capa-annotator
  namespace: openshift-machine-api
spec:
  replicas: 1
  selector:
    matchLabels:
      app: capa-annotator
  template:
    metadata:
      labels:
        app: capa-annotator
    spec:
      serviceAccountName: capa-annotator
      containers:
      - name: controller
        image: quay.io/jhjaggars/capa-annotator:latest
        args:
        - --leader-elect=true
        - --metrics-bind-address=:8080
        - --health-addr=:9440
        ports:
        - containerPort: 8080
          name: metrics
        - containerPort: 9440
          name: health
        livenessProbe:
          httpGet:
            path: /healthz
            port: health
        readinessProbe:
          httpGet:
            path: /readyz
            port: health
```

## Configuration

### Command-line Flags

- `--version` - Print version and exit
- `--metrics-bind-address` - Address for hosting metrics (default: `:8080`)
- `--namespace` - Watch specific namespace (default: all namespaces)
- `--leader-elect` - Enable leader election (default: `false`)
- `--leader-elect-resource-namespace` - Namespace for leader election
- `--leader-elect-lease-duration` - Lease duration (default: `120s`)
- `--health-addr` - Health check address (default: `:9440`)
- `--feature-gates` - Feature gate configuration

### Environment Variables

AWS credentials are read from Kubernetes secrets referenced in the MachineSet provider specs.

## RBAC Requirements

The controller requires the following permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: capa-annotator
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

## Development

### Prerequisites

- Go 1.24 or later
- Docker (for building container images)

### Building

```bash
# Download dependencies
go mod download

# Build the binary
make build

# Run all tests (includes integration tests - requires kubebuilder)
make test

# Run unit tests only (no external dependencies)
make test-unit

# Run tests with coverage report
make test-coverage

# Run tests with race detector
make test-race

# Format code
make fmt

# Run go vet
make vet

# Clean build artifacts
make clean
```

### Testing

The project includes comprehensive test coverage:

- **Unit Tests**: 11 test cases covering reconciliation logic, annotation setting, and architecture detection
- **Test Coverage**: ~63% code coverage
- **Integration Tests**: Ginkgo/BDD tests (require kubebuilder binaries)

```bash
# Run unit tests (recommended for local development)
make test-unit

# Generate coverage report (opens in browser)
make test-coverage
open coverage.html

# Run all tests including integration tests
# Note: Requires kubebuilder to be installed
make test
```

**Test Coverage Includes:**
- ✅ Empty instance type handling
- ✅ Standard instance types (a1.2xlarge, p2.16xlarge)
- ✅ GPU instances with proper GPU count
- ✅ ARM64 architecture detection (m6g.4xlarge)
- ✅ Missing/invalid architecture defaults to amd64
- ✅ Invalid instance types with graceful error handling
- ✅ Preservation of existing user annotations
```

### Testing Locally

You can run the controller locally against a Kubernetes cluster:

```bash
# Ensure your kubeconfig is pointing to the correct cluster
export KUBECONFIG=/path/to/kubeconfig

# Run the controller
./bin/capa-annotator --leader-elect=false
```

## License

Licensed under the Apache License, Version 2.0. See the LICENSE file for details.

## Credits

This project is based on code from the [openshift/machine-api-provider-aws](https://github.com/openshift/machine-api-provider-aws) project.
