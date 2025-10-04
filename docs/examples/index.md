# Examples

This section provides practical examples of using the Git Change Operator in various scenarios. Each example includes complete YAML configurations and explanations.

## Basic Examples

### [Basic GitCommit](basic-gitcommit.md)

- Creating your first GitCommit resource
- Basic authentication setup
- Simple file creation

### [GitCommit with Resource References](gitcommit-resourcerefs.md)

- Resource reference strategies
- Dynamic file generation
- Multiple resource references

### [PullRequest Creation](pullrequest.md)

- GitHub integration setup
- Branch management
- Pull request metadata

### [Prometheus API Integration](prometheus-api-example.md)

- Multiple REST API queries
- JSON parsing with CEL (Common Expression Language) expressions
- Prometheus metrics integration
- Configurable output formatting

## Advanced Scenarios

### [Advanced Configurations](advanced.md)

- Mixed resource references
- Complex file structures
- Error handling strategies

### [Corporate Environment Setup](corporate-setup.md)

- Proxy configuration
- Certificate management
- Security best practices

## Quick Start

If you're new to the Git Change Operator, start with the [Basic GitCommit](basic-gitcommit.md) example and work your way through the more complex scenarios.

## Example Repository Structure

All examples assume a typical repository structure:

```
your-repo/
├── manifests/           # Kubernetes manifests
│   ├── deployments/
│   ├── services/
│   └── configmaps/
├── docs/               # Documentation
└── README.md
```

## Prerequisites

Before running these examples, ensure you have:

1. **Kubernetes cluster** with the Git Change Operator installed
2. **Git repository** with appropriate permissions
3. **Authentication secrets** properly configured
4. **kubectl** access to your cluster

See the [Installation Guide](../user-guide/installation.md) for setup instructions.