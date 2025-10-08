# Requirements Q&A: AWS IRSA Support

## Discovery Questions

### Q1: Will this feature be used primarily in EKS (Amazon Elastic Kubernetes Service) environments?
**Default if unknown:** Yes (IRSA is an EKS-native feature)

### Q2: Should IRSA authentication take priority over secret-based credentials when both are available?
**Default if unknown:** Yes (IRSA is more secure and modern approach)

### Q3: Will users need to continue supporting IAM instance profile authentication as a fallback?
**Default if unknown:** Yes (for backward compatibility with existing deployments)

### Q4: Should the controller log which authentication method is being used for troubleshooting?
**Default if unknown:** Yes (visibility is crucial for debugging authentication issues)

### Q5: Will this feature need to work with custom AWS STS endpoints (GovCloud, China regions)?
**Default if unknown:** No (standard AWS regions are most common)

## Expert Questions

### Q6: Should the controller validate that the projected service account token exists before attempting to use IRSA?
**Default if unknown:** No (AWS SDK will handle token validation and return appropriate errors)

### Q7: Should we add environment variable configuration for the token file path to allow non-standard mount locations?
**Default if unknown:** No (standard path `/var/run/secrets/eks.amazonaws.com/serviceaccount/token` follows AWS best practices)

### Q8: Should the controller fail fast if IRSA environment variables are present but incomplete?
**Default if unknown:** Yes (fail fast to help users identify configuration errors early)

### Q9: Should we preserve the temporary shared credentials file cleanup logic when IRSA is in use?
**Default if unknown:** Yes (cleanup logic should remain for secret-based auth, IRSA doesn't create temp files)

### Q10: Should IRSA support be feature-flagged to allow gradual rollout?
**Default if unknown:** No (IRSA is a well-established AWS feature, direct implementation is appropriate)

## Discovery Answers

### A1: No
This will only be used in an OpenShift environment running on AWS (not EKS)

### A2: Yes
IRSA is preferred

### A3: No

### A4: Yes

### A5: Yes

## Expert Answers

### A6: No

### A7: No

### A8: Yes

### A9: Yes

### A10: No

## Context Findings

### Similar Features Found
- **Custom endpoint resolution**: `pkg/client/client.go:411-444` - The codebase already supports custom AWS endpoints via `resolveEndpoints()` function, which reads from OpenShift Infrastructure object and creates custom endpoint resolvers
- **Credential chain pattern**: `pkg/client/client.go:356-402` - Current `newAWSSession()` function follows this pattern:
  1. Check if `secretName != ""` → load credentials from Kubernetes secret
  2. Otherwise → fall back to IAM instance profile (implicit)
- **AWS SDK credential precedence**: The AWS SDK v1 automatically checks environment variables including `AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE` before other credential sources

### Implementation Patterns
- **Session creation**: Uses `session.NewSessionWithOptions()` at `pkg/client/client.go:389`
- **Logging pattern**: Uses `klog.Info()` for informational messages about credential/config sources (see line 269 for region caching, 512 for custom CA bundle)
- **Secret handling**: `sharedCredentialsFileFromSecret()` at lines 457-482 creates temporary files for credentials
- **Cleanup pattern**: Temporary credential files are removed after session creation (line 396)
- **Controller integration**: MachineSet controller at `pkg/controller/controller.go:149-156` requires `CredentialsSecret` to be non-nil and passes it to `AwsClientBuilder`

### Technical Constraints
- **OpenShift-specific**: Codebase uses OpenShift API types (`configv1.Infrastructure`, `machinev1beta1.MachineSet`)
- **Custom endpoints required**: Must support custom AWS STS endpoints for GovCloud/China regions (per user requirement A5)
- **Secret validation**: Controller currently returns error if `providerConfig.CredentialsSecret == nil` at `pkg/controller/controller.go:149-151`
- **AWS SDK v1**: Uses `github.com/aws/aws-sdk-go` (v1), not v2 - IRSA support via environment variables is built-in
- **No existing IRSA code**: Grep search for IRSA-related patterns (`AWS_ROLE_ARN`, `WebIdentityCredentials`) found no existing implementation

### Key Integration Points
- **File to modify**: `pkg/client/client.go` - specifically the `newAWSSession()` function (lines 356-402)
- **Controller change needed**: `pkg/controller/controller.go:149-151` - must allow nil `CredentialsSecret` when IRSA is configured
- **Environment variables to check**: `AWS_ROLE_ARN`, `AWS_WEB_IDENTITY_TOKEN_FILE` (standard IRSA env vars)
- **Custom endpoints integration**: IRSA must work with existing `resolveEndpoints()` functionality for GovCloud/China support

## Key Insights

1. **OpenShift, not EKS**: Despite IRSA being an AWS EKS feature, this implementation is specifically for OpenShift on AWS, which has its own OIDC provider integration

2. **IRSA takes priority**: When both IRSA and secret-based credentials are configured, IRSA should be preferred as it's more secure and modern

3. **Fail fast on misconfiguration**: Rather than silently falling back, the controller should error immediately if IRSA variables are partially configured to help users debug setup issues

4. **AWS SDK handles the heavy lifting**: AWS SDK v1 automatically detects and uses IRSA environment variables - no need for explicit credential provider configuration

5. **Custom endpoints are critical**: Support for GovCloud and China regions requires IRSA to work seamlessly with the existing custom endpoint resolution logic

6. **Minimal controller changes**: The controller validation logic needs updating to allow nil CredentialsSecret when IRSA is configured, but the core reconciliation logic remains unchanged

7. **No IAM instance profile fallback needed**: The deployment will explicitly configure either IRSA or secrets - no need to support the IAM instance profile fallback that currently exists implicitly
