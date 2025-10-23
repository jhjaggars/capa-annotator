# Build stage
FROM registry.access.redhat.com/ubi9/go-toolset:1.24 AS builder

# Build arguments for multi-arch support
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /opt/app-root/src

# Copy go.mod and go.sum
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies
RUN go mod download

# Copy the source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build the binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o capa-annotator ./cmd/controller

# Runtime stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

WORKDIR /

# Copy the binary from builder
COPY --from=builder /opt/app-root/src/capa-annotator .

ENTRYPOINT ["/capa-annotator"]
