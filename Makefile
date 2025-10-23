.PHONY: build test test-unit test-integration test-coverage test-race clean fmt vet lint image image-multiarch push push-multiarch tidy

# Binary name
BINARY_NAME=capa-annotator
BIN_DIR=bin

# envtest/kubebuilder configuration
ENVTEST_K8S_VERSION = 1.33.0
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
# Use release-0.20 to match our controller-runtime v0.20.4 dependency
ENVTEST = go run sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.20

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod
GOCLEAN=$(GOCMD) clean

# Container image parameters
IMAGE_REGISTRY?=ghcr.io
IMAGE_NAME?=$(IMAGE_REGISTRY)/jhjaggars/$(BINARY_NAME)
IMAGE_TAG?=latest
PLATFORMS?=linux/amd64,linux/arm64

all: build

# Build the binary
build:
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/controller

# Run tests
test:
	$(GOTEST) -v ./...

# Run unit tests only (skip integration tests that need kubebuilder)
test-unit:
	$(GOTEST) -v ./pkg/... -short

# Run integration tests (uses setup-envtest to download kubebuilder assets)
test-integration:
	@export KUBEBUILDER_ASSETS=$$($(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path --bin-dir $(PROJECT_DIR)/bin) && \
		$(GOTEST) -v ./pkg/controller -run TestReconciler -timeout 5m -race

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -short -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detector
test-race:
	$(GOTEST) -v -race ./...

# Run go vet
vet:
	$(GOVET) ./...

# Format code
fmt:
	$(GOFMT) ./...

# Run golangci-lint (if installed)
lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping lint"

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BIN_DIR)

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Build container image (single architecture)
image:
	podman build -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Push container image (single architecture)
push: image
	podman push $(IMAGE_NAME):$(IMAGE_TAG)

# Build multi-architecture container image
image-multiarch:
	podman build --platform=$(PLATFORMS) --manifest $(IMAGE_NAME):$(IMAGE_TAG) .

# Push multi-architecture container image
push-multiarch: image-multiarch
	podman manifest push --all $(IMAGE_NAME):$(IMAGE_TAG) docker://$(IMAGE_NAME):$(IMAGE_TAG)
