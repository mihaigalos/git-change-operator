# git-change-operator

A Kubernetes operator that enables automated Git operations from within your cluster. Commit files directly or reference existing Kubernetes resources (Secrets, ConfigMaps, etc.) and push them to Git repositories with flexible output strategies.

## Features

- **Direct File Commits**: Commit static file content to Git repositories
- **Resource References**: Reference arbitrary Kubernetes resources and commit their data
- **Flexible Output Strategies**: 
  - Dump entire resources as YAML
  - Extract all resource fields as separate files
  - Extract specific fields with custom naming
- **Write Modes**: Overwrite or append to existing files
- **Git Operations**: Support for both direct commits and pull requests
- **Secure Authentication**: Uses Kubernetes Secrets for Git authentication

## Resource Reference Capabilities

The operator can reference any Kubernetes resource and extract its data using various strategies:

### Output Strategies
1. **Dump**: Output entire resource as YAML
2. **Fields**: Extract all data fields as separate files  
3. **Single-Field**: Extract specific fields with custom naming

### Write Modes
- **Overwrite**: Replace file content (default)
- **Append**: Add to existing file content

See [Resource Reference Guide](test/resources/RESOURCE_REFERENCE_GUIDE.md) for detailed documentation and examples.

## Quick Start

1. **Install the operator**:
   ```bash
   helm install git-change-operator helm/git-change-operator/
   ```

2. **Create a Git token secret**:
   ```bash
   kubectl create secret generic git-token --from-literal=token=your_github_token
   ```

3. **Apply a GitCommit resource**:
   ```yaml
   apiVersion: git.galos.one/v1
   kind: GitCommit
   metadata:
     name: example-commit
   spec:
     repository: "https://github.com/your/repo.git"
     branch: "main"
     commitMessage: "Add configuration from Secret"
     authSecretRef: "git-token"
     resourceRefs:
     - apiVersion: "v1"
       kind: "Secret"
       name: "my-secret"
       strategy:
         type: "dump"
         path: "config/secret-dump"
   ```

## Documentation

- [Resource Reference Guide](test/resources/RESOURCE_REFERENCE_GUIDE.md) - Comprehensive guide for using resource references
- [API Reference](api/v1/) - CRD definitions and types
- [Examples](test/resources/) - Example resources demonstrating various strategies

## Development

Built with:
- Go 1.21+
- Kubebuilder v3
- controller-runtime