# Testing Strategy

This document describes the testing strategy for CAPA Annotator, including the test levels, CI configuration, and how dependency updates are validated.

## Test Levels

CAPA Annotator uses a multi-layered testing approach to ensure code quality and catch issues at different levels.

### 1. Unit Tests

**Purpose:** Fast, isolated tests with no external dependencies.

**Command:**
```bash
make test-unit
```

**Details:**
- Uses Go's `-short` flag to skip integration tests
- Uses fake AWS client (`pkg/client/fake/fake.go`)
- No external dependencies (no Kubernetes, no AWS)
- Fast execution (~1-2 seconds)
- Best for TDD and rapid iteration
- Located throughout the codebase alongside production code

**What they test:**
- Business logic in isolation
- Utility functions
- Error handling
- Edge cases

### 2. Integration Tests

**Purpose:** Test controller reconciliation loop with real Kubernetes API.

**Command:**
```bash
make test-integration
```

**Details:**
- Requires envtest (setup-envtest@release-0.20)
- Envtest automatically downloads Kubernetes binaries (etcd, kube-apiserver) for version 1.33.0
- Uses fake AWS client but real Kubernetes API server
- Medium execution time (~10-30 seconds)
- Tests full controller reconciliation loop with CAPI resources
- Located in `pkg/controller/controller_test.go`

**What they test:**
- Controller reconciliation logic
- Kubernetes resource interactions (MachineDeployment, AWSMachineTemplate, AWSCluster)
- Annotation updates
- Error handling in the reconciliation loop
- Leader election (disabled in tests)

### 3. LocalStack Integration Tests

**Purpose:** Test AWS SDK integration with emulated AWS services.

**Command:**
```bash
make test-localstack
```

**Details:**
- Requires Podman and podman-compose
- Uses **real AWS SDK client** pointed at LocalStack
- Emulates AWS EC2, IAM, and STS services
- Tests actual AWS API behavior
- Slower execution (~30-60 seconds including LocalStack startup)
- Tests IRSA (IAM Roles for Service Accounts) authentication
- Located in `pkg/controller/controller_localstack_test.go` (requires `-tags=localstack`)

**What they test:**
- Real AWS SDK behavior against emulated AWS
- IRSA authentication flows (AWS_ROLE_ARN, AWS_WEB_IDENTITY_TOKEN_FILE)
- EC2 API integration (DescribeInstanceTypes, DescribeRegions)
- Region validation and caching
- Instance type caching

**LocalStack Commands:**
```bash
# Start LocalStack
make localstack-up

# Check health
make localstack-health

# View logs
make localstack-logs

# Stop LocalStack
make localstack-down
```

### 4. Race Detector

**Purpose:** Detect data races and concurrency issues.

**Command:**
```bash
make test-race
```

**Details:**
- Runs unit tests with Go's race detector (`-race` flag)
- Uses `-short` flag to focus on fast unit tests
- Catches concurrency bugs in cache implementations
- Critical for validating thread-safe code (RegionCache, InstanceTypesCache)

### 5. Code Quality Checks

**Commands:**
```bash
# Format code
make fmt

# Run go vet
make vet

# Run linter (if golangci-lint installed)
make lint

# Tidy dependencies
make tidy
```

### 6. Coverage Report

**Command:**
```bash
make test-coverage
```

**Details:**
- Generates `coverage.out` file
- Opens `coverage.html` in browser
- Shows line-by-line coverage for all packages

## CI/CD Configuration

### GitHub Actions Workflows

All CI workflows are located in `.github/workflows/`.

#### 1. test.yml - Main Test Workflow

**Triggers:**
- Push to `main` or `initial-implementation` branches
- Pull requests to `main`
- Pull request target with "safe-to-test" label

**Jobs:**

| Job Name | Purpose | What It Runs |
|----------|---------|--------------|
| **Run Tests** | Unit tests + coverage | `make test-unit`, `make test-coverage`, uploads to Codecov |
| **Integration Tests** | Integration tests | `make test-integration` |
| **Lint** | Code quality | `make vet`, golangci-lint |
| **Build** | Compilation | `make build`, uploads binary artifact |
| **Dependency Verification** | Dependency integrity | `go mod verify`, `go mod tidy` check |
| **Race Detector** | Concurrency safety | `go test -race -short ./...` |
| **LocalStack Integration Tests** | AWS SDK integration | Starts LocalStack, runs `go test -tags=localstack` |

**All 7 jobs are required** for pull requests to be merged (see Required Checks below).

#### 2. localstack-tests.yml - Standalone LocalStack Tests

**Triggers:**
- Push to `main`
- Pull requests to `main`
- Manual workflow dispatch

**Purpose:**
- Standalone LocalStack testing workflow (for manual testing and debugging)
- Note: LocalStack tests are now also integrated into test.yml as a required check

#### 3. auto-merge-mintmaker.yml - Automated Dependency PR Merging

**Triggers:**
- Pull request target events
- Workflow run completion (Tests workflow)

**Logic:**
- Only processes MintMaker bot PRs (`app/red-hat-konflux-kflux-prd-rh03`)
- Distinguishes MAJOR vs MINOR/PATCH updates
- Auto-merges MINOR/PATCH updates + digest/hash updates
- **Blocks MAJOR version updates** (requires manual review)
- Respects "no-automerge" label
- Waits for all required checks to pass

**Required Checks (7 total):**
1. Run Tests
2. Integration Tests
3. Lint
4. Build
5. LocalStack Integration Tests
6. Dependency Verification
7. Race Detector

**Smart Version Detection:**
- Parses semantic versions from PR titles
- Detects digest/hash updates (always safe to merge)
- Detects major version changes (requires manual review)
- Falls back to manual review if version parsing fails

**Example PR Titles:**
- ✅ Auto-merge: `chore(deps): update module sigs.k8s.io/cluster-api to v1.10.3` (patch)
- ✅ Auto-merge: `chore(deps): update konflux digest to abc123` (digest)
- ⚠️ Manual review: `fix(deps): update module sigs.k8s.io/cluster-api to v2.0.0` (major)

#### 4. build-image.yml - Container Build

**Triggers:**
- Version tags (`v*`)
- Manual workflow dispatch

**Purpose:**
- Multi-arch container builds (amd64, arm64)
- Pushes to ghcr.io

### Konflux/Tekton Pipelines

Located in `.tekton/`, these provide extensive security and quality checks:

**Security Checks:**
- Clair scan (vulnerability scanning)
- Snyk SAST (static application security testing)
- ClamAV scan (malware detection)
- Coverity static analysis
- RPMs signature scan
- Ecosystem cert preflight checks

**Quality Checks:**
- Build container with buildah
- Deprecated base image check
- Shell script validation
- Unicode character validation (suspicious characters)

**Pipelines:**
- `capa-annotator-eda5d-pull-request.yaml` - Runs on PRs
- `capa-annotator-eda5d-push.yaml` - Runs on pushes to main

## Testing Matrix

| Test Type | Dependencies | Execution Time | Frequency | CI Required |
|-----------|--------------|----------------|-----------|-------------|
| Unit Tests | None | ~1-2s | Every commit | ✅ Yes |
| Integration Tests | envtest (K8s 1.33.0) | ~10-30s | Every commit | ✅ Yes |
| LocalStack Tests | Podman, LocalStack | ~30-60s | Every commit | ✅ Yes |
| Race Detector | None | ~2-5s | Every commit | ✅ Yes |
| Dependency Verification | None | ~5-10s | Every commit | ✅ Yes |
| Lint | golangci-lint | ~10-20s | Every commit | ✅ Yes |
| Build | None | ~5-10s | Every commit | ✅ Yes |
| Security Scans (Konflux) | Tekton, Snyk, Clair | ~5-10m | Every PR/push | ⚠️ Separate |

## Dependency Update Safety

### How Dependency Updates Are Validated

1. **MintMaker bot creates PR** with dependency updates
2. **GitHub Actions runs 7 required checks:**
   - Unit tests (catch breaking API changes)
   - Integration tests (catch Kubernetes API issues)
   - LocalStack tests (catch AWS SDK breaking changes)
   - Race detector (catch new concurrency bugs)
   - Dependency verification (validate checksums, ensure tidiness)
   - Lint (catch code quality regressions)
   - Build (catch compilation errors)
3. **Auto-merge workflow evaluates PR:**
   - Checks if author is MintMaker bot
   - Checks for "no-automerge" label
   - Detects version change type (MAJOR vs MINOR/PATCH)
   - Waits for all 7 required checks to pass
4. **Decision:**
   - ✅ **Auto-merge:** MINOR/PATCH updates + digest updates (if all checks pass)
   - ⚠️ **Manual review:** MAJOR version updates
   - ❌ **Block:** Any check failures

### Manual Override

To prevent auto-merge for a specific PR:
```bash
gh pr edit <PR_NUMBER> --add-label no-automerge
```

### What CI Catches

✅ **CI will catch:**
- Breaking API changes (unit/integration tests)
- Compilation errors (build job)
- AWS SDK behavior changes (LocalStack tests)
- Concurrency regressions (race detector)
- Kubernetes API incompatibilities (integration tests)
- Dependency corruption (go mod verify)
- Untidy dependencies (go mod tidy check)

⚠️ **CI may NOT catch:**
- Runtime behavior changes in production AWS (LocalStack is an emulation)
- Performance regressions (no performance tests yet)
- Issues specific to real Kubernetes clusters (envtest is a simulation)
- License changes in dependencies (no license scanning in GitHub Actions)

## Running Tests Locally

### Quick Development Loop

```bash
# Run unit tests (fastest)
make test-unit

# Run with watch mode (requires entr or similar)
find . -name '*.go' | entr -c make test-unit
```

### Pre-commit Checks

```bash
# Run all local checks before committing
make test-unit && make test-integration && make vet && make lint && make build
```

### Full Test Suite

```bash
# Run all tests (requires LocalStack)
make test
make test-localstack
make test-race
```

### Debugging Test Failures

**Unit test failures:**
```bash
# Run specific test
go test -v -short -run TestSpecificTest ./pkg/controller

# Run with verbose output
go test -v -short ./pkg/controller
```

**Integration test failures:**
```bash
# Run specific integration test
go test -v -run TestReconciler ./pkg/controller

# Debug envtest issues
KUBEBUILDER_ATTACH_CONTROL_PLANE_OUTPUT=true go test -v -run TestReconciler ./pkg/controller
```

**LocalStack test failures:**
```bash
# Start LocalStack and keep it running
make localstack-up

# Run tests manually
AWS_ENDPOINT_URL=http://localhost:4566 \
AWS_ACCESS_KEY_ID=test \
AWS_SECRET_ACCESS_KEY=test \
AWS_REGION=us-east-1 \
go test -v -tags=localstack -run TestLocalStack ./pkg/controller

# Check LocalStack logs
make localstack-logs

# Stop LocalStack
make localstack-down
```

## Best Practices

### Writing Tests

1. **Prefer unit tests** for business logic
   - Fast feedback loop
   - Easy to debug
   - No external dependencies

2. **Use integration tests** for controller behavior
   - Test full reconciliation loop
   - Validate Kubernetes resource interactions
   - Use realistic resource structures

3. **Use LocalStack tests** for AWS SDK integration
   - Test actual SDK calls
   - Validate authentication flows (IRSA)
   - Test region/instance type caching

4. **Always run race detector** for concurrent code
   - Critical for cache implementations
   - Run before committing changes to shared state

### Before Merging

1. Run full test suite locally: `make test && make test-localstack && make test-race`
2. Verify dependencies are tidy: `make tidy`
3. Check formatting: `make fmt`
4. Run linter: `make lint`
5. Ensure all CI checks pass (GitHub Actions will block merge if not)

### Reviewing Dependency PRs

**MINOR/PATCH updates:**
- Review CI check results
- Look for any test failures
- Check for breaking changes in release notes (even for minor versions!)
- Auto-merge will handle if all checks pass

**MAJOR updates:**
- Manually review release notes and changelogs
- Look for breaking changes, deprecations, migrations
- Test locally with `make test && make test-localstack`
- Consider updating code to use new APIs
- Manually approve and merge after validation

**Digest/hash updates:**
- Generally safe (same version, different build)
- Auto-merge will handle if all checks pass

## Test Coverage Goals

Current coverage targets:
- **Overall:** >70%
- **Controller:** >80%
- **Utils:** >90%
- **Client:** >75%

View coverage report:
```bash
make test-coverage
# Opens coverage.html in browser
```

## Troubleshooting

### "envtest binaries not found"

```bash
# Manually install envtest
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.20
setup-envtest use 1.33.0
```

### "LocalStack not starting"

```bash
# Check Docker/Podman is running
podman --version
podman ps

# Check LocalStack compose file
cat test/localstack/docker-compose.yml

# Try starting manually
cd test/localstack && podman compose up
```

### "Race detector finds issues"

Race conditions are serious bugs. Fix them before merging:

```bash
# Run race detector with verbose output
go test -race -v ./...

# Common fixes:
# 1. Use sync.Mutex for shared state
# 2. Use channels for communication
# 3. Use atomic operations for simple counters
```

## References

- [Testing in Go](https://go.dev/doc/tutorial/add-a-test)
- [Kubernetes Controller Testing](https://book.kubebuilder.io/cronjob-tutorial/writing-tests)
- [LocalStack Documentation](https://docs.localstack.cloud/)
- [Go Race Detector](https://go.dev/doc/articles/race_detector)
