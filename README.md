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

### AWS Authentication

The controller supports two authentication methods:

#### 1. IRSA (IAM Roles for Service Accounts) - Recommended

IRSA provides a more secure authentication method using projected service account tokens instead of static credentials.

**Prerequisites:**
- OpenShift cluster on AWS with OIDC provider configured
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
        image: quay.io/jhjaggars/capa-annotator:latest
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

**MachineSet Configuration:**

When using IRSA, the `credentialsSecret` field in the MachineSet is optional:

```yaml
apiVersion: machine.openshift.io/v1beta1
kind: MachineSet
metadata:
  name: my-machineset
  namespace: openshift-machine-api
spec:
  template:
    spec:
      providerSpec:
        value:
          instanceType: m5.large
          # credentialsSecret can be omitted when IRSA is configured
          placement:
            region: us-east-1
```

#### 2. Secret-based Authentication - Legacy/Fallback

AWS credentials can be provided via Kubernetes secrets referenced in MachineSet specs.

**MachineSet Configuration:**
```yaml
apiVersion: machine.openshift.io/v1beta1
kind: MachineSet
metadata:
  name: my-machineset
  namespace: openshift-machine-api
spec:
  template:
    spec:
      providerSpec:
        value:
          credentialsSecret:
            name: aws-cloud-credentials
          instanceType: m5.large
          placement:
            region: us-east-1
```

**AWS Credentials Secret:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-cloud-credentials
  namespace: openshift-machine-api
type: Opaque
data:
  aws_access_key_id: <base64-encoded-access-key>
  aws_secret_access_key: <base64-encoded-secret-key>
```

### Authentication Priority

The controller automatically selects the authentication method in the following priority:

1. **IRSA** (highest priority) - If both `AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE` environment variables are set
2. **Secret-based** (fallback) - If `credentialsSecret` is specified in the MachineSet
3. **Error** - If neither method is configured

**Benefits of IRSA:**
- ✅ No static credentials stored in Kubernetes secrets
- ✅ Automatic token rotation by Kubernetes
- ✅ Fine-grained IAM permissions per service account
- ✅ Better security posture and audit trail
- ✅ Works seamlessly with custom AWS endpoints (GovCloud, China regions)

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

- **Unit Tests**: 21 test cases covering reconciliation logic, annotation setting, architecture detection, and IRSA authentication
- **Test Coverage**: ~63% code coverage
- **Integration Tests**: 8 Ginkgo/BDD tests using envtest with a real Kubernetes API server

#### Quick Start

```bash
# Run unit tests (no setup required - recommended for local development)
make test-unit

# Generate coverage report
make test-coverage
open coverage.html
```

#### Integration Tests

Integration tests use [envtest](https://book.kubebuilder.io/reference/envtest.html) to run against a real Kubernetes API server. These tests verify the full controller lifecycle.

The integration tests use `setup-envtest` from controller-runtime, which automatically downloads the necessary Kubernetes binaries (etcd, kube-apiserver) for the specified version. **No manual installation or sudo required!**

```bash
# Run integration tests (automatically downloads K8s 1.33.2 binaries if needed)
make test-integration

# Run all tests (unit + integration)
make test
```

**How it works:**
- First run downloads kubebuilder assets to `./bin` (~100MB, one-time)
- Subsequent runs reuse cached binaries
- Pinned to Kubernetes 1.33.0 for reproducibility
- No sudo or system-wide installation needed
- Works on macOS, Linux, and Windows

#### Test Coverage

**Unit Tests Cover:**
- ✅ Empty instance type handling
- ✅ Standard instance types (a1.2xlarge, p2.16xlarge)
- ✅ GPU instances with proper GPU count
- ✅ ARM64 architecture detection (m6g.4xlarge)
- ✅ Missing/invalid architecture defaults to amd64
- ✅ Invalid instance types with graceful error handling
- ✅ Preservation of existing user annotations
- ✅ IRSA authentication with both environment variables
- ✅ IRSA partial configuration error handling
- ✅ IRSA priority over secret-based authentication
- ✅ Secret-based authentication fallback
- ✅ Custom AWS endpoint integration with IRSA

**Integration Tests Cover:**
- ✅ Full reconciliation loop with real Kubernetes API
- ✅ Annotation updates on MachineSet resources
- ✅ Event recording for errors
- ✅ Feature gate behavior (MachineAPIMigration)

#### CI/CD

All tests run automatically in GitHub Actions:
- Unit tests run on every push and PR
- Integration tests run in a separate job using setup-envtest
- Coverage reports are uploaded to Codecov
- No special CI setup required - same Makefile targets work everywhere
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
