# CAPA Annotator Deployment Manifests

This directory contains production-ready Kubernetes deployment manifests for the CAPA Annotator controller.

## Overview

The manifests follow Kubernetes best practices and include:

- **High Availability**: 2 replicas with leader election, pod anti-affinity, and topology spread constraints
- **Security**: Non-root user, read-only filesystem, dropped capabilities, seccomp profile
- **Observability**: Metrics endpoint, health checks, Prometheus ServiceMonitor
- **Resilience**: PodDisruptionBudget, resource limits, proper liveness/readiness probes
- **IRSA Support**: Configured for IAM Roles for Service Accounts on AWS

## Prerequisites

1. Kubernetes cluster with Cluster API (CAPI) and Cluster API Provider AWS (CAPA) installed
2. AWS IAM role configured for IRSA with the following permissions:
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
3. (Optional) Prometheus Operator installed for ServiceMonitor support

## Quick Start

### 1. Configure IRSA

Before deploying, configure the IAM role ARN in the ServiceAccount:

**Edit `serviceaccount.yaml`:**
```yaml
annotations:
  eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/capa-annotator-role"
```

Replace `ACCOUNT_ID` and `capa-annotator-role` with your actual AWS account ID and IAM role name.

**Note**: When using IRSA, Kubernetes automatically injects the required AWS environment variables (`AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE`) into the pod. You do not need to edit `deployment.yaml` - the ServiceAccount annotation is sufficient.

### 2. Deploy

Apply all manifests in order:

```bash
# Apply in dependency order
kubectl apply -f namespace.yaml
kubectl apply -f serviceaccount.yaml
kubectl apply -f rbac.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f poddisruptionbudget.yaml

# Optional: If you have Prometheus Operator installed
kubectl apply -f servicemonitor.yaml
```

Or apply all at once:

```bash
kubectl apply -f deploy/
```

### 3. Verify Deployment

Check that the controller is running:

```bash
# Check pod status
kubectl get pods -n capa-annotator-system

# Check logs
kubectl logs -n capa-annotator-system -l app.kubernetes.io/name=capa-annotator -f

# Check leader election
kubectl get leases -n capa-annotator-system
```

## Configuration Options

### Container Image

To use a different container image, edit `deployment.yaml`:

```yaml
containers:
- name: controller
  image: quay.io/your-org/capa-annotator:your-tag
```

### Scaling

The deployment is configured for HA with 2 replicas. To change the replica count:

```yaml
spec:
  replicas: 3  # Increase for higher availability
```

**Note**: When using more than 1 replica, ensure `--leader-elect=true` is set (already configured).

### Namespace Watching

By default, the controller watches all namespaces. To watch a specific namespace:

```yaml
args:
- --namespace=my-namespace
```

### Resource Limits

Adjust resource requests/limits based on your cluster size:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

### AWS Region

If you need to set a default AWS region (optional):

```yaml
env:
- name: AWS_REGION
  value: "us-west-2"
```

## IAM Role Setup for IRSA

### EKS Clusters

1. Create IAM policy with required permissions (see Prerequisites)
2. Create IAM role with trust relationship:

```bash
eksctl create iamserviceaccount \
  --name=capa-annotator \
  --namespace=capa-annotator-system \
  --cluster=your-cluster-name \
  --attach-policy-arn=arn:aws:iam::ACCOUNT_ID:policy/capa-annotator-policy \
  --approve \
  --override-existing-serviceaccounts
```

### CAPI Clusters with OIDC Provider

1. Get your cluster's OIDC provider URL from the AWSCluster resource
2. Create IAM role with trust policy:

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
          "OIDC_PROVIDER_URL:sub": "system:serviceaccount:capa-annotator-system:capa-annotator"
        }
      }
    }
  ]
}
```

3. Update ServiceAccount annotation with the correct format for your OIDC provider

## Troubleshooting

### Check controller logs

```bash
kubectl logs -n capa-annotator-system -l app.kubernetes.io/name=capa-annotator --tail=100
```

### Verify RBAC permissions

```bash
kubectl auth can-i get machinedeployments --as=system:serviceaccount:capa-annotator-system:capa-annotator --all-namespaces
```

### Test AWS authentication

```bash
kubectl exec -n capa-annotator-system deployment/capa-annotator -- env | grep AWS
```

### Check metrics

```bash
kubectl port-forward -n capa-annotator-system svc/capa-annotator-metrics 8080:8080
curl http://localhost:8080/metrics
```

### Verify health checks

```bash
kubectl port-forward -n capa-annotator-system svc/capa-annotator-metrics 9440:9440
curl http://localhost:9440/healthz
curl http://localhost:9440/readyz
```

## File Descriptions

- **namespace.yaml**: Creates the `capa-annotator-system` namespace
- **serviceaccount.yaml**: ServiceAccount with IRSA annotations for AWS authentication
- **rbac.yaml**: ClusterRole and ClusterRoleBinding with required CAPI permissions
- **deployment.yaml**: Main controller deployment with HA configuration
- **service.yaml**: Service exposing metrics endpoint for Prometheus
- **servicemonitor.yaml**: Prometheus Operator ServiceMonitor for metrics collection
- **poddisruptionbudget.yaml**: PDB ensuring at least 1 replica during disruptions

## Monitoring

The controller exposes Prometheus metrics on port 8080. If you have Prometheus Operator installed:

```bash
# Check ServiceMonitor
kubectl get servicemonitor -n capa-annotator-system

# View metrics in Prometheus UI
# The ServiceMonitor will automatically configure Prometheus to scrape the controller
```

## Uninstalling

To remove the controller:

```bash
kubectl delete -f deploy/
```

Or individually:

```bash
kubectl delete -f servicemonitor.yaml  # Optional
kubectl delete -f poddisruptionbudget.yaml
kubectl delete -f service.yaml
kubectl delete -f deployment.yaml
kubectl delete -f rbac.yaml
kubectl delete -f serviceaccount.yaml
kubectl delete -f namespace.yaml
```

## Production Considerations

1. **Image Pull Policy**: Change to `Always` for production to ensure latest security patches
2. **Resource Limits**: Adjust based on your cluster size and MachineDeployment count
3. **Monitoring**: Set up alerts for controller health and AWS API errors
4. **Backup**: Ensure RBAC and deployment configurations are version controlled
5. **Updates**: Use rolling updates with proper testing in staging first
6. **Security**: Regularly rotate IAM role credentials and review permissions
