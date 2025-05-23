# Examples

This section provides practical examples of using the Git Change Operator in various scenarios. Each example includes complete YAML configurations and explanations.

## Basic Examples

### [Basic GitCommit](basic-gitcommit.md)
Learn how to create simple git commits with static files and basic resource references.

**What you'll learn:**
- Creating your first GitCommit resource
- Basic authentication setup
- Simple file creation

### [GitCommit with Resource References](gitcommit-resourcerefs.md)
Advanced GitCommit configurations using resource references to automatically generate files from cluster state.

**What you'll learn:**
- Resource reference strategies
- Dynamic file generation
- Multiple resource references

### [PullRequest Creation](pullrequest.md)
Create GitHub pull requests automatically with the PullRequest resource.

**What you'll learn:**
- GitHub integration setup
- Branch management
- Pull request metadata

## Advanced Scenarios

### [Advanced Configurations](advanced.md)
Complex use cases combining multiple features and advanced configuration options.

**What you'll learn:**
- Mixed resource references
- Complex file structures
- Error handling strategies

### [Corporate Environment Setup](corporate-setup.md)
Special considerations for corporate environments with proxies, certificates, and security policies.

**What you'll learn:**
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

## Contributing Examples

Have a useful example to share? See our [Contributing Guide](../development/contributing.md) for how to submit new examples.