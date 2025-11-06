#!/bin/bash
# LocalStack initialization script for CAPA Annotator testing
# This script runs when LocalStack becomes ready

set -e

echo "=== Initializing LocalStack for CAPA Annotator Tests ==="

# Wait for LocalStack to be fully ready
echo "Waiting for LocalStack services to be ready..."
until curl -s http://localhost:4566/_localstack/health | grep -q '"ec2": "running"'; do
  echo "  EC2 service not ready yet, waiting..."
  sleep 1
done
echo "✓ LocalStack EC2 service is ready"

# Set AWS CLI to use LocalStack endpoint
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1
export AWS_ENDPOINT_URL=http://localhost:4566

echo ""
echo "=== Verifying EC2 Instance Types ==="
# LocalStack should provide instance types out of the box
# Verify we can query them
aws ec2 describe-instance-types \
  --instance-types a1.2xlarge m6g.4xlarge p2.16xlarge \
  --endpoint-url http://localhost:4566 \
  --region us-east-1 \
  --query 'InstanceTypes[*].[InstanceType,VCpuInfo.DefaultVCpus,MemoryInfo.SizeInMiB]' \
  --output table || echo "  Note: Instance types may use LocalStack defaults"

echo ""
echo "=== Verifying Regions ==="
# Test region listing
aws ec2 describe-regions \
  --endpoint-url http://localhost:4566 \
  --region us-east-1 \
  --query 'Regions[*].RegionName' \
  --output table || echo "  Note: Using default regions"

echo ""
echo "=== Setting up IAM for IRSA Testing ==="
# Create a test IAM role for IRSA authentication testing
aws iam create-role \
  --endpoint-url http://localhost:4566 \
  --region us-east-1 \
  --role-name test-irsa-role \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": {"Federated": "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE"},
      "Action": "sts:AssumeRoleWithWebIdentity"
    }]
  }' 2>/dev/null || echo "  Role may already exist"

# Attach EC2 read-only policy to the role
aws iam attach-role-policy \
  --endpoint-url http://localhost:4566 \
  --region us-east-1 \
  --role-name test-irsa-role \
  --policy-arn arn:aws:iam::aws:policy/AmazonEC2ReadOnlyAccess 2>/dev/null || true

echo ""
echo "=== LocalStack Initialization Complete ==="
echo "Services available at: http://localhost:4566"
echo "AWS Region: us-east-1"
echo "Test credentials: test/test"
