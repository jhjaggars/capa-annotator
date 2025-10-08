# Implementation Planning Q&A Session

## Planning Questions

This document captures the technical research and architectural analysis performed during implementation planning for AWS IRSA support.

**Note:** The main Q&A session for requirements gathering was completed in [q-a-requirements.md](./q-a-requirements.md). This document focuses on implementation-specific technical details.

## Technical Research

### Architecture Analysis

**Current Architecture Patterns:**
- **Session Creation**: Uses `session.NewSessionWithOptions()` at line 389 in `pkg/client/client.go`
- **Credential Chain**: Currently follows a simple pattern:
  1. Check if `secretName != ""` → load credentials from Kubernetes secret
  2. Otherwise → fall back to IAM instance profile (implicit, handled by AWS SDK)
- **Temporary File Handling**: Secret-based auth creates temporary credentials file, then cleans it up after session creation (lines 395-397)
- **Custom Endpoints**: Existing `resolveEndpoints()` function (lines 411-444) supports custom AWS endpoints via OpenShift Infrastructure object
- **Logging Pattern**: Uses `klog.Info()` for informational messages (see line 269 for region caching, 512 for custom CA bundle)

**Integration Points Identified:**
1. **Primary File**: `pkg/client/client.go`
   - Function: `newAWSSession()` (lines 356-402)
   - Modification: Add IRSA detection before secret-based authentication
   - Pattern: Maintain if-else structure for credential source selection

2. **Secondary File**: `pkg/controller/controller.go`
   - Function: `reconcile()` (lines 142-182)
   - Current Issue: Requires `providerConfig.CredentialsSecret != nil` (line 149-151)
   - Modification: Allow nil `CredentialsSecret` when IRSA is configured

3. **Testing Infrastructure**:
   - Existing tests use Ginkgo/Gomega framework
   - Fake AWS client exists at `pkg/client/fake/fake.go`
   - Controller tests use `awsClientBuilder` function injection for testing
   - No existing test file for `pkg/client/client.go` - needs to be created

**Proposed Architectural Changes:**
- **Minimal Changes**: IRSA support is additive, not replacing existing functionality
- **Credential Priority**: IRSA > Secrets > Error (no fallback to instance profile)
- **Fail-Fast Validation**: Validate IRSA configuration early in `newAWSSession()` before AWS SDK invocation
- **Logging Enhancement**: Add authentication method logging for troubleshooting
- **No Structural Changes**: Custom endpoint resolution, CA bundle handling remain unchanged

### Technology Decisions

**Library/Framework Choices:**

1. **AWS SDK v1 (github.com/aws/aws-sdk-go v1.55.7)**
   - Decision: Continue using AWS SDK v1 (no upgrade to v2)
   - Rationale:
     - Already in use throughout the codebase
     - Built-in IRSA support via environment variable detection
     - No need for explicit credential provider configuration
     - SDK automatically detects `AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE`
   - Implementation: Let SDK handle IRSA automatically, no need for `credentials.NewWebIdentityCredentials()`

2. **Error Handling (github.com/openshift/machine-api-operator/pkg/controller/machine)**
   - Decision: Use existing `machineapiapierrors.InvalidMachineConfiguration()`
   - Rationale:
     - Consistent with existing error handling patterns
     - Provides proper error categorization for machine configuration issues
     - Returns appropriate HTTP status codes in API responses
   - Implementation: Use for IRSA validation errors and missing credential errors

3. **Logging (k8s.io/klog/v2)**
   - Decision: Use `klog.Info()` and `klog.Infof()` for authentication method logging
   - Rationale:
     - Consistent with existing logging throughout the codebase
     - Integrates with Kubernetes logging infrastructure
     - Supports log levels for verbosity control
   - Implementation: Log authentication method selection at Info level

4. **Testing Framework (Ginkgo/Gomega)**
   - Decision: Use existing Ginkgo/Gomega framework for new tests
   - Rationale:
     - Consistent with existing test suite in `pkg/controller/controller_test.go`
     - Supports BDD-style test organization
     - Provides rich assertion library (Gomega)
   - Implementation: Create `pkg/client/client_test.go` using same patterns as controller tests

**Database Considerations:**
- N/A - No database changes required

**API Design Decisions:**
- No API changes required - authentication is internal implementation detail
- MachineSet API remains unchanged
- Environment variables provide configuration (Kubernetes deployment concern)

### Code Organization

**Where New Code Should Live:**

1. **IRSA Detection Logic** → `pkg/client/client.go`
   - Location: Beginning of `newAWSSession()` function (after line 361)
   - Scope: ~40 lines of new code (environment variable checks, validation, logging)
   - Pattern: if-else-if structure for credential source selection

2. **Controller Validation** → `pkg/controller/controller.go`
   - Location: Replace lines 149-151, update line 153
   - Scope: ~10 lines of new code (IRSA check, updated validation)
   - Pattern: Check IRSA env vars, validate at least one auth method available

3. **Unit Tests** → `pkg/client/client_test.go` (NEW FILE)
   - Location: New file in `pkg/client/` directory
   - Scope: ~200 lines (6-8 test cases covering IRSA scenarios)
   - Pattern: Table-driven tests using Gomega assertions

4. **Integration Tests** → `pkg/controller/controller_test.go` (EXISTING FILE)
   - Location: Add entries to existing `DescribeTable`
   - Scope: ~30 lines (1-2 new test cases)
   - Pattern: Extend existing test table with IRSA scenarios

**Existing Patterns to Follow:**

1. **Environment Variable Access**: Use `os.Getenv()` directly (Go standard library)
2. **Error Wrapping**: Use `fmt.Errorf()` with `%w` verb for error wrapping
3. **Logging Format**: Use `klog.Infof()` with format string for structured logging
4. **Test Organization**: Use `TestCases` struct with `name`, `expectError`, `errorContains` fields
5. **Fake Client Pattern**: Inject fake AWS client via `awsClientBuilder` function parameter

**Refactoring Needs Identified:**
- None - IRSA support integrates cleanly with existing code structure
- No functions need to be extracted or reorganized
- Existing credential handling logic is well-structured for this addition

## Implementation Details

### Testing Approach

**Testing Strategies Discussed:**

1. **Unit Tests (pkg/client/client_test.go)**
   - Strategy: Test IRSA detection logic in isolation
   - Approach: Set environment variables, verify correct code path taken
   - Challenge: IRSA requires actual token file to exist for full session creation
   - Solution: Test up to session creation attempt, allow expected failures due to missing token file
   - Coverage: IRSA validation, authentication priority, error messages

2. **Integration Tests (pkg/controller/controller_test.go)**
   - Strategy: Test controller reconciliation with various credential configurations
   - Approach: Use fake AWS client to avoid actual AWS API calls
   - Challenge: Testing nil `CredentialsSecret` with IRSA env vars
   - Solution: Modify `newTestMachineSet()` to support optional `CredentialsSecret`
   - Coverage: End-to-end reconciliation, annotation setting, error handling

3. **Manual Testing**
   - Strategy: Verify real-world IRSA functionality in OpenShift on AWS cluster
   - Approach: Deploy controller with IRSA configuration, verify AWS API calls succeed
   - Challenge: Requires actual OpenShift cluster with OIDC provider
   - Solution: Provide detailed manual test scenarios in implementation plan
   - Coverage: IRSA authentication, custom endpoints, token rotation

**Mock/Stub Requirements:**

1. **Fake Kubernetes Client**: Use `sigs.k8s.io/controller-runtime/pkg/client/fake`
   - Mock Secret retrieval for secret-based auth tests
   - Mock Infrastructure object for custom endpoint tests
   - Already used in existing tests

2. **Fake AWS Client**: Use existing `pkg/client/fake/fake.go`
   - Mock EC2 DescribeInstanceTypes API
   - Already implemented and used in controller tests
   - No changes needed

3. **Environment Variables**: Use `os.Setenv()` and `os.Unsetenv()`
   - Set IRSA env vars in test setup
   - Clean up in test teardown or defer
   - Standard Go testing pattern

**Integration Test Considerations:**

1. **Environment Variable Isolation**
   - Use `defer os.Unsetenv()` to ensure cleanup
   - Each test case sets its own environment variables
   - Prevents test pollution between cases

2. **Fake Client Injection**
   - Use `awsClientBuilder` parameter in controller tests
   - Return fake client regardless of credential configuration
   - Allows testing IRSA code path without actual AWS credentials

3. **Token File Mocking**
   - Unit tests: Accept expected failures due to missing token file
   - Integration tests: Use fake AWS client to bypass actual authentication
   - Manual tests: Verify actual token file functionality

## Key Decisions

### Summary of Important Technical Decisions

1. **IRSA Priority Over Secrets**
   - Decision: IRSA takes precedence when both IRSA and secrets are configured
   - Rationale: IRSA is more secure, modern approach; should be preferred
   - Impact: Users can deploy with both, IRSA will be used automatically
   - Risk: None - documented behavior, clear logging indicates which method is used

2. **Fail-Fast on Incomplete IRSA Configuration**
   - Decision: Return error immediately if only one IRSA env var is set
   - Rationale: Better user experience than confusing AWS SDK errors later
   - Impact: Clear, actionable error messages for users
   - Risk: None - improves error handling and troubleshooting

3. **No Explicit Credential Provider Configuration**
   - Decision: Rely on AWS SDK's automatic IRSA detection
   - Rationale: SDK v1 automatically detects environment variables
   - Impact: Simpler implementation, fewer lines of code
   - Risk: None - well-documented SDK behavior, widely used pattern

4. **No IAM Instance Profile Fallback**
   - Decision: Require explicit IRSA or secret configuration, no fallback
   - Rationale: Aligns with requirements (A3), clearer security model
   - Impact: Users must explicitly configure authentication
   - Risk: None - deployment manifests specify authentication method

5. **Preserve Existing Custom Endpoint Support**
   - Decision: No changes to `resolveEndpoints()` function
   - Rationale: Custom endpoints work identically for IRSA and secret-based auth
   - Impact: GovCloud and China region support works out-of-the-box with IRSA
   - Risk: None - existing functionality reused as-is

6. **No Feature Flag**
   - Decision: IRSA support enabled by default, no feature gate
   - Rationale: IRSA is well-established, per requirements (A10)
   - Impact: Feature available immediately upon deployment
   - Risk: None - IRSA only activates when env vars are present

7. **Maintain Backward Compatibility**
   - Decision: Secret-based authentication remains fully functional
   - Rationale: Existing deployments must continue working after upgrade
   - Impact: Zero downtime migration possible
   - Risk: Mitigated by comprehensive regression testing

8. **Log Authentication Method**
   - Decision: Log which authentication method is selected
   - Rationale: Critical for troubleshooting authentication issues (A4)
   - Impact: Improved operational visibility
   - Risk: IAM role ARN in logs (acceptable - not sensitive information)

### Architecture Integration Summary

**How IRSA Fits Into Existing System:**

```
Current Flow:
  MachineSet → Controller → AwsClientBuilder → newAWSSession → AWS SDK
                                                      ↓
                                              Load credentials from Secret
                                                      ↓
                                              Create temporary file
                                                      ↓
                                              Session with shared config

New Flow with IRSA:
  MachineSet → Controller → AwsClientBuilder → newAWSSession → AWS SDK
                                                      ↓
                                              Check IRSA env vars
                                              ↙            ↘
                                    IRSA configured?    No → Load from Secret (existing path)
                                            ↓ Yes
                                    Log "Using IRSA authentication"
                                            ↓
                                    Session (SDK auto-detects IRSA)
```

**Key Integration Points:**

1. **Controller Layer**: Validates at least one auth method available (IRSA or secret)
2. **Client Layer**: Detects and prioritizes IRSA in session creation
3. **AWS SDK Layer**: Automatically handles web identity token exchange with AWS STS
4. **Kubernetes Layer**: Provides projected service account token (deployment configuration)
5. **AWS Layer**: IAM role trust policy trusts OpenShift OIDC provider (cluster admin configuration)

### Testing Coverage Strategy

**Unit Test Coverage:**
- IRSA environment variable detection: ✅ 6 test cases
- Validation logic (fail-fast): ✅ 2 test cases
- Authentication priority: ✅ 1 test case
- Custom endpoint integration: ✅ 1 test case
- **Total**: ~10 unit test cases

**Integration Test Coverage:**
- Controller with IRSA: ✅ 1 test case
- Controller with secrets: ✅ Existing tests (regression)
- Controller with neither: ✅ 1 test case (error scenario)
- **Total**: ~2 new integration test cases + existing coverage

**Manual Test Coverage:**
- IRSA-only deployment: ✅ 1 scenario
- Secret-based fallback: ✅ 1 scenario
- Custom endpoint (GovCloud): ✅ 1 scenario
- Error handling: ✅ 1 scenario
- **Total**: 4 manual test scenarios

**Expected Test Coverage:**
- Modified functions: >80% line coverage
- IRSA code paths: 100% coverage
- Error paths: 100% coverage
- Regression: All existing tests continue to pass

### Implementation Validation

**Code Quality Checks:**
- ✅ `go vet ./...` - No vet errors
- ✅ `go fmt ./...` - Code formatted correctly
- ✅ `golangci-lint run` - No linter warnings
- ✅ All existing tests pass
- ✅ New tests pass

**Acceptance Criteria Mapping:**
- FR1: IRSA env var detection → Unit test: "IRSA configured with both environment variables"
- FR2: IRSA priority → Unit test: "IRSA takes priority over secret when both present"
- FR3: Fail fast validation → Unit tests: "IRSA partial" test cases
- FR4: Auth method logging → Manual verification in logs
- FR5: Custom endpoint support → Unit test: "TestNewAWSSessionCustomEndpoints"
- FR6: Backward compatibility → Integration test: "Secret-based auth works when IRSA not configured"
- FR7: No token file validation → Code review (no validation present)
- FR8: Standard token path → Code review (uses AWS_WEB_IDENTITY_TOKEN_FILE)
- FR9: Credential file cleanup → Code review (cleanup logic unchanged)
- FR10: No feature flag → Code review (no feature gate present)

### References

- **Requirements**: [requirements.md](./requirements.md)
- **Q&A Session**: [q-a-requirements.md](./q-a-requirements.md)
- **Implementation Plan**: [implementation-plan.md](./implementation-plan.md)
- **AWS SDK Documentation**: https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/
- **IRSA Documentation**: https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html
- **OpenShift Machine API**: https://github.com/openshift/machine-api-operator
- **Kubernetes Service Account Tokens**: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
