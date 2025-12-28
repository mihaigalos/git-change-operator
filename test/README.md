# Testing Guide

This project includes comprehensive tests that can be run without needing a real Kubernetes cluster.

**🚀 Quick Start:** Run `just test-integration` or `just test-all` - all required tools and binaries will be automatically downloaded!

## Test Types

### 1. Unit Tests
Pure unit tests that use fake clients and don't require external dependencies:

```bash
just test-unit
# or directly
go test -v ./test/*unit_test.go
```

These tests cover:
- GitCommit and PullRequest validation logic
- Controller reconcile logic with mocked clients
- Edge cases like missing secrets and invalid configurations

### 2. Integration Tests (with envtest)
Integration tests using controller-runtime's `envtest` that creates a minimal API server:

```bash
just test-integration
# or directly  
go test -v ./test/suite_test.go ./test/gitcommit_controller_test.go ./test/pullrequest_controller_test.go
```

These tests cover:
- Full controller reconciliation loops
- CRD validation
- Status updates
- Resource lifecycle management

### 3. Running Tests

**Quick Start (Recommended):**
```bash
# Run integration tests with automatic setup
just test-integration

# Or run all tests (unit + integration) with automatic setup  
just test-all

# For unit tests only (no setup required)
just test
```

**Manual Setup Commands:**
```bash
# Set up test environment manually (if needed)
just setup-test-env

# Run unit tests only (no external dependencies)
just test-unit
```

## Test Environment

The tests use:
- **Fake clients** for unit tests (no external dependencies)
- **envtest** for integration tests (starts a minimal etcd + API server)
- **Ginkgo/Gomega** for BDD-style testing (integration tests)
- **Standard Go testing** for unit tests

### Integration Test Setup

Integration tests require kubebuilder binaries. To set them up:

```bash
# Install setup-envtest tool
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Download and setup the binaries
# For corporate environments with proxy restrictions:
# 1. First configure your corporate environment:
#    source corporate-config.env
# 2. Then run setup-envtest (it will use SETUP_ENVTEST_INDEX from corporate config):
#    setup-envtest use --bin-dir ./bin/kubebuilder
#
# For direct GitHub access (public environments):
setup-envtest use --bin-dir ./bin/kubebuilder
export KUBEBUILDER_ASSETS=$(pwd)/bin/kubebuilder/k8s/1.34.1-darwin-arm64

# Or if you have direct GitHub access:
setup-envtest use --bin-dir /usr/local/kubebuilder/bin

# Now you can run integration tests
just test-integration
```

## What Gets Tested

### GitCommit Controller
- Reconciliation with valid/invalid configurations
- Authentication secret handling
- Status phase transitions
- Error handling for missing secrets
- Git repository validation

### PullRequest Controller  
- Pull request creation logic
- Branch validation (head != base)
- GitHub API interaction patterns
- File content management
- Status tracking

### API Types
- Field validation
- Required field checks
- Custom resource creation/deletion
- Status subresource updates

## Running Tests in CI/CD

The tests are designed to run in CI environments without requiring:
- Real Kubernetes clusters
- External git repositories  
- GitHub API tokens
- Network connectivity

All external dependencies are mocked or use fake implementations.

## TODO
curl -s "https://cloud.galos.one/prometheus/api/v1/query?query=scalar(max(max_over_time(smartmeter%7Bkind%3D%22total_power%22%7D%5B1d%5D))-max(max_over_time(smartmeter%7Bkind%3D%22total_power%22%7D%5B1d%5D%20offset%201d)))"

{"status":"success","data":{"resultType":"scalar","result":[1759433836.397,"24.450000000004366"]}}