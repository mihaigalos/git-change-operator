# API Reference

Welcome to the Git Change Operator API reference documentation. This section provides detailed information about the Custom Resource Definitions (CRDs), configuration options, and API specifications.

## Available Resources

### GitCommit
The `GitCommit` custom resource allows you to create git commits automatically based on Kubernetes cluster resources.

**Key Features:**
- Automatic file creation from cluster resources
- Resource reference strategies (dump, fields, single-field)
- Configurable write modes (overwrite, append)
- Authentication via Kubernetes secrets

[View GitCommit CRD Specification](crd-spec.md#gitcommit)

### PullRequest
The `PullRequest` custom resource creates GitHub pull requests automatically with files generated from cluster resources.

**Key Features:**
- Automatic branch creation and management
- GitHub API integration
- File generation from resource references
- Pull request metadata configuration

[View PullRequest CRD Specification](crd-spec.md#pullrequest)

## Configuration Reference

| Topic | Description |
|-------|-------------|
| [CRD Specification](crd-spec.md) | Complete schema for GitCommit and PullRequest resources |
| [Resource Reference Strategies](resource-reference-strategies.md) | How to extract data from Kubernetes resources |
| [Write Modes](write-modes.md) | File writing behavior (overwrite vs append) |
| [Error Handling](error-handling.md) | Common errors and troubleshooting |
| [Full API Reference](api.md) | Complete API documentation |

## Quick Links

- **[Getting Started](../user-guide/index.md)** - Installation and basic setup
- **[Examples](../examples/index.md)** - Practical usage examples

## API Versions

The Git Change Operator uses the API group `gco.galos.one` with version `v1`.

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
# or
kind: PullRequest
```

All resources in this API group follow the same versioning scheme and are designed to be backward compatible within the major version.