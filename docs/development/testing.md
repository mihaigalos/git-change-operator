# Testing Guide

This guide covers all aspects of testing the Git Change Operator, from unit tests to end-to-end integration testing in Kubernetes environments.

## Overview

The Git Change Operator uses a comprehensive testing strategy:

- **Unit Tests** - Test individual components in isolation
- **Integration Tests** - Test controller logic with fake Kubernetes API
- **End-to-End Tests** - Test complete workflows in real Kubernetes clusters
- **Performance Tests** - Validate operator performance under load
- **Security Tests** - Verify security configurations and policies

## Quick Start

### Run All Tests

```bash
# Run everything
make test-all

# Just unit tests
make test

# Just integration tests  
make test-integration

# Just e2e tests
make test-e2e
```

### Test Configuration

Unit tests can be configured with environment variables:

```bash
# Use different test timeout
export TEST_TIMEOUT=300s
make test

# Enable debug logging
export TEST_DEBUG=true
make test

# Use custom test assets
export TEST_ASSETS_DIR=/path/to/assets
make test
```