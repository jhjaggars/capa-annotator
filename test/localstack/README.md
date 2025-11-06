# LocalStack Testing Setup

This directory contains the LocalStack configuration for testing CAPA Annotator against emulated AWS services.

## Overview

LocalStack provides a local AWS cloud stack that emulates AWS services, allowing us to test AWS API interactions without hitting real AWS infrastructure.

## Services Emulated

- **EC2**: For DescribeInstanceTypes and DescribeRegions API calls
- **STS**: For credential validation and AssumeRole operations
- **IAM**: For IRSA (IAM Roles for Service Accounts) testing

## Prerequisites

- Docker with Compose V2 (`docker compose` command)
- `curl` (for health checks)
- `awscli` (optional, for manual testing)

## Quick Start

### Start LocalStack

```bash
# From the repository root
make localstack-up

# Or directly with docker compose
cd test/localstack
docker compose up -d
```

### Check Status

```bash
# Check if LocalStack is healthy
curl http://localhost:4566/_localstack/health

# Check specific service status
curl http://localhost:4566/_localstack/health | jq '.services.ec2'
```

### Run Tests

```bash
# Run LocalStack-specific tests
make test-localstack

# Or manually
AWS_ENDPOINT_URL=http://localhost:4566 \
AWS_ACCESS_KEY_ID=test \
AWS_SECRET_ACCESS_KEY=test \
AWS_REGION=us-east-1 \
go test -v -tags=localstack -run TestLocalStack ./pkg/controller
```

### Stop LocalStack

```bash
# From the repository root
make localstack-down

# Or directly
cd test/localstack
docker compose down
```

## Configuration

### Container Socket

The docker-compose.yml mounts the container runtime socket:
- Docker: `/var/run/docker.sock` (default in CI)
- Podman (rootless): `/run/user/1000/podman/podman.sock`
- Custom UID: Update `1000` to your user ID (`id -u`) if using rootless podman

### Environment Variables

LocalStack uses these test credentials:
- `AWS_ACCESS_KEY_ID=test`
- `AWS_SECRET_ACCESS_KEY=test`
- `AWS_REGION=us-east-1`
- `AWS_ENDPOINT_URL=http://localhost:4566`

## Initialization Scripts

Scripts in `./init/` run when LocalStack becomes ready:

- `01-setup-ec2.sh`: Verifies EC2 services and sets up test IAM roles

## Testing Scenarios

LocalStack enables testing:

1. **Real AWS SDK behavior** - Uses actual AWS SDK against local endpoint
2. **Region resolution** - Tests DescribeRegions API calls
3. **Instance type queries** - Tests DescribeInstanceTypes API
4. **IRSA authentication** - Tests IAM role assumption flows
5. **Error scenarios** - Can simulate API failures and throttling
6. **Cache behavior** - Tests cache TTL with real timing

## Manual Testing

```bash
# Start LocalStack
make localstack-up

# Test EC2 API manually
aws ec2 describe-instance-types \
  --endpoint-url http://localhost:4566 \
  --region us-east-1 \
  --instance-types a1.2xlarge

# Test regions
aws ec2 describe-regions \
  --endpoint-url http://localhost:4566 \
  --region us-east-1

# Test IAM
aws iam list-roles \
  --endpoint-url http://localhost:4566 \
  --region us-east-1
```

## Troubleshooting

### LocalStack not starting

```bash
# Check Docker socket permissions
ls -la /var/run/docker.sock

# View LocalStack logs
docker logs capa-annotator-localstack

# Restart LocalStack
make localstack-down && make localstack-up
```

### Tests failing

```bash
# Verify LocalStack is healthy
curl http://localhost:4566/_localstack/health

# Check if AWS_ENDPOINT_URL is set
echo $AWS_ENDPOINT_URL

# Run tests with verbose logging
AWS_ENDPOINT_URL=http://localhost:4566 go test -v -tags=localstack -run TestLocalStack ./pkg/controller
```

### Port already in use

```bash
# Check what's using port 4566
lsof -i :4566

# Stop any existing LocalStack containers
docker stop capa-annotator-localstack
docker rm capa-annotator-localstack
```

## CI Integration

For CI environments (GitHub Actions, etc.):

```bash
export DOCKER_SOCK=/var/run/docker.sock
cd test/localstack
docker compose up -d
```

## References

- [LocalStack Documentation](https://docs.localstack.cloud/)
- [LocalStack EC2 Coverage](https://docs.localstack.cloud/user-guide/aws/ec2/)
- [AWS SDK Go v2](https://aws.github.io/aws-sdk-go-v2/)
