# Resource References

The Git Change Operator supports referencing arbitrary Kubernetes resources and committing their content to Git repositories with flexible output strategies. This powerful feature enables you to automatically export cluster state to Git repositories.

## Overview

Instead of specifying static file content, you can reference existing Kubernetes resources (Secrets, ConfigMaps, Custom Resources, etc.) and have the operator extract and commit their data using various strategies.

## Basic Concept

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: export-resources
spec:
  repository: https://github.com/user/repo.git
  branch: main
  commitMessage: "Export cluster resources"
  authSecretRef: git-token
  resourceRefs:
    - name: app-config
      kind: ConfigMap
      strategy:
        type: dump
        path: configs/app
```

## Resource Reference Structure

```yaml
resourceRefs:
  - apiVersion: "v1"           # API version of the resource
    kind: "ConfigMap"          # Kind of the resource  
    name: "my-config"          # Name of the resource
    namespace: "default"       # Namespace (optional, defaults to resource namespace)
    strategy:                  # Output strategy configuration
      type: "dump"             # Output type: dump, fields, or single-field
      path: "output/path"      # Base path for output files
      writeMode: "overwrite"   # Write mode: overwrite or append
      fieldRef:                # Field reference (for single-field strategy)
        key: "fieldname"       # Field key to extract
        fileName: "custom.txt" # Custom filename (optional)
```

## Output Strategies

### Dump Strategy

Exports the entire resource as YAML, perfect for backing up complete resource definitions.

```yaml
strategy:
  type: dump
  path: backups/configmaps
```

**Example Output**: `backups/configmaps/my-config.yaml`
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  database.url: postgres://localhost:5432/mydb
  log.level: info
```

### Fields Strategy

Extracts all data fields as separate files, useful for exporting configuration files individually.

```yaml
strategy:
  type: fields
  path: configs/app
```

**Example Output**:
- `configs/app/database.url` → `postgres://localhost:5432/mydb`
- `configs/app/log.level` → `info`

### Single-Field Strategy

Extracts one specific field, ideal for extracting individual configuration files or secrets.

```yaml
strategy:
  type: single-field
  path: configs/database
  fieldRef:
    key: database.url
    fileName: connection.txt
```

**Example Output**: `configs/database/connection.txt` → `postgres://localhost:5432/mydb`

## Write Modes

### Overwrite Mode (Default)

Replaces existing file content completely.

```yaml
writeMode: overwrite
```

### Append Mode

Adds content to existing files, useful for log aggregation or cumulative exports.

```yaml
writeMode: append
```

## Practical Examples

### Export All ConfigMaps

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: backup-configs
spec:
  repository: https://github.com/company/k8s-backups.git
  branch: main
  commitMessage: "Backup ConfigMaps - {{.Timestamp}}"
  authSecretRef: backup-token
  resourceRefs:
    - name: app-config
      kind: ConfigMap
      strategy:
        type: dump
        path: backups/configmaps
    - name: database-config
      kind: ConfigMap  
      strategy:
        type: fields
        path: configs/database
```

### Secret Management

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: export-certificates
spec:
  repository: https://github.com/company/certificates.git
  branch: certificates
  commitMessage: "Update TLS certificates"
  authSecretRef: cert-manager-token
  resourceRefs:
    - name: tls-secret
      kind: Secret
      strategy:
        type: single-field
        path: certs/api
        fieldRef:
          key: tls.crt
          fileName: certificate.pem
    - name: tls-secret
      kind: Secret
      strategy:
        type: single-field  
        path: certs/api
        fieldRef:
          key: tls.key
          fileName: private-key.pem
```

### Custom Resources

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: export-custom-resources
spec:
  repository: https://github.com/company/cluster-state.git
  branch: main
  commitMessage: "Export application configurations"
  authSecretRef: git-token
  resourceRefs:
    - apiVersion: apps.company.com/v1
      kind: Application
      name: web-app
      strategy:
        type: dump
        path: applications/web
    - apiVersion: networking.istio.io/v1beta1
      kind: VirtualService
      name: web-routing
      strategy:
        type: dump
        path: networking/virtual-services
```

## Combining with Static Files

You can mix resource references with static files in the same commit:

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: complete-backup
spec:
  repository: https://github.com/company/backups.git
  branch: main
  commitMessage: "Complete cluster backup"
  authSecretRef: git-token
  files:
    - path: README.md
      content: |
        # Cluster Backup
        Generated at: {{.Timestamp}}
        Cluster: production
  resourceRefs:
    - name: app-config
      kind: ConfigMap
      strategy:
        type: dump
        path: configs
```

## Best Practices

### Security Considerations

!!! warning "Sensitive Data"
    Be careful when exporting Secrets or resources containing sensitive information. Ensure your Git repository has appropriate access controls.

### Path Organization

Use clear, hierarchical paths:
```yaml
# Good
path: backups/production/configmaps
path: configs/database/connection-strings
path: certificates/tls/web-app

# Avoid
path: stuff
path: data
path: output
```

### Error Handling

The operator will:
- Skip resources that don't exist
- Log warnings for missing fields
- Continue processing other resources if one fails

### Performance Tips

- Use specific resource references rather than broad exports
- Consider resource size when using dump strategy
- Use append mode judiciously to avoid large files

## Troubleshooting

### Common Issues

**Resource not found**:
```
Error: resource "my-config" of kind "ConfigMap" not found in namespace "default"
```
- Verify the resource exists: `kubectl get configmap my-config`
- Check the namespace specification

**Field not found**:
```
Warning: field "missing-key" not found in ConfigMap "my-config"
```
- List available fields: `kubectl get configmap my-config -o yaml`
- Verify the field key spelling

**Permission denied**:
```
Error: failed to get resource: configmaps "my-config" is forbidden
```
- Check RBAC permissions for the operator service account
- Ensure the operator can read the referenced resource types

### Debugging

Enable debug logging to see detailed resource processing:
```yaml
# In operator deployment
env:
  - name: LOG_LEVEL
    value: debug
```

Check operator logs:
```bash
kubectl logs -l app=git-change-operator -f
```