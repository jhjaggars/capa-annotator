.PHONY: build test test-unit test-coverage test-race clean fmt vet lint image tidy

# Binary name
BINARY_NAME=capa-annotator
BIN_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod
GOCLEAN=$(GOCMD) clean

# Docker parameters
IMAGE_REGISTRY?=quay.io
IMAGE_NAME?=$(IMAGE_REGISTRY)/jhjaggars/$(BINARY_NAME)
IMAGE_TAG?=latest

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

# Build container image
image:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Push container image
push: image
	docker push $(IMAGE_NAME):$(IMAGE_TAG)
