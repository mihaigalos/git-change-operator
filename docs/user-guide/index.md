# User Guide

Welcome to the Git Change Operator User Guide! This comprehensive guide will help you understand, install, configure, and use the Git Change Operator effectively.

## Getting Started

If you're new to the Git Change Operator, start here:

1. **[Installation](installation.md)** - Install the operator in your Kubernetes cluster
2. **[Quick Start](quick-start.md)** - Create your first GitCommit resource
3. **[Configuration](configuration.md)** - Configure authentication and operator settings

## Core Concepts

### Resource Types

The operator provides two main resource types:

- **[GitCommit Resources](gitcommit.md)** - Direct commits to Git repositories
- **[PullRequest Resources](pullrequest.md)** - Create GitHub pull requests

### Advanced Features

- **[Resource References](resource-references.md)** - Reference existing Kubernetes resources
- **[Authentication](authentication.md)** - Set up secure Git authentication

## Common Workflows

### Basic File Commit
```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: basic-commit
spec:
  repository: https://github.com/user/repo.git
  branch: main
  commitMessage: "Add configuration file"
  authSecretRef: git-token
  files:
    - path: config/app.yaml
      content: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: app-config
```

### Resource Reference
```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: export-configmap
spec:
  repository: https://github.com/user/repo.git
  branch: main
  commitMessage: "Export ConfigMap data"
  authSecretRef: git-token
  resourceRefs:
    - name: app-config
      kind: ConfigMap
      strategy:
        type: fields
        outputPath: configs/
```

## Best Practices

### Security
- Always use Kubernetes Secrets for Git authentication
- Limit operator permissions using RBAC
- Use service accounts with minimal required permissions

### Performance
- Use specific resource references instead of broad selectors
- Consider using append mode for log-like data
- Batch related changes in single commits

### Organization
- Use meaningful commit messages with context
- Organize files with clear directory structures
- Use consistent naming conventions

## Troubleshooting

### Common Issues

- **Authentication failures**: Verify Git token has correct permissions
- **Resource not found**: Check resource exists in the correct namespace
- **Permission errors**: Ensure operator has RBAC permissions for referenced resources

### Getting Help

- Check the [Error Handling](../reference/error-handling.md) reference
- Review operator logs: `kubectl logs -l app=git-change-operator`
- Join our community discussions on GitHub

## What's Next?

- Explore [Examples](../examples/index.md) for real-world use cases
- Check the [API Reference](../reference/index.md) for detailed specifications
- Learn about [Development](../development/index.md) if you want to contribute