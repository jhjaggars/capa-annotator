# Implementation Planning Q&A - MachineSet Annotator

## Project Context
This implementation plan focuses on porting MachineSet annotation functionality from `machine-api-provider-aws` into a standalone controller. The requirements are well-defined in `requirements.md` and the source code exists at `/Users/jjaggars/code/machine-api-provider-aws/`.

## Architecture Analysis Summary
**Source Project Structure:**
- Main controller: `pkg/actuators/machineset/controller.go` (183 lines)
- Instance type cache: `pkg/actuators/machineset/ec2_instance_types.go` (216 lines)
- AWS client wrapper: `pkg/client/client.go` (517 lines)
- Provider spec utility: `pkg/actuators/machine/utils.go` (ProviderSpecFromRawExtension function)
- Main entry point: `cmd/manager/main.go` (266 lines)

**Key Dependencies:**
- Go module: github.com/openshift/machine-api-provider-aws
- AWS SDK: github.com/aws/aws-sdk-go v1.55.7
- OpenShift APIs: github.com/openshift/api (machine v1beta1, config v1)
- Controller runtime: sigs.k8s.io/controller-runtime v0.21.0
- Kubernetes client-go: k8s.io/client-go v0.33.3

## Planning Questions

### Q1: Module Path and Project Naming
The new standalone controller needs a Go module path. Based on your username and project location, what should the module path be?

**Smart Default:** `github.com/jjaggars/capi-annotator`

**Context:** This will be used in:
- `go.mod` module declaration
- Import statements throughout the codebase
- Build and image tags

**Answer:** `github.com/jhjaggars/capi-annotator`
