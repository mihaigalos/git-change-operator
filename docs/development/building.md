# Building from Source

This guide covers how to build the Git Change Operator from source code, including development builds, production builds, and container images.

## Steps

### 1. Clone Repository

```bash
git clone https://github.com/mihaigalos/git-change-operator.git
cd git-change-operator
```

### 2. Build Binary

```bash
# Build for current platform
make build

# The binary will be in bin/manager
./bin/manager --help
```

### 3. Build Container Image

```bash
# Build Docker image
make docker-build

# Tag for your registry (optional)
docker tag git-change-operator:latest your-registry/git-change-operator:latest
```

## Detailed Build Process

### Understanding the Makefile

The project uses a comprehensive Makefile for all build operations:

```bash
# Show all available targets
make help
```

Key targets:
- `build` - Build the manager binary
- `docker-build` - Build container image
- `manifests` - Generate CRD and RBAC manifests
- `generate` - Generate Go code (deepcopy methods, etc.)
- `test` - Run unit tests
- `lint` - Run code linting

### Build Configuration

Build configuration is controlled by several variables:

```makefile
# Key variables in Makefile
VERSION ?= $(shell cat VERSION)
IMG ?= git-change-operator:$(VERSION)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
```

You can override these:

```bash
# Build for Linux AMD64
make build GOOS=linux GOARCH=amd64

# Build with custom image tag
make docker-build IMG=my-registry/git-change-operator:v1.0.0
```

## Development Builds

### Local Development

For local development and testing:

```bash
# Install dependencies
go mod download

# Generate code and manifests
make generate manifests

# Build and test
make build test

# Run locally (requires kubeconfig)
make run
```

### Multi-Architecture Builds

```bash
# Build for multiple architectures
make build-multi-arch

# This creates binaries for:
# - linux/amd64
# - linux/arm64
# - darwin/amd64
# - darwin/arm64
```

## Container Images

### Basic Docker Build

```bash
# Build image with default tag
make docker-build

# Verify image
docker images | grep git-change-operator
```

### Multi-Architecture Images

```bash
# Build for multiple platforms
make docker-buildx

# This creates images for:
# - linux/amd64
# - linux/arm64
```


## Next Steps

After building successfully:

1. [Run Tests](testing.md) - Validate your build
2. [Deploy Locally](../user-guide/installation.md) - Test deployment
3. [Contributing](contributing.md) - Contribute your changes

For production deployments, see our [Installation Guide](../user-guide/installation.md).