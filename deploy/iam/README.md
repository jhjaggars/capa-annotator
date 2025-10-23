# CAPA Annotator IAM Setup Guide

This directory contains IAM policy templates and setup instructions for configuring AWS IAM Roles for Service Accounts (IRSA) with the CAPA Annotator controller.

## Overview

The CAPA Annotator controller requires minimal AWS permissions to function:

- **`ec2:DescribeInstanceTypes`** - Query instance type details (CPU, memory, GPU, architecture)
- **`ec2:DescribeRegions`** - Validate AWS regions (cached for 30 minutes)

These are **read-only** operations with no resource modification capabilities.

## Files in This Directory

- **`policy.json`** - IAM permissions policy (minimal EC2 describe permissions)
- **`trust-policy.json`** - OIDC trust policy template for AssumeRoleWithWebIdentity
- **`terraform/`** - Complete Terraform module for IAM role and policy creation

## Prerequisites

Before setting up IAM, you need:

1. **OIDC Provider URL** for your Kubernetes cluster
2. **AWS Account ID**
3. **Namespace** where CAPA Annotator will be deployed (default: `capa-annotator-system`)
4. **Service Account name** (default: `capa-annotator`)

### Finding Your OIDC Provider URL

The OIDC provider URL format varies by cluster type:

#### For EKS Clusters

```bash
# Get OIDC provider URL from EKS cluster
aws eks describe-cluster --name YOUR_CLUSTER_NAME --query "cluster.identity.oidc.issuer" --output text
# Example output: https://oidc.eks.us-west-2.amazonaws.com/id/EXAMPLED539D4633E53DE1B71EXAMPLE
```

Remove the `https://` prefix to get: `oidc.eks.us-west-2.amazonaws.com/id/EXAMPLED539D4633E53DE1B71EXAMPLE`

#### For CAPI Clusters

Check your AWSCluster resource for OIDC configuration:

```bash
kubectl get awscluster YOUR_CLUSTER_NAME -o jsonpath='{.status.oidcProvider.url}'
# Or check the identity provider configuration
kubectl get awscluster YOUR_CLUSTER_NAME -o yaml
```

For CAPA-managed clusters, look for the S3 OIDC discovery endpoint in the cluster status.

#### Verify OIDC Provider Exists in AWS

```bash
aws iam list-open-id-connect-providers
```

Look for your OIDC provider URL in the output. If it doesn't exist, you need to create it first.

## Setup Methods

Choose one of the following methods to create the IAM role:

### Method 1: AWS CLI (Recommended for Quick Setup)

#### Step 1: Set Environment Variables

```bash
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
export OIDC_PROVIDER_URL="oidc.eks.us-west-2.amazonaws.com/id/EXAMPLED539D4633E53DE1B71EXAMPLE"
export NAMESPACE="capa-annotator-system"
export SERVICE_ACCOUNT_NAME="capa-annotator"
export ROLE_NAME="capa-annotator-role"
export POLICY_NAME="capa-annotator-policy"
```

**Important**: Replace the `OIDC_PROVIDER_URL` with your actual OIDC provider URL (without `https://` prefix).

#### Step 2: Create IAM Policy

```bash
aws iam create-policy \
  --policy-name "${POLICY_NAME}" \
  --policy-document file://policy.json \
  --description "Policy for CAPA Annotator controller"
```

This will output the policy ARN. Save it for the next step:

```bash
export POLICY_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:policy/${POLICY_NAME}"
```

#### Step 3: Create Trust Policy from Template

Create a temporary trust policy file with your specific values:

```bash
cat > trust-policy-configured.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::${AWS_ACCOUNT_ID}:oidc-provider/${OIDC_PROVIDER_URL}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "${OIDC_PROVIDER_URL}:sub": "system:serviceaccount:${NAMESPACE}:${SERVICE_ACCOUNT_NAME}",
          "${OIDC_PROVIDER_URL}:aud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
EOF
```

#### Step 4: Create IAM Role

```bash
aws iam create-role \
  --role-name "${ROLE_NAME}" \
  --assume-role-policy-document file://trust-policy-configured.json \
  --description "IAM role for CAPA Annotator controller using IRSA"
```

#### Step 5: Attach Policy to Role

```bash
aws iam attach-role-policy \
  --role-name "${ROLE_NAME}" \
  --policy-arn "${POLICY_ARN}"
```

#### Step 6: Verify Role Creation

```bash
aws iam get-role --role-name "${ROLE_NAME}"
aws iam list-attached-role-policies --role-name "${ROLE_NAME}"
```

#### Step 7: Get Role ARN

```bash
aws iam get-role --role-name "${ROLE_NAME}" --query 'Role.Arn' --output text
```

Save this ARN - you'll need it for the Kubernetes ServiceAccount annotation.

### Method 2: Terraform (Recommended for Production)

#### Step 1: Create terraform.tfvars

Create a `terraform.tfvars` file in the `terraform/` directory:

```hcl
# terraform/terraform.tfvars
oidc_provider_url     = "oidc.eks.us-west-2.amazonaws.com/id/EXAMPLED539D4633E53DE1B71EXAMPLE"
namespace             = "capa-annotator-system"
service_account_name  = "capa-annotator"
role_name             = "capa-annotator-role"
policy_name           = "capa-annotator-policy"

tags = {
  Environment = "production"
  ManagedBy   = "Terraform"
  Component   = "capa-annotator"
}
```

**Important**: Replace `oidc_provider_url` with your actual OIDC provider URL (without `https://` prefix).

#### Step 2: Initialize Terraform

```bash
cd terraform/
terraform init
```

#### Step 3: Plan and Apply

```bash
# Review the changes
terraform plan

# Apply the configuration
terraform apply
```

#### Step 4: Get Role ARN

```bash
terraform output role_arn
```

Save this ARN for the Kubernetes ServiceAccount annotation.

## Configuring Kubernetes

After creating the IAM role, configure the CAPA Annotator ServiceAccount:

### Option 1: Edit serviceaccount.yaml

Edit `deploy/serviceaccount.yaml`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: capa-annotator
  namespace: capa-annotator-system
  annotations:
    # For EKS clusters:
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/capa-annotator-role"

    # For other CAPI clusters, check your OIDC provider documentation
    # The annotation key may differ based on your OIDC implementation
```

### Option 2: Patch Existing ServiceAccount

```bash
kubectl annotate serviceaccount capa-annotator \
  -n capa-annotator-system \
  eks.amazonaws.com/role-arn="arn:aws:iam::ACCOUNT_ID:role/capa-annotator-role"
```

## Verification

### 1. Verify ServiceAccount Annotation

```bash
kubectl get serviceaccount capa-annotator -n capa-annotator-system -o yaml
```

Look for the `eks.amazonaws.com/role-arn` annotation.

### 2. Verify Pod Environment Variables

After deploying the controller:

```bash
kubectl get pods -n capa-annotator-system
kubectl exec -n capa-annotator-system POD_NAME -- env | grep AWS
```

You should see:
- `AWS_ROLE_ARN` - Your IAM role ARN
- `AWS_WEB_IDENTITY_TOKEN_FILE` - Path to the projected token

### 3. Check Controller Logs

```bash
kubectl logs -n capa-annotator-system -l app.kubernetes.io/name=capa-annotator --tail=50
```

Look for:
- `"Using IRSA authentication with role: arn:aws:iam::..."`
- No AWS authentication errors

### 4. Test AWS API Access

The controller should successfully query AWS APIs. Check for successful reconciliation:

```bash
# Check MachineDeployment annotations
kubectl get machinedeployment YOUR_MD -o yaml | grep -A 5 annotations
```

Look for annotations like:
- `machine.openshift.io/vCPU`
- `machine.openshift.io/memoryMb`
- `machine.openshift.io/GPU`
- `capacity.cluster-autoscaler.kubernetes.io/labels`

## Troubleshooting

### OIDC Provider Not Found

**Error**: `InvalidIdentityToken` or `An error occurred (AccessDenied) when calling the AssumeRoleWithWebIdentity operation`

**Solution**: Verify the OIDC provider exists in your AWS account:

```bash
aws iam list-open-id-connect-providers
```

If not found, you need to create it. For EKS:

```bash
eksctl utils associate-iam-oidc-provider --cluster YOUR_CLUSTER_NAME --approve
```

For CAPI clusters, refer to your cluster's OIDC setup documentation.

### Trust Relationship Mismatch

**Error**: `Not authorized to perform sts:AssumeRoleWithWebIdentity`

**Solution**: Verify the trust policy subject matches exactly:

```bash
# Expected format
system:serviceaccount:NAMESPACE:SERVICE_ACCOUNT_NAME

# Check your role's trust policy
aws iam get-role --role-name capa-annotator-role --query 'Role.AssumeRolePolicyDocument'
```

Ensure:
1. Namespace matches exactly
2. Service account name matches exactly
3. OIDC provider URL matches exactly (no `https://` prefix)

### Missing Permissions

**Error**: `AccessDenied` when calling `DescribeInstanceTypes` or `DescribeRegions`

**Solution**: Verify the policy is attached:

```bash
aws iam list-attached-role-policies --role-name capa-annotator-role
aws iam get-policy-version \
  --policy-arn arn:aws:iam::ACCOUNT_ID:policy/capa-annotator-policy \
  --version-id v1
```

### Token Not Mounted

**Error**: `AWS_WEB_IDENTITY_TOKEN_FILE not found`

**Solution**: This usually indicates that the pod identity webhook or EKS pod identity is not properly configured. When IRSA is working correctly:

1. Kubernetes automatically mounts the service account token at `/var/run/secrets/kubernetes.io/serviceaccount/token`
2. The pod identity webhook (or EKS) automatically injects the `AWS_WEB_IDENTITY_TOKEN_FILE` environment variable
3. No custom volume configuration is needed in the deployment

**To fix**:
- Verify the ServiceAccount has the correct IRSA annotation
- Check that the pod identity webhook is installed (for non-EKS clusters)
- For EKS: Ensure the cluster has IAM OIDC provider enabled
- Restart the deployment after fixing the ServiceAccount annotation

### Wrong Annotation Key

**Error**: IRSA not working, no environment variables set

**Solution**: The annotation key varies by OIDC provider:
- **EKS**: `eks.amazonaws.com/role-arn`
- **CAPA/Generic**: Check your OIDC provider documentation

Some CAPI implementations may use different annotation keys. Check your cluster's documentation.

## Security Best Practices

1. **Least Privilege**: The provided policy grants only the minimum required permissions
2. **Scope to Service Account**: Trust policy restricts AssumeRole to specific namespace and service account
3. **Audience Validation**: Condition checks `aud=sts.amazonaws.com` to prevent token reuse
4. **Regular Rotation**: IRSA tokens rotate automatically (default: 24 hours)
5. **No Long-Lived Credentials**: No static AWS access keys required

## Updating the IAM Role

### Using AWS CLI

```bash
# Update the policy
aws iam create-policy-version \
  --policy-arn arn:aws:iam::ACCOUNT_ID:policy/capa-annotator-policy \
  --policy-document file://policy.json \
  --set-as-default

# Update trust policy
aws iam update-assume-role-policy \
  --role-name capa-annotator-role \
  --policy-document file://trust-policy-configured.json
```

### Using Terraform

```bash
cd terraform/
terraform plan
terraform apply
```

## Deleting the IAM Role

### Using AWS CLI

```bash
# Detach policy
aws iam detach-role-policy \
  --role-name capa-annotator-role \
  --policy-arn arn:aws:iam::ACCOUNT_ID:policy/capa-annotator-policy

# Delete role
aws iam delete-role --role-name capa-annotator-role

# Delete policy
aws iam delete-policy --policy-arn arn:aws:iam::ACCOUNT_ID:policy/capa-annotator-policy
```

### Using Terraform

```bash
cd terraform/
terraform destroy
```

## Additional Resources

- [AWS IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
- [CAPI AWS Provider Documentation](https://cluster-api-aws.sigs.k8s.io/)
- [Kubernetes Service Account Token Projection](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#serviceaccount-token-volume-projection)
- [AWS STS AssumeRoleWithWebIdentity](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html)

## Support

If you encounter issues:

1. Check the [Troubleshooting](#troubleshooting) section above
2. Review controller logs: `kubectl logs -n capa-annotator-system -l app.kubernetes.io/name=capa-annotator`
3. Verify AWS permissions: `aws iam simulate-principal-policy --policy-source-arn ROLE_ARN --action-names ec2:DescribeInstanceTypes ec2:DescribeRegions`
4. Open an issue at [github.com/jhjaggars/capa-annotator](https://github.com/jhjaggars/capa-annotator/issues)
