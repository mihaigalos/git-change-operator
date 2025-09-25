# GitCommit Resources

The `GitCommit` resource is the primary way to automate Git commit operations based on Kubernetes cluster data. This guide covers everything you need to know about creating, configuring, and managing GitCommit resources.

## Overview

GitCommit resources enable you to:
- **Extract data from Kubernetes resources** and commit it to Git repositories
- **Transform resource data** using various strategies and templates
- **Automate configuration synchronization** between cluster and Git
- **Implement GitOps workflows** with automated commits

## Basic GitCommit Resource

### Minimal Example

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: simple-commit
  namespace: default
spec:
  repository: "https://github.com/myorg/config-repo.git"
  branch: "main"
  message: "Update configuration from cluster"
  resourceRef:
    apiVersion: v1
    kind: ConfigMap
    name: app-config
    namespace: default
    path: "config.yaml"
```

This creates a commit with the contents of the `app-config` ConfigMap.

### Complete Example

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: comprehensive-commit
  namespace: default
  labels:
    app: myapp
    environment: production
spec:
  # Git repository configuration
  repository: "https://github.com/myorg/config-repo.git"
  branch: "main"
  message: |
    Update {{ .resourceRef.kind }}/{{ .resourceRef.name }} configuration
    
    Namespace: {{ .resourceRef.namespace }}
    Updated: {{ .timestamp }}
    Cluster: {{ .cluster.name }}

  # Authentication
  credentials:
    secretName: git-credentials
    usernameKey: username
    passwordKey: token

  # Git author information
  author:
    name: "Git Change Operator"
    email: "operator@mycompany.com"

  # Resource reference
  resourceRef:
    apiVersion: v1
    kind: ConfigMap
    name: app-config
    namespace: default
    path: "applications/myapp/config.yaml"
    strategy: "template"
    template: |
      # Application Configuration
      # Generated: {{ .timestamp }}
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: {{ .metadata.name }}
        namespace: {{ .metadata.namespace }}
        labels:
      {{ range $key, $value := .metadata.labels }}
        {{ $key }}: {{ $value }}
      {{ end }}
      data:
      {{ range $key, $value := .data }}
        {{ $key }}: |
      {{ $value | indent 4 }}
      {{ end }}

  # File handling
  writeMode: "overwrite"
  fileMode: "0644"
  createDirs: true

  # Reconciliation settings
  reconcileInterval: "300s"
  suspend: false

  # Retry policy
  retryPolicy:
    maxRetries: 3
    backoff: "30s"
```

## Resource Reference Strategies

GitCommit resources can extract data from Kubernetes resources using different strategies:

### Full Resource Strategy

Extract the entire resource as YAML:

```yaml
spec:
  resourceRef:
    apiVersion: v1
    kind: ConfigMap
    name: app-config
    namespace: default
    path: "configmaps/app-config.yaml"
    strategy: "full"  # Default strategy
```

Results in a file containing the complete ConfigMap YAML.

### Value Strategy

Extract a specific field value:

```yaml
spec:
  resourceRef:
    apiVersion: v1
    kind: Secret
    name: database-secret
    namespace: default
    path: "database-url.txt"
    strategy: "value"
    fieldPath: "data.url"
```

Results in a file containing only the decoded value of `data.url`.

### Template Strategy

Transform resource data using Go templates:

```yaml
spec:
  resourceRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
    namespace: default
    path: "deployments/myapp.env"
    strategy: "template"
    template: |
      # Environment configuration for {{ .metadata.name }}
      REPLICAS={{ .spec.replicas }}
      IMAGE={{ (index .spec.template.spec.containers 0).image }}
      {{ range .spec.template.spec.containers }}
      {{ range .env }}
      {{ .name }}={{ .value }}
      {{ end }}
      {{ end }}
```

### JSONPath Strategy

Extract data using JSONPath expressions:

```yaml
spec:
  resourceRef:
    apiVersion: v1
    kind: Pod
    name: mypod
    namespace: default
    path: "pod-ip.txt"
    strategy: "jsonpath"
    jsonPath: "{.status.podIP}"
```

## Write Modes

Control how data is written to files:

### Overwrite Mode

Replace the entire file content (default):

```yaml
spec:
  writeMode: "overwrite"
  resourceRef:
    # ... resource config
    path: "config.yaml"
```

### Append Mode

Add content to the end of existing files:

```yaml
spec:
  writeMode: "append"
  resourceRef:
    # ... resource config  
    path: "logs/events.log"
    template: |
      {{ .timestamp }}: {{ .metadata.name }} updated in {{ .metadata.namespace }}
```

### Merge Mode

Intelligently merge structured data:

```yaml
spec:
  writeMode: "merge"
  resourceRef:
    # ... resource config
    path: "combined-config.yaml"
    strategy: "template"
    template: |
      {{ .metadata.name }}:
        namespace: {{ .metadata.namespace }}
        data:
      {{ range $key, $value := .data }}
        {{ $key }}: {{ $value }}
      {{ end }}
```

## Advanced Resource References

### Multiple Resources

Reference multiple resources in a single commit:

```yaml
spec:
  resourceRefs:  # Note: plural form
    - apiVersion: v1
      kind: ConfigMap
      name: app-config
      namespace: default
      path: "config/app.yaml"
    - apiVersion: v1
      kind: Secret
      name: app-secrets
      namespace: default  
      path: "secrets/app.yaml"
      strategy: "template"
      template: |
        # Secrets (values redacted)
        {{ range $key, $value := .data }}
        {{ $key }}: "[REDACTED]"
        {{ end }}
```

### Resource Selectors

Select resources using labels or field selectors:

```yaml
spec:
  resourceRef:
    apiVersion: v1
    kind: ConfigMap
    namespace: default
    selector:
      matchLabels:
        app: myapp
        environment: production
    path: "configs/"  # Directory for multiple resources
```

### Cross-Namespace References

Reference resources from different namespaces:

```yaml
spec:
  resourceRef:
    apiVersion: v1
    kind: Secret
    name: shared-secret
    namespace: shared-resources  # Different namespace
    path: "secrets/shared.yaml"
```

## File Encryption

Protect sensitive files by encrypting them before committing to Git repositories using age encryption:

### Basic Encryption with SSH Key

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: encrypted-secrets
  namespace: default
spec:
  repository: "https://github.com/myorg/secure-configs.git"
  branch: "main"
  authSecretRef: "git-credentials"
  commitMessage: "Add encrypted database configuration"
  
  encryption:
    enabled: true
    recipients:
      - type: ssh
        secretRef:
          name: ssh-keys
          key: id_rsa.pub
  
  files:
    - path: "database/production.yaml"
      content: |
        database:
          host: prod-db.internal
          username: app_user
          password: super-secret-password
          ssl_key: |
            -----BEGIN PRIVATE KEY-----
            MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC...
            -----END PRIVATE KEY-----
```

### Multiple Recipient Types

Support for different encryption methods in a single GitCommit:

```yaml
spec:
  encryption:
    enabled: true
    fileExtension: ".secret"  # Custom extension (default: .age)
    recipients:
      # Age public key
      - type: age
        secretRef:
          name: age-keys
          key: team-public-key
      
      # SSH public key (from authorized_keys format)
      - type: ssh
        secretRef:
          name: ssh-keys
          key: id_rsa.pub
      
      # Passphrase-based encryption
      - type: passphrase
        secretRef:
          name: encryption-secrets
          key: backup-passphrase

  resourceRefs:
    - apiVersion: v1
      kind: Secret
      name: database-credentials
      namespace: production
      path: "secrets/database.yaml"
      # Will be encrypted as secrets/database.yaml.secret
```

### Encrypted Resource References

Encrypt sensitive data from Kubernetes resources:

```yaml
spec:
  encryption:
    enabled: true
    recipients:
      - type: age
        secretRef:
          name: backup-keys
          key: public-key

  resourceRefs:
    # Encrypt TLS certificates
    - apiVersion: v1
      kind: Secret
      name: tls-cert
      namespace: ingress-system
      path: "certificates/tls.yaml"
    
    # Encrypt database connection strings
    - apiVersion: v1
      kind: ConfigMap
      name: database-config
      namespace: app
      path: "config/database.yaml"
    
    # Encrypt API keys and tokens
    - apiVersion: v1
      kind: Secret
      name: api-credentials
      namespace: integration
      path: "secrets/api-keys.yaml"
      strategy: "fields"  # Extract individual fields as encrypted files
```

### Setting Up Encryption Secrets

Create the necessary secrets for encryption:

```yaml
# Age key secret
apiVersion: v1
kind: Secret
metadata:
  name: age-keys
  namespace: default
type: Opaque
data:
  # Base64 encoded age public key
  team-public-key: YWdlMXh4eGJ4eGJ4eGJ4eGJ4eGJ4eGJ4eGJ4eGJ4eGJ4eGJ4...

---
# SSH key secret  
apiVersion: v1
kind: Secret
metadata:
  name: ssh-keys
  namespace: default
type: Opaque
data:
  # Base64 encoded SSH public key
  id_rsa.pub: c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCZ1FD...

---
# Passphrase secret
apiVersion: v1
kind: Secret
metadata:
  name: encryption-secrets
  namespace: default
type: Opaque
data:
  # Base64 encoded passphrase
  backup-passphrase: bXktc2VjdXJlLXBhc3NwaHJhc2UtZm9yLWJhY2t1cHM=
```

### Encryption Best Practices

**🔐 Security Considerations:**
- Store encryption keys securely in Kubernetes Secrets
- Use different keys for different environments (dev/staging/prod)
- Regularly rotate encryption keys and passphrases
- Consider using age keys for better security than passphrases
- Validate encrypted files can be decrypted before committing

**📁 File Management:**
- Encrypted files use `.age` extension by default (customizable)
- Original filenames are preserved with encryption extension added
- Already encrypted files (`.age` extension) are not re-encrypted
- Use descriptive paths to organize encrypted content

**🔄 GitOps Integration:**
- Encrypted files can be safely stored in public repositories
- Use tools like SOPS or age CLI for manual decryption when needed
- Consider automation for decrypting files in CI/CD pipelines
- Document which files are encrypted for team awareness

## Templating

### Template Functions

Available template functions in GitCommit resources:

```yaml
template: |
  # String functions
  name: {{ .metadata.name | upper }}
  namespace: {{ .metadata.namespace | lower }}
  
  # Date functions
  updated: {{ .timestamp | date "2006-01-02 15:04:05" }}
  
  # Encoding functions
  secret: {{ .data.password | b64decode }}
  encoded: {{ "plain text" | b64encode }}
  
  # Conditional logic
  {{ if .metadata.labels }}
  labels:
  {{ range $key, $value := .metadata.labels }}
    {{ $key }}: {{ $value }}
  {{ end }}
  {{ end }}
  
  # Math functions
  scaled_replicas: {{ .spec.replicas | add 1 }}
  
  # Custom functions
  cluster: {{ cluster_name }}
  git_commit: {{ git_commit_sha }}
```

### Template Variables

Available variables in templates:

| Variable | Description | Example |
|----------|-------------|---------|
| `.metadata` | Resource metadata | `.metadata.name`, `.metadata.namespace` |
| `.spec` | Resource specification | `.spec.replicas`, `.spec.template` |
| `.status` | Resource status | `.status.phase`, `.status.conditions` |
| `.data` | ConfigMap/Secret data | `.data.config`, `.data.token` |
| `.timestamp` | Current timestamp | `2024-01-15T10:30:00Z` |
| `.cluster` | Cluster information | `.cluster.name`, `.cluster.version` |

### Complex Templates

```yaml
template: |
  # Multi-resource configuration template
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: generated-config
    namespace: {{ .metadata.namespace }}
    labels:
      generated-by: git-change-operator
      source-resource: {{ .metadata.name }}
      timestamp: "{{ .timestamp | date "20060102-150405" }}"
  data:
    config.yaml: |
  {{ range $key, $value := .data }}
    {{ $key }}: {{ $value | quote }}
  {{ end }}
    
    metadata.json: |
      {
        "source": {
          "name": "{{ .metadata.name }}",
          "namespace": "{{ .metadata.namespace }}",
          "resourceVersion": "{{ .metadata.resourceVersion }}"
        },
        "generated": {
          "timestamp": "{{ .timestamp }}",
          "cluster": "{{ cluster_name }}",
          "operator": "git-change-operator"
        }
      }
```

## Lifecycle Management

### Reconciliation

Control when and how GitCommit resources are reconciled:

```yaml
spec:
  # Reconcile every 5 minutes
  reconcileInterval: "300s"
  
  # Suspend reconciliation
  suspend: true
  
  # One-time execution
  schedule: "once"
  
  # Cron-based execution
  schedule: "0 */6 * * *"  # Every 6 hours
```

### Conditions and Status

Monitor GitCommit resource status:

```bash
# Check GitCommit status
kubectl get gitcommit mycommit -o yaml

# Example status
status:
  phase: "Completed"  # Pending, Running, Completed, Failed
  conditions:
  - type: "Ready"
    status: "True"
    lastTransitionTime: "2024-01-15T10:30:00Z"
    reason: "CommitSuccessful"
    message: "Successfully committed to repository"
  lastCommitSHA: "abc123def456"
  lastCommitTime: "2024-01-15T10:30:00Z"
  observedGeneration: 1
```

### Events

Monitor GitCommit events:

```bash
# View events
kubectl describe gitcommit mycommit

# Example events
Events:
  Type    Reason           Age   From                     Message
  ----    ------           ----  ----                     -------
  Normal  CommitStarted    5m    gitcommit-controller     Starting Git commit operation
  Normal  ResourceFetched  5m    gitcommit-controller     Successfully fetched resource data
  Normal  FileWritten      5m    gitcommit-controller     File written to repository
  Normal  CommitCreated    5m    gitcommit-controller     Commit created: abc123def456
```

## Error Handling

### Common Errors and Solutions

#### Authentication Failures

```yaml
# Error: Authentication failed
status:
  phase: "Failed"
  conditions:
  - type: "Ready"
    status: "False"
    reason: "AuthenticationFailed"
    message: "Invalid credentials for repository"

# Solution: Check credentials secret
kubectl get secret git-credentials -o yaml
```

#### Resource Not Found

```yaml
# Error: Resource reference not found
status:
  phase: "Failed"
  conditions:
  - type: "Ready"
    status: "False"
    reason: "ResourceNotFound"
    message: "ConfigMap 'missing-config' not found in namespace 'default'"

# Solution: Verify resource exists
kubectl get configmap missing-config
```

#### Invalid Git Repository

```yaml
# Error: Invalid repository URL
status:
  phase: "Failed" 
  conditions:
  - type: "Ready"
    status: "False"
    reason: "RepositoryError"
    message: "Repository not found or access denied"

# Solution: Verify repository URL and access
git clone https://github.com/myorg/config-repo.git
```

### Retry Configuration

Configure retry behavior for failed operations:

```yaml
spec:
  retryPolicy:
    maxRetries: 5
    backoff: "30s"
    maxBackoff: "300s"
    backoffMultiplier: 2.0
    retryableErrors:
      - "AuthenticationFailed"
      - "NetworkError"  
      - "TemporaryFailure"
```

## Security Considerations

### RBAC Permissions

GitCommit resources require appropriate RBAC permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: default
  name: gitcommit-reader
rules:
# Permission to manage GitCommit resources
- apiGroups: ["gco.galos.one"]
  resources: ["gitcommits"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Permission to read referenced resources
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets"]
  verbs: ["get", "list", "watch"]
```

### Credential Management

Best practices for credential management:

```yaml
# Use separate secrets for different repositories
apiVersion: v1
kind: Secret
metadata:
  name: production-repo-creds
  namespace: production
type: Opaque
data:
  username: cHJvZC11c2Vy
  token: Z2hwX3Rva2VuX2hlcmU=

---
# Reference in GitCommit
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: prod-commit
  namespace: production
spec:
  credentials:
    secretName: production-repo-creds
```

### Sensitive Data Handling

Prevent sensitive data from being committed:

```yaml
spec:
  resourceRef:
    apiVersion: v1
    kind: Secret
    name: app-secrets
    path: "secrets/app.yaml"
    strategy: "template"
    template: |
      # Redacted secrets configuration
      {{ range $key, $value := .data }}
      {{ $key }}: "[REDACTED-{{ $value | len }}-bytes]"
      {{ end }}
```

## Monitoring and Observability

### Metrics

Monitor GitCommit operations with Prometheus metrics:

```promql
# Total GitCommit operations
git_change_operator_gitcommit_operations_total

# Operation duration
git_change_operator_gitcommit_duration_seconds

# Success rate
rate(git_change_operator_gitcommit_operations_total{status="success"}[5m])
```

### Logging

Configure structured logging for debugging:

```yaml
# Enable debug logging for GitCommit controller
env:
- name: LOG_LEVEL
  value: "debug"
- name: LOG_FORMAT
  value: "json"
```

Example log output:
```json
{
  "level": "info",
  "timestamp": "2024-01-15T10:30:00Z",
  "logger": "gitcommit-controller",
  "message": "Successfully created commit",
  "gitcommit": "default/mycommit",
  "repository": "https://github.com/myorg/repo.git",
  "commitSHA": "abc123def456",
  "duration": "2.3s"
}
```

## Performance Optimization

### Batching Operations

Group multiple resource changes into single commits:

```yaml
spec:
  # Wait for multiple changes before committing
  batchInterval: "60s"
  batchSize: 10
  
  resourceRefs:
    - apiVersion: v1
      kind: ConfigMap
      namespace: default
      selector:
        matchLabels:
          batch-group: "app-configs"
```

### Resource Caching

Enable caching for frequently accessed resources:

```yaml
spec:
  cache:
    enabled: true
    ttl: "300s"
    maxSize: 100
  
  resourceRef:
    # Cached resource reference
    apiVersion: v1
    kind: ConfigMap
    name: frequently-updated-config
```

## Use Cases and Patterns

### Configuration Backup

Automatically backup all ConfigMaps:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: config-backup
spec:
  repository: "https://github.com/myorg/k8s-backups.git"
  branch: "main"
  message: "Daily configuration backup - {{ .timestamp | date "2006-01-02" }}"
  schedule: "0 2 * * *"  # Daily at 2 AM
  
  resourceRef:
    apiVersion: v1
    kind: ConfigMap
    namespace: "*"  # All namespaces
    selector:
      matchLabels:
        backup: "enabled"
    path: "configs/{{ .metadata.namespace }}/{{ .metadata.name }}.yaml"
```

### Application Deployment Tracking

Track deployment changes:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: deployment-tracker
spec:
  repository: "https://github.com/myorg/deployment-history.git"
  branch: "deployments"
  message: "Deployment update: {{ .metadata.name }} to {{ (index .spec.template.spec.containers 0).image }}"
  
  resourceRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
    namespace: production
    path: "deployments/{{ .timestamp | date "2006/01/02" }}/{{ .metadata.name }}.yaml"
    strategy: "template"
    template: |
      deployment: {{ .metadata.name }}
      namespace: {{ .metadata.namespace }}
      replicas: {{ .spec.replicas }}
      image: {{ (index .spec.template.spec.containers 0).image }}
      timestamp: {{ .timestamp }}
      resourceVersion: {{ .metadata.resourceVersion }}
```

### Multi-Environment Sync

Sync configurations across environments:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: cross-env-sync
spec:
  repository: "https://github.com/myorg/multi-env-configs.git"
  branch: "sync/{{ .cluster.name }}"
  message: "Sync {{ .metadata.name }} from {{ .cluster.name }}"
  
  resourceRef:
    apiVersion: v1
    kind: ConfigMap
    name: shared-config
    namespace: default
    path: "environments/{{ .cluster.name }}/config.yaml"
```

## Best Practices

### Resource Organization

1. **Use meaningful names** for GitCommit resources
2. **Group related resources** using labels and selectors
3. **Organize files** in logical directory structures
4. **Use consistent naming** conventions

### Template Design

1. **Keep templates simple** and readable
2. **Handle missing data** gracefully
3. **Use comments** to document template logic
4. **Test templates** thoroughly

### Performance

1. **Set appropriate reconcile intervals** based on change frequency
2. **Use resource selectors** to limit scope
3. **Enable caching** for frequently accessed resources
4. **Monitor resource usage** and adjust limits

### Security

1. **Use least-privilege RBAC** permissions
2. **Store credentials** securely in secrets
3. **Audit Git operations** regularly
4. **Avoid committing sensitive data**

## Troubleshooting

### Debug GitCommit Issues

```bash
# Check GitCommit status
kubectl get gitcommit -o wide

# Describe resource for events
kubectl describe gitcommit mycommit

# Check controller logs
kubectl logs -n git-change-operator-system -l control-plane=controller-manager

# Validate RBAC permissions
kubectl auth can-i get configmaps --as=system:serviceaccount:git-change-operator-system:git-change-operator-controller-manager
```

### Test Templates

Test templates before applying:

```bash
# Use a temporary GitCommit with dry-run
kubectl apply --dry-run=server -f test-gitcommit.yaml

# Validate template syntax
git-change-operator validate-template --template-file=template.txt --resource-file=resource.yaml
```

## Next Steps

- [PullRequest Resources](pullrequest.md) - Learn about automated PR creation
- [Resource References](resource-references.md) - Deep dive into resource extraction
- [Authentication](authentication.md) - Configure secure Git access
- [Examples](../examples/gitcommit-resourcerefs.md) - See real-world examples

For advanced GitCommit patterns, see our [Advanced Examples](../examples/advanced.md).