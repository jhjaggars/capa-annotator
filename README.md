# CAPA Annotator

A standalone Kubernetes controller that assigns CPU, memory, GPU, and
architecture annotations to MachineDeployment objects by querying AWS EC2
instance type information.

## Overview

This controller provides annotation functionality for Cluster API (CAPI)
MachineDeployments on AWS. It enables cluster-autoscaler to scale from zero by
providing instance capacity information via annotations.

## How It Works

The controller watches MachineDeployment resources in your cluster and:

1. Extracts the instance type from the MachineDeployment's AWSMachineTemplate
2. Queries the AWS EC2 API for instance type details (CPU, memory, GPU, architecture)
3. Caches the instance type information for 24 hours
4. Sets the following annotations on the MachineDeployment:
   - `machine.openshift.io/vCPU` - Number of vCPUs for the instance type
   - `machine.openshift.io/memoryMb` - Memory in MB for the instance type
   - `machine.openshift.io/GPU` - Number of GPUs for the instance type
   - `capacity.cluster-autoscaler.kubernetes.io/labels` - Architecture label (e.g., `kubernetes.io/arch=amd64`)

## Deployment

### Prerequisites

- Kubernetes cluster with Cluster API (CAPI) and Cluster API Provider AWS (CAPA) installed
- AWS credentials configured (via IRSA, environment variables, or ~/.aws/credentials)
- Network access to AWS EC2 API endpoints

### Building

```bash
# Build the binary
make build

# Build the container image (single architecture)
make image IMAGE_TAG=v0.1.0

# Build multi-architecture container image (amd64 + arm64)
make image-multiarch IMAGE_TAG=v0.1.0

# Push the container image (single architecture)
make push IMAGE_TAG=v0.1.0

# Push multi-architecture container image
make push-multiarch IMAGE_TAG=v0.1.0
```

### Using Pre-built Images

Multi-architecture images are available from GitHub Container Registry:

```bash
# Pull the latest image (automatically selects the correct architecture)
podman pull ghcr.io/jhjaggars/capa-annotator:latest

# Pull a specific version
podman pull ghcr.io/jhjaggars/capa-annotator:v0.1.0
```

Supported architectures:
- `linux/amd64` (x86_64)
- `linux/arm64` (ARM64/aarch64)

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
        image: ghcr.io/jhjaggars/capa-annotator:latest
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

### AWS Authentication

The controller supports two authentication methods:

#### 1. IRSA (IAM Roles for Service Accounts) - Recommended

IRSA provides a more secure authentication method using projected service account tokens instead of static credentials.

**Prerequisites:**
- Kubernetes cluster on AWS with OIDC provider configured (e.g., EKS, CAPI-managed cluster)
- IAM role with appropriate EC2 permissions
- IAM role trust policy configured for the cluster's OIDC provider

**IAM Role Trust Policy Example:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT_ID:oidc-provider/OIDC_PROVIDER_URL"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "OIDC_PROVIDER_URL:sub": "system:serviceaccount:openshift-machine-api:capa-annotator"
        }
      }
    }
  ]
}
```

**IAM Role Permissions Required:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstanceTypes",
        "ec2:DescribeRegions"
      ],
      "Resource": "*"
    }
  ]
}
```

**Deployment Configuration:**
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: capa-annotator
  namespace: openshift-machine-api
  annotations:
    # Note: Annotation format may vary based on OIDC provider
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/capa-annotator-role"
---
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
        image: ghcr.io/jhjaggars/capa-annotator:latest
        env:
        - name: AWS_ROLE_ARN
          value: "arn:aws:iam::ACCOUNT_ID:role/capa-annotator-role"
        - name: AWS_WEB_IDENTITY_TOKEN_FILE
          value: "/var/run/secrets/eks.amazonaws.com/serviceaccount/token"
        volumeMounts:
        - name: aws-iam-token
          mountPath: "/var/run/secrets/eks.amazonaws.com/serviceaccount"
          readOnly: true
      volumes:
      - name: aws-iam-token
        projected:
          sources:
          - serviceAccountToken:
              audience: sts.amazonaws.com
              expirationSeconds: 3600
              path: token
```

**Example CAPI MachineDeployment:**

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: my-workers
  namespace: default
spec:
  clusterName: my-cluster
  replicas: 3
  template:
    spec:
      clusterName: my-cluster
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: my-workers
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
        kind: AWSMachineTemplate
        name: my-workers
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AWSMachineTemplate
metadata:
  name: my-workers
  namespace: default
spec:
  template:
    spec:
      instanceType: m5.large
      # AWS credentials come from IRSA or default credential chain
```

#### 2. Default Credential Chain - Fallback

When IRSA is not configured, the controller falls back to the default AWS credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. EC2 instance metadata (for controllers running on EC2)

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
- Podman (for building container images)

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

#### Quick Start

```bash
# Run unit tests (no setup required - recommended for local development)
make test-unit

# Generate coverage report
make test-coverage
open coverage.html
```

#### Integration Tests

Integration tests use [envtest](https://book.kubebuilder.io/reference/envtest.html) to run against a Kubernetes API server.

The integration tests use `setup-envtest` from controller-runtime, which
automatically downloads the necessary Kubernetes binaries (etcd,
kube-apiserver) for the specified version.

```bash
# Run integration tests (automatically downloads K8s 1.33.2 binaries if needed)
make test-integration

# Run all tests (unit + integration)
make test
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
