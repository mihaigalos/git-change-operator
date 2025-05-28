# Resource Reference Strategies

Resource reference strategies determine how data is extracted from Kubernetes resources and written to files in your Git repository.

## Overview

The Git Change Operator supports three strategies for extracting data from Kubernetes resources:

1. **[Dump Strategy](#dump-strategy)** - Export the entire resource as YAML
2. **[Fields Strategy](#fields-strategy)** - Extract all data fields as separate files  
3. **[Single-Field Strategy](#single-field-strategy)** - Extract a specific field with custom naming

## Dump Strategy

The `dump` strategy exports the complete Kubernetes resource as a YAML file.

### Configuration

```yaml
resourceReferences:
  - name: "my-configmap"
    apiVersion: "v1"
    kind: "ConfigMap" 
    namespace: "default"
    strategy: "dump"
    output:
      path: "exported/my-configmap.yaml"
```

### Output

For a ConfigMap like this:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: default
data:
  app.properties: |
    server.port=8080
    debug=true
  version: "1.0.0"
```

The `dump` strategy creates `exported/my-configmap.yaml` with the complete resource:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: default
  resourceVersion: "12345"
  # ... other metadata
data:
  app.properties: |
    server.port=8080
    debug=true
  version: "1.0.0"
```

### Use Cases
- **Backup and versioning**: Complete resource snapshots
- **Resource migration**: Moving resources between clusters
- **Audit trails**: Full resource history in Git

## Fields Strategy

The `fields` strategy extracts each data field from the resource as a separate file.

### Configuration

```yaml
resourceReferences:
  - name: "my-configmap"
    apiVersion: "v1"
    kind: "ConfigMap"
    namespace: "default" 
    strategy: "fields"
    output:
      path: "config/"  # Directory path
```

### Output

For the same ConfigMap above, the `fields` strategy creates:

- `config/app.properties`:
  ```
  server.port=8080
  debug=true
  ```

- `config/version`:
  ```
  1.0.0
  ```

### Supported Resource Types

| Resource Type | Extracted Fields |
|---------------|------------------|
| ConfigMap | `.data.*` and `.binaryData.*` |
| Secret | `.data.*` (base64 decoded) |
| Custom Resources | `.spec.*`, `.status.*`, and other top-level fields |

### Use Cases
- **Configuration management**: Individual config files
- **Template generation**: Separate files for different components
- **Fine-grained versioning**: Track changes to individual settings

## Single-Field Strategy

The `single-field` strategy extracts a specific field with custom file naming and path control.

### Configuration

```yaml
resourceReferences:
  - name: "database-secret"
    apiVersion: "v1"
    kind: "Secret"
    namespace: "default"
    strategy: "single-field"
    field: "password"  # Field to extract
    output:
      path: "secrets/db-password.txt"  # Custom file path
```

### Output

For a Secret like this:
```yaml
apiVersion: v1
kind: Secret
data:
  username: dXNlcg==  # "user"
  password: cGFzcw==  # "pass"
```

The `single-field` strategy creates `secrets/db-password.txt` with:
```
pass
```

### Field Path Syntax

The `field` parameter supports dot notation for nested fields:

```yaml
# Extract nested field from Custom Resource
resourceReferences:
  - name: "my-app-config"
    apiVersion: "apps.example.com/v1"
    kind: "AppConfig"
    strategy: "single-field"
    field: "spec.database.host"  # Nested field
    output:
      path: "config/db-host.txt"
```

### Use Cases
- **Credential extraction**: Individual secrets or tokens
- **Configuration values**: Specific settings for external systems  
- **Custom naming**: Files with meaningful names for consumers

## Advanced Examples

### Multiple Strategies in One GitCommit

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: multi-strategy-commit
spec:
  repository:
    url: "https://github.com/user/config-repo.git"
  auth:
    secretName: "git-credentials"
  commit:
    author: "Config Operator <config@example.com>"
    message: "Update configurations and secrets"
    
  resourceReferences:
    # Full backup of critical ConfigMap
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "production"
      strategy: "dump"
      output:
        path: "backups/app-config-backup.yaml"
        
    # Individual config files for deployment
    - name: "app-config" 
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "production"
      strategy: "fields"
      output:
        path: "deploy/config/"
        
    # Extract database password for external use
    - name: "db-credentials"
      apiVersion: "v1" 
      kind: "Secret"
      namespace: "production"
      strategy: "single-field"
      field: "password"
      output:
        path: "secrets/db-password"
```

### Working with Custom Resources

```yaml
resourceReferences:
  # Dump entire custom resource
  - name: "my-application"
    apiVersion: "apps.example.com/v1"
    kind: "Application"
    namespace: "default"
    strategy: "dump"
    output:
      path: "applications/my-app.yaml"
      
  # Extract specific configuration section
  - name: "my-application"
    apiVersion: "apps.example.com/v1" 
    kind: "Application"
    namespace: "default"
    strategy: "single-field"
    field: "spec.configuration"
    output:
      path: "config/app-settings.json"
```

## Strategy Selection Guidelines

| Use Case | Recommended Strategy | Reason |
|----------|---------------------|---------|
| Resource backup | `dump` | Preserves complete resource state |
| Config file deployment | `fields` | Creates usable individual files |
| Secret extraction | `single-field` | Precise control over sensitive data |
| Documentation generation | `dump` | Complete resource schemas |
| Template processing | `fields` | Separate files for different templates |
| External tool integration | `single-field` | Custom paths and naming |

## Error Handling

### Common Issues

| Error | Cause | Solution |
|-------|--------|----------|
| "Field not found" | Invalid field path in `single-field` strategy | Check resource structure and field path |
| "Resource not found" | Referenced resource doesn't exist | Verify resource name, namespace, and API version |
| "No data fields" | Resource has no extractable data | Use `dump` strategy or check resource content |

### Validation

The operator validates resource references at reconciliation time:
- Checks if referenced resources exist
- Validates field paths for `single-field` strategy  
- Ensures output paths are valid

See [Error Handling](error-handling.md) for detailed troubleshooting information.