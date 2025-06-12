# GitCommit with Resource References

This example shows how to use resource references to automatically export Kubernetes resources to Git repositories using different extraction strategies.

## Prerequisites

- Basic GitCommit setup completed (see [Basic GitCommit](basic-gitcommit.md))
- Existing ConfigMaps and Secrets in your cluster
- Understanding of [Resource Reference Strategies](../reference/resource-reference-strategies.md)

## Sample Kubernetes Resources

Let's create some sample resources to export:

```bash
# Create a sample ConfigMap
kubectl create configmap app-config \
  --from-literal=database.host=localhost \
  --from-literal=database.port=5432 \
  --from-literal=redis.host=redis.example.com \
  --from-file=app.properties=/path/to/app.properties

# Create a sample Secret
kubectl create secret generic api-credentials \
  --from-literal=username=api-user \
  --from-literal=password=secret-password \
  --from-literal=api-key=abc123def456
```

## Example 1: Dump Strategy

Export complete resources as YAML files for backup purposes:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: resource-backup
spec:
  repository:
    url: "https://github.com/your-username/k8s-backups.git"
    branch: "main"
    
  auth:
    secretName: "git-credentials"
    
  commit:
    author: "Backup Operator <backup@example.com>"
    message: "Automated backup of cluster resources"
    
  resourceReferences:
    # Backup ConfigMap as complete YAML
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "dump"
      output:
        path: "backups/configmaps/app-config.yaml"
        
    # Backup Secret as complete YAML  
    - name: "api-credentials"
      apiVersion: "v1"
      kind: "Secret"
      namespace: "default"
      strategy: "dump"
      output:
        path: "backups/secrets/api-credentials.yaml"
```

**Result**: Creates complete YAML files preserving all metadata:

```
backups/
├── configmaps/
│   └── app-config.yaml
└── secrets/
    └── api-credentials.yaml
```

## Example 2: Fields Strategy

Extract individual data fields as separate files:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: config-export
spec:
  repository:
    url: "https://github.com/your-username/app-configs.git"
    
  auth:
    secretName: "git-credentials"
    
  commit:
    author: "Config Operator <config@example.com>"
    message: "Export application configuration files"
    
  resourceReferences:
    # Extract all ConfigMap fields as separate files
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "fields"
      output:
        path: "production/config/"
        
    # Extract all Secret fields as separate files
    - name: "api-credentials"
      apiVersion: "v1" 
      kind: "Secret"
      namespace: "default"
      strategy: "fields"
      output:
        path: "production/secrets/"
```

**Result**: Creates individual files for each data field:

```
production/
├── config/
│   ├── database.host
│   ├── database.port
│   ├── redis.host
│   └── app.properties
└── secrets/
    ├── username
    ├── password
    └── api-key
```

## Example 3: Single-Field Strategy

Extract specific fields with custom naming:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: targeted-export
spec:
  repository:
    url: "https://github.com/your-username/deployment-configs.git"
    
  auth:
    secretName: "git-credentials"
    
  commit:
    author: "Deployment Operator <deploy@example.com>"
    message: "Update deployment configuration values"
    
  resourceReferences:
    # Extract specific database host for external systems
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "single-field"
      field: "database.host"
      output:
        path: "external-systems/database-endpoint.txt"
        
    # Extract API key for CI/CD pipeline
    - name: "api-credentials"
      apiVersion: "v1"
      kind: "Secret" 
      namespace: "default"
      strategy: "single-field"
      field: "api-key"
      output:
        path: "ci-cd/api-credentials/key.txt"
        
    # Extract application properties for configuration management
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "single-field"
      field: "app.properties"
      output:
        path: "apps/myapp/application.properties"
```

**Result**: Creates specifically named files:

```
external-systems/
└── database-endpoint.txt  # Contains: localhost

ci-cd/
└── api-credentials/
    └── key.txt            # Contains: abc123def456

apps/
└── myapp/
    └── application.properties  # Contains: contents of app.properties
```

## Example 4: Mixed Strategies

Combine multiple strategies in a single GitCommit for comprehensive exports:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: comprehensive-export
spec:
  repository:
    url: "https://github.com/your-username/k8s-exports.git"
    
  auth:
    secretName: "git-credentials"
    
  commit:
    author: "K8s Operator <k8s@example.com>"
    message: "Comprehensive configuration export"
    
  files:
    # Add metadata about the export
    - path: "metadata/export-info.yaml"
      content: |
        export_timestamp: "2023-10-01T10:00:00Z"
        cluster: "production"
        namespace: "default"
        operator_version: "v1.0.0"
        
  resourceReferences:
    # Full backup for disaster recovery
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "dump"
      output:
        path: "backup/full/app-config-backup.yaml"
        
    # Individual config files for deployment
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "fields"
      output:
        path: "deploy/config/"
        
    # Specific database password for external access
    - name: "api-credentials"
      apiVersion: "v1"
      kind: "Secret"
      namespace: "default"
      strategy: "single-field"
      field: "password"
      output:
        path: "external/db-password"
        
    # API key for monitoring system
    - name: "api-credentials"
      apiVersion: "v1"
      kind: "Secret"
      namespace: "default"
      strategy: "single-field"
      field: "api-key"
      output:
        path: "monitoring/api-key.txt"
```

## Example 5: Cross-Namespace Resources

Export resources from multiple namespaces:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: multi-namespace-export
  namespace: default  # GitCommit can be in any namespace
spec:
  repository:
    url: "https://github.com/your-username/multi-ns-config.git"
    
  auth:
    secretName: "git-credentials"
    
  commit:
    author: "Multi-NS Operator <multi@example.com>"
    message: "Export configurations from multiple namespaces"
    
  resourceReferences:
    # Production namespace ConfigMap
    - name: "prod-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "production"
      strategy: "fields"
      output:
        path: "environments/production/"
        
    # Staging namespace ConfigMap
    - name: "stage-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "staging"
      strategy: "fields"
      output:
        path: "environments/staging/"
        
    # Development namespace Secret
    - name: "dev-secrets"
      apiVersion: "v1"
      kind: "Secret"
      namespace: "development"
      strategy: "single-field"
      field: "database.password"
      output:
        path: "environments/development/db-password"
```

## Example 6: Custom Resources

Export custom Kubernetes resources:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: custom-resource-export
spec:
  repository:
    url: "https://github.com/your-username/custom-resources.git"
    
  auth:
    secretName: "git-credentials"
    
  commit:
    author: "CRD Operator <crd@example.com>"
    message: "Export custom resource configurations"
    
  resourceReferences:
    # Export complete custom resource
    - name: "my-application"
      apiVersion: "apps.example.com/v1"
      kind: "Application"
      namespace: "default"
      strategy: "dump"
      output:
        path: "applications/my-application.yaml"
        
    # Extract specific configuration section
    - name: "my-application"
      apiVersion: "apps.example.com/v1"
      kind: "Application"
      namespace: "default"
      strategy: "single-field"
      field: "spec.configuration"
      output:
        path: "configurations/my-app-config.json"
        
    # Export database configuration
    - name: "database-config"
      apiVersion: "database.example.com/v1"
      kind: "PostgreSQLCluster"
      namespace: "default"
      strategy: "single-field"
      field: "spec.postgresql.parameters"
      output:
        path: "databases/postgresql/parameters.yaml"
```

## Automation with Write Modes

### Append Mode for Log Aggregation

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: log-aggregation
spec:
  repository:
    url: "https://github.com/your-username/cluster-logs.git"
    
  auth:
    secretName: "git-credentials"
    
  commit:
    author: "Log Aggregator <logs@example.com>"
    message: "Append latest application logs"
    
  writeMode: "append"  # Append to existing files
  
  files:
    - path: "logs/collection-timestamp.log"
      content: |
        === Log Collection: 2023-10-01T10:00:00Z ===
        
  resourceReferences:
    # Append application logs
    - name: "app-logs"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "single-field"
      field: "application.log"
      output:
        path: "logs/application.log"
        
    # Append error logs
    - name: "error-logs"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "single-field"
      field: "error.log"
      output:
        path: "logs/errors.log"
```

## Monitoring and Validation

### Check GitCommit Status

```bash
# Get status of all GitCommits
kubectl get gitcommits

# Detailed status of specific GitCommit
kubectl describe gitcommit comprehensive-export

# Check conditions
kubectl get gitcommit comprehensive-export -o jsonpath='{.status.conditions}'
```

### Verify Git Repository

```bash
# Check latest commits
git log --oneline -5

# Verify file contents
ls -la deploy/config/
cat external/db-password

# Check commit hash matches
kubectl get gitcommit comprehensive-export -o jsonpath='{.status.lastCommitHash}'
```

## Troubleshooting Resource References

### Resource Not Found

```bash
# Verify resource exists
kubectl get configmap app-config -n default

# Check resource in different namespace
kubectl get configmap app-config --all-namespaces

# List all ConfigMaps
kubectl get configmaps
```

### Field Path Issues

```bash
# Inspect resource structure
kubectl get configmap app-config -o yaml

# Check available fields
kubectl get configmap app-config -o jsonpath='{.data}'

# Test field path
kubectl get configmap app-config -o jsonpath='{.data.database\.host}'
```

### RBAC Issues

```bash
# Check operator permissions
kubectl auth can-i get configmaps \
  --as=system:serviceaccount:git-change-operator-system:git-change-operator-controller-manager

# Check cross-namespace permissions
kubectl auth can-i get secrets -n production \
  --as=system:serviceaccount:git-change-operator-system:git-change-operator-controller-manager
```

## Best Practices

1. **Strategy Selection**:
   - Use `dump` for complete backups
   - Use `fields` for configuration management
   - Use `single-field` for specific integrations

2. **Path Organization**:
   ```
   ├── backup/          # Complete resource dumps
   ├── deploy/          # Deployment configurations  
   ├── external/        # External system integrations
   └── monitoring/      # Monitoring system configs
   ```

3. **Resource Naming**:
   - Use descriptive resource names
   - Include environment in ConfigMap names
   - Separate secrets by purpose

4. **Security Considerations**:
   - Be careful with Secret exports
   - Use appropriate Git repository permissions
   - Consider separate repositories for sensitive data

## Next Steps

- [PullRequest Creation](pullrequest.md) - Create pull requests with resource references
- [Advanced Scenarios](advanced.md) - Complex multi-cluster setups