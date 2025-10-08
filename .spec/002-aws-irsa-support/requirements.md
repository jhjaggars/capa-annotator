# Requirements Specification: AWS IRSA Support

## Overview
Add support for AWS IAM Roles for Service Accounts (IRSA) authentication to the capa-annotator controller, enabling it to authenticate to AWS APIs using projected service account tokens instead of static credentials stored in Kubernetes secrets.

## Problem Statement
Currently, the capa-annotator controller requires AWS credentials to be stored in Kubernetes secrets, which introduces security and operational overhead:
- Static credentials must be rotated manually
- Secrets must be managed and distributed securely
- No support for modern OpenShift on AWS authentication patterns that use IRSA

IRSA provides a more secure authentication method where:
- No long-lived credentials need to be stored
- Tokens are automatically rotated
- Fine-grained IAM permissions can be assigned per service account
- Works with custom AWS endpoints (GovCloud, China regions)

## Solution Overview
Implement IRSA support by modifying the AWS session creation logic to detect and prioritize IRSA environment variables (`AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE`). The AWS SDK v1 will automatically handle the web identity token exchange with AWS STS. This will be the preferred authentication method, taking priority over secret-based credentials when both are present.

## Functional Requirements

### FR1: IRSA Environment Variable Detection
The controller MUST detect when IRSA environment variables are present:
- `AWS_ROLE_ARN`: The IAM role ARN to assume
- `AWS_WEB_IDENTITY_TOKEN_FILE`: Path to the projected service account token file

### FR2: Authentication Priority
When both IRSA and secret-based credentials are available, IRSA MUST take priority (as confirmed in A2).

### FR3: IRSA Validation
The controller MUST fail fast with a clear error message if IRSA environment variables are present but incomplete (as confirmed in A8):
- If `AWS_ROLE_ARN` is set but `AWS_WEB_IDENTITY_TOKEN_FILE` is missing → error
- If `AWS_WEB_IDENTITY_TOKEN_FILE` is set but `AWS_ROLE_ARN` is missing → error

### FR4: Authentication Method Logging
The controller MUST log which authentication method is being used (as confirmed in A4):
- Log when IRSA is detected and being used
- Provide visibility for troubleshooting authentication issues

### FR5: Custom Endpoint Support
IRSA MUST work with custom AWS STS endpoints for GovCloud and China regions (as confirmed in A5). This must integrate with the existing `resolveEndpoints()` functionality.

### FR6: Backward Compatibility for Secrets
The controller MUST continue to support secret-based authentication as a fallback when IRSA is not configured. However, IAM instance profile fallback is NOT required (as confirmed in A3).

### FR7: No Token File Validation
The controller MUST NOT validate that the projected service account token file exists before attempting to use IRSA (as confirmed in A6). Let the AWS SDK handle validation and errors.

### FR8: Standard Token Path Only
The controller MUST NOT support custom token file paths via additional environment variables (as confirmed in A7). Only the standard `AWS_WEB_IDENTITY_TOKEN_FILE` environment variable will be used.

### FR9: Credential File Cleanup
The existing temporary shared credentials file cleanup logic MUST be preserved and continue to work for secret-based authentication (as confirmed in A9). IRSA does not create temporary files.

### FR10: No Feature Flag
IRSA support MUST NOT be behind a feature flag (as confirmed in A10). It should be directly implemented as IRSA is a well-established AWS feature.

## Technical Requirements

### TR1: File Modifications
**Primary file**: `pkg/client/client.go`
- Modify the `newAWSSession()` function (lines 356-402)
- Add IRSA detection logic before secret-based credential loading
- Add appropriate logging using `klog.Info()`

**Secondary file**: `pkg/controller/controller.go`
- Modify the credential secret validation (lines 149-151)
- Allow `providerConfig.CredentialsSecret` to be nil when IRSA environment variables are present
- Update validation logic to check for IRSA OR secret (not just secret)

### TR2: Authentication Flow
The new authentication priority in `newAWSSession()` must be:
1. **First**: Check for IRSA environment variables (`AWS_ROLE_ARN` AND `AWS_WEB_IDENTITY_TOKEN_FILE`)
   - If both present → use IRSA (AWS SDK handles automatically)
   - If only one present → return error (fail fast)
2. **Second**: Check for `secretName != ""`
   - Load credentials from Kubernetes secret
3. **Third**: No authentication configured → return error

### TR3: AWS SDK Integration
- Continue using AWS SDK v1 (`github.com/aws/aws-sdk-go`)
- Rely on SDK's built-in IRSA support via environment variables
- No need to explicitly use `credentials.NewWebIdentityCredentials` - SDK handles this automatically when environment variables are set

### TR4: Custom Endpoint Integration
- IRSA must work with existing `resolveEndpoints()` function (lines 411-444)
- Custom STS endpoints must be applied to IRSA authentication flows
- No changes to endpoint resolution logic required

### TR5: Error Handling
- Return `machineapiapierrors.InvalidMachineConfiguration()` for incomplete IRSA configuration
- Provide clear error messages indicating which environment variable is missing
- Maintain existing error handling patterns for secret-based authentication

### TR6: OpenShift Compatibility
- Solution must work in OpenShift on AWS environments (not EKS, as confirmed in A1)
- Continue using OpenShift API types (`configv1.Infrastructure`, `machinev1beta1.MachineSet`)
- IRSA implementation must be compatible with OpenShift's OIDC provider configuration

## Implementation Hints

### Hint 1: Modify newAWSSession() in pkg/client/client.go
```go
func newAWSSession(...) (*session.Session, error) {
    sessionOptions := session.Options{
        Config: aws.Config{
            Region: aws.String(region),
        },
    }

    // Check for IRSA (highest priority)
    roleARN := os.Getenv("AWS_ROLE_ARN")
    tokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")

    if roleARN != "" || tokenFile != "" {
        // Validate both are present (fail fast)
        if roleARN == "" {
            return nil, fmt.Errorf("AWS_WEB_IDENTITY_TOKEN_FILE is set but AWS_ROLE_ARN is missing")
        }
        if tokenFile == "" {
            return nil, fmt.Errorf("AWS_ROLE_ARN is set but AWS_WEB_IDENTITY_TOKEN_FILE is missing")
        }
        klog.Infof("Using IRSA authentication (AWS_ROLE_ARN=%s)", roleARN)
        // AWS SDK will automatically use web identity provider - no code needed
    } else if secretName != "" {
        // Existing secret-based auth logic...
        klog.Info("Using secret-based authentication")
        // ... existing code ...
    } else {
        return nil, fmt.Errorf("no AWS credentials configured: neither IRSA nor secret-based auth available")
    }

    // Continue with existing endpoint resolution and session creation...
}
```

### Hint 2: Update Controller Validation in pkg/controller/controller.go
```go
func (r *Reconciler) reconcile(machineSet *machinev1beta1.MachineSet) (ctrl.Result, error) {
    // ... existing code ...

    // Determine secret name (may be nil for IRSA)
    secretName := ""
    if providerConfig.CredentialsSecret != nil {
        secretName = providerConfig.CredentialsSecret.Name
    }

    // Validate that either IRSA or secret is configured
    roleARN := os.Getenv("AWS_ROLE_ARN")
    tokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
    hasIRSA := roleARN != "" && tokenFile != ""

    if !hasIRSA && secretName == "" {
        return ctrl.Result{}, mapierrors.InvalidMachineConfiguration(
            "no AWS credentials configured for machineSet %s: neither IRSA nor credentialsSecret specified",
            machineSet.Name)
    }

    // Pass empty string for secretName if using IRSA
    awsClient, err := r.AwsClientBuilder(r.Client, secretName, machineSet.Namespace, ...)
    // ... rest of existing code ...
}
```

### Hint 3: Follow Existing Patterns
- Use `klog.Info()` for authentication method logging (see line 269, 512 for examples)
- Maintain temporary file cleanup pattern (line 395-397)
- Continue using `session.NewSessionWithOptions()` (line 389)
- Preserve custom endpoint resolution integration (line 380-382)

### Hint 4: Testing Considerations
- Unit tests should verify IRSA detection logic
- Unit tests should verify fail-fast behavior for incomplete IRSA config
- Integration tests should verify IRSA works with custom endpoints
- Mock AWS SDK session creation to test different authentication paths

## Acceptance Criteria

- [ ] IRSA environment variables (`AWS_ROLE_ARN`, `AWS_WEB_IDENTITY_TOKEN_FILE`) are detected in `newAWSSession()`
- [ ] IRSA authentication takes priority over secret-based credentials when both are present
- [ ] Controller fails fast with clear error if only one IRSA environment variable is set
- [ ] Controller logs "Using IRSA authentication" when IRSA is detected and used
- [ ] Controller logs "Using secret-based authentication" when secrets are used
- [ ] IRSA works with custom AWS STS endpoints (GovCloud, China regions)
- [ ] Secret-based authentication continues to work when IRSA is not configured
- [ ] Temporary credential file cleanup logic continues to work for secret-based auth
- [ ] Controller allows `CredentialsSecret` to be nil in MachineSet when IRSA is configured
- [ ] Controller returns clear error when neither IRSA nor secret credentials are available
- [ ] Unit tests cover IRSA detection logic and validation
- [ ] Integration tests verify IRSA works in OpenShift on AWS environments
- [ ] Documentation includes IRSA setup examples for OpenShift on AWS

## Assumptions

1. **OpenShift OIDC Provider**: Assume that OpenShift on AWS clusters have an OIDC provider configured for IRSA to work
2. **IAM Role Trust Policy**: Assume cluster administrators will configure the IAM role trust policy to trust the OpenShift OIDC provider
3. **Service Account Token Projection**: Assume the controller deployment will have the service account token projected into the pod at the standard path
4. **AWS SDK Behavior**: Assume AWS SDK v1 will automatically detect and use IRSA environment variables without explicit credential provider configuration
5. **No Token Rotation Logic**: Assume AWS SDK handles token rotation automatically - no custom rotation logic needed
6. **Deployment Configuration**: Assume deployment manifests (ServiceAccount, Deployment) will be created separately as part of deployment documentation
7. **Standard Token Path**: Assume the standard IRSA token path `/var/run/secrets/eks.amazonaws.com/serviceaccount/token` will be used (even though this is OpenShift, not EKS - the path is a convention)

## Out of Scope

The following items are explicitly NOT included in this specification:

1. **IAM instance profile fallback**: No support for falling back to IAM instance profiles (per A3)
2. **Custom token file paths**: No support for configuring custom token file paths via additional environment variables (per A7)
3. **Token file validation**: No pre-flight validation that token files exist (per A6)
4. **Feature flag**: No feature gate to enable/disable IRSA support (per A10)
5. **Deployment manifests**: Creating actual Kubernetes manifests (ServiceAccount, Deployment YAML) is out of scope - only code changes
6. **IAM role creation**: Creating or managing AWS IAM roles and trust policies
7. **OIDC provider setup**: Setting up OpenShift OIDC provider integration with AWS
8. **Migration tooling**: Automated migration from secret-based to IRSA authentication
9. **Credential rotation**: Custom token rotation logic (handled by AWS SDK)
10. **Alternative authentication methods**: Support for other AWS authentication methods (EC2 instance metadata, ECS task roles, etc.)
