# API Reference & CRD Specification

Complete API documentation and Custom Resource Definition (CRD) specifications for the Git Change Operator.

## API Group and Versions

- **API Group**: `gco.galos.one`
- **Version**: `v1`
- **Scope**: Namespaced

## Custom Resources

The operator provides three custom resources:

1. **GitCommit** - Automated git commits from Kubernetes resources
2. **PullRequest** - Automated pull request creation
3. **GitChangeOperator** - Operator configuration and resource management

## GitChangeOperator Resource

### Overview
The `GitChangeOperator` resource configures the operator itself and manages operator-level resources like metrics services, ServiceMonitor, and Ingress. This CR allows dynamic configuration of the running operator without requiring Helm redeployment.

### Resource Definition

```yaml
apiVersion: gco.galos.one/v1
kind: GitChangeOperator
metadata:
  name: string
  namespace: string
spec:
  replicaCount: int                    # optional - Number of operator replicas
  image:
    repository: string                 # Container image repository
    tag: string                       # Container image tag
    pullPolicy: string                # Image pull policy
  operator:
    leaderElect: boolean              # Enable leader election
    metricsAddr: string               # Metrics server address
    probeAddr: string                 # Health probe address
  rbac:
    create: boolean                   # Create RBAC resources
  serviceAccount:
    create: boolean                   # Create ServiceAccount
    name: string                      # ServiceAccount name
  metrics:
    enabled: boolean                  # Enable metrics
    service:
      type: string                    # Service type (ClusterIP, NodePort, etc.)
      port: int                       # Service port
    serviceMonitor:
      enabled: boolean                # Enable ServiceMonitor
      name: string                    # ServiceMonitor name
      interval: string                # Scrape interval
      scrapeTimeout: string           # Scrape timeout
      labels: map[string]string       # Additional labels
      annotations: map[string]string  # Additional annotations
  ingress:
    enabled: boolean                  # Enable Ingress
    name: string                      # Ingress name
    ingressClassName: string          # IngressClass to use
    labels: map[string]string         # Additional labels
    annotations: map[string]string    # Additional annotations
    hosts: []IngressHost              # Host configurations
    tls: []IngressTLS                 # TLS configurations
  crds:
    install: boolean                  # Install CRDs
status:
  phase: string                       # Current phase (Ready, etc.)
  observedGeneration: int             # Last observed generation
```

### Complete Example

```yaml
apiVersion: gco.galos.one/v1
kind: GitChangeOperator
metadata:
  name: git-change-operator
  namespace: git-change-operator-system
spec:
  replicaCount: 1
  image:
    repository: ghcr.io/mihaigalos/git-change-operator
    tag: latest
    pullPolicy: IfNotPresent
  operator:
    leaderElect: true
    metricsAddr: ":8080"
    probeAddr: ":8081"
  rbac:
    create: true
  serviceAccount:
    create: true
  metrics:
    enabled: true
    service:
      type: ClusterIP
      port: 8080
    serviceMonitor:
      enabled: true
      name: git-change-operator-metrics
      interval: "30s"
      scrapeTimeout: "10s"
      labels:
        prometheus: kube-prometheus
      annotations:
        monitoring/scrape: "true"
  ingress:
    enabled: true
    name: git-change-operator-ingress
    ingressClassName: nginx
    labels:
      app: git-change-operator
    annotations:
      cert-manager.io/cluster-issuer: letsencrypt-prod
    hosts:
      - host: git-change-operator.example.com
        paths:
          - path: /metrics
            pathType: Prefix
            backend:
              service:
                name: git-change-operator-metrics-service
                port:
                  number: 8080
    tls:
      - hosts:
          - git-change-operator.example.com
        secretName: git-change-operator-tls
```

### Managed Resources

When a GitChangeOperator CR is created, the operator dynamically manages the following resources:

- **Service** - Metrics service (when `metrics.enabled: true`)
- **ServiceMonitor** - Prometheus ServiceMonitor (when `metrics.serviceMonitor.enabled: true`)
- **Ingress** - Ingress resource (when `ingress.enabled: true`)

These resources are owned by the GitChangeOperator CR and will be automatically deleted when the CR is removed.

### Configuration Notes

- The GitChangeOperator CR is typically created by Helm during installation
- Changes to the CR spec are reconciled automatically by the operator
- Bootstrap resources (RBAC, CRDs, Deployment) are still managed via Helm/Kustomize
- Runtime-configurable resources (Service, ServiceMonitor, Ingress) are managed by the operator itself

## GitCommit Resource

### Overview
The `GitCommit` resource enables automated git commits by reading data from Kubernetes resources and writing files to a git repository.

### Resource Definition

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: string
  namespace: string  # optional, defaults to "default"
spec:
  repository: string               # required - Git repository URL
  branch: string                  # optional - Target branch (default: "main")
  authSecretRef: string          # required - Authentication secret name
  commitMessage: string          # required - Git commit message
  files: []FileSpec             # optional - Static files to commit
  resourceReferences: []ResourceReferenceSpec  # optional - Kubernetes resource references
  writeMode: string             # optional - "overwrite" (default) or "append"
  encryption: EncryptionSpec   # optional - File encryption configuration
  schedule: string             # optional - Cron schedule for recurring commits
  suspend: boolean             # optional - Suspend scheduled execution
  maxExecutionHistory: int     # optional - Number of execution records to keep (default: 10)
status:
  conditions: []Condition      # Status conditions
  lastCommitHash: string      # Last successful commit SHA
  nextScheduledTime: string   # Next scheduled execution time (RFC3339)
  lastScheduledTime: string   # Last scheduled execution time (RFC3339)
```

### Complete Example

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: example-gitcommit
  namespace: default
spec:
  repository: "https://github.com/user/repo.git"
  branch: "main"
  authSecretRef: "git-credentials"
  commitMessage: "Automated commit from Kubernetes cluster"
  
  # Static files
  files:
    - path: "config/app-config.yaml"
      content: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: static-config
        data:
          key: value
          
  # Resource references
  resourceReferences:
    - name: "my-configmap"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "dump"
      output:
        path: "exported/configmap.yaml"
        
  # Encryption configuration
  encryption:
    enabled: true
    fileExtension: ".encrypted"
    recipients:
      - type: ssh
        secretRef:
          name: ssh-keys
          key: id_rsa.pub
      - type: yubikey
        secretRef:
          name: yubikey-piv
          key: public-key
          
  writeMode: "overwrite"
```

### Field Specifications

#### spec.repository
| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `repository` | string | ✓ | Git repository URL (HTTPS format) | - |

**Example:**
```yaml
repository: "https://github.com/user/repo.git"
```

#### spec.branch
| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `branch` | string | ✗ | Target branch for commits | `"main"` |

**Example:**
```yaml
branch: "development"
```

#### spec.authSecretRef
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `authSecretRef` | string | ✓ | Name of Kubernetes Secret containing git credentials |

The referenced Secret must contain:
```yaml
apiVersion: v1
kind: Secret
type: Opaque
data:
  username: <base64-encoded-username>
  password: <base64-encoded-token-or-password>
```

**Example:**
```yaml
authSecretRef: "git-credentials"
```

#### spec.commitMessage
| Field | Type | Required | Description | Format |
|-------|------|----------|-------------|--------|
| `commitMessage` | string | ✓ | Git commit message | Free text |

**Example:**
```yaml
commitMessage: "Automated update from Kubernetes cluster - {{ .Timestamp }}"
```

#### spec.files
Array of static files to include in the commit.

```yaml
files:
  - path: string     # required - File path in repository
    content: string  # required - File content
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | ✓ | Relative path in Git repository |
| `content` | string | ✓ | File content (supports multiline YAML) |

**Examples:**
```yaml
files:
  # Simple configuration file
  - path: "config/app.properties"
    content: |
      server.port=8080
      debug=true
      
  # JSON configuration  
  - path: "config/settings.json"
    content: |
      {
        "environment": "production",
        "features": {
          "authentication": true
        }
      }
```

#### spec.resourceReferences
Array of Kubernetes resource references for dynamic content extraction.

```yaml
resourceReferences:
  - name: string          # required - Resource name
    apiVersion: string    # required - Resource API version
    kind: string         # required - Resource kind
    namespace: string    # optional - Resource namespace (required for namespaced resources)
    strategy: string     # required - Extraction strategy
    field: string        # optional - Field path (required for single-field strategy)
    output: OutputSpec   # required - Output configuration
```

| Field | Type | Required | Description | Values |
|-------|------|----------|-------------|--------|
| `name` | string | ✓ | Name of Kubernetes resource | - |
| `apiVersion` | string | ✓ | API version of resource | e.g. `"v1"`, `"apps/v1"` |
| `kind` | string | ✓ | Resource kind | e.g. `"ConfigMap"`, `"Secret"` |
| `namespace` | string | ✗ | Resource namespace | Required for namespaced resources |
| `strategy` | string | ✓ | Data extraction strategy | `"dump"`, `"fields"`, `"single-field"` |
| `field` | string | ✗ | Field path for extraction | Required when `strategy: "single-field"` |
| `output` | OutputSpec | ✓ | Output configuration | - |

##### Extraction Strategies

| Strategy | Description | Use Case | Output |
|----------|-------------|----------|--------|
| `dump` | Export entire resource as YAML | Resource backup, migration | Complete YAML file |
| `fields` | Extract all data fields as separate files | Configuration management | Multiple files |
| `single-field` | Extract specific field | Credential extraction | Single file |

##### Field Path Syntax

For `single-field` strategy, use dot notation:

```yaml
# Simple field
field: "password"

# Nested field
field: "spec.database.host"

# Array element
field: "data.config.yaml"
```

##### OutputSpec

```yaml
output:
  path: string   # required - Output path in repository
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | ✓ | Relative path in Git repository where extracted data will be written |

**Examples:**
```yaml
resourceReferences:
  # Dump entire ConfigMap as YAML
  - name: "app-config"
    apiVersion: "v1"
    kind: "ConfigMap"
    namespace: "production"
    strategy: "dump"
    output:
      path: "config/app-config.yaml"
      
  # Extract all fields as separate files
  - name: "database-secret"
    apiVersion: "v1"
    kind: "Secret"
    namespace: "production"
    strategy: "fields"
    output:
      path: "secrets/database/"
      
  # Extract single field
  - name: "database-secret"
    apiVersion: "v1"
    kind: "Secret"
    namespace: "production"
    strategy: "single-field"
    field: "password"
    output:
      path: "credentials/db-password.txt"
```

#### spec.encryption
Optional configuration for encrypting files before committing to git using age encryption.

```yaml
encryption:
  enabled: boolean           # required - Enable/disable encryption
  fileExtension: string     # optional - Custom file extension (default: ".age")
  recipients: []RecipientSpec  # optional - List of encryption recipients
```

| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `enabled` | boolean | ✓ | Enable/disable encryption | - |
| `fileExtension` | string | ✗ | File extension for encrypted files | `".age"` |
| `recipients` | []RecipientSpec | ✗ | List of encryption recipients | `[]` |

##### RecipientSpec

```yaml
recipients:
  - type: string        # required - Recipient type
    secretRef:         # required - Secret reference
      name: string     # required - Secret name
      key: string      # optional - Key within Secret
```

| Field | Type | Required | Description | Values |
|-------|------|----------|-------------|--------|
| `type` | string | ✓ | Recipient type | `"age"`, `"ssh"`, `"passphrase"`, `"yubikey"` |
| `secretRef.name` | string | ✓ | Name of Kubernetes Secret containing the recipient | - |
| `secretRef.key` | string | ✗ | Key within the Secret | Defaults to appropriate key for type |

**Recipient Types:**

| Type | Description | Secret Key Format | Use Case |
|------|-------------|------------------|----------|
| `age` | Age encryption public key | age1... format | Software-based encryption |
| `ssh` | SSH public key | ssh-rsa/ssh-ed25519 format | Team collaboration |
| `passphrase` | Password-based encryption | Plain text passphrase | Simple shared secrets |
| `yubikey` | YubiKey PIV public key | PIV public key format | Hardware security |

**Example:**
```yaml
encryption:
  enabled: true
  fileExtension: ".encrypted"
  recipients:
    # Age key recipient
    - type: age
      secretRef:
        name: age-keys
        key: public-key
    
    # SSH key recipient
    - type: ssh
      secretRef:
        name: ssh-keys
        key: id_rsa.pub
    
    # Passphrase recipient
    - type: passphrase
      secretRef:
        name: passwords
        key: encryption-passphrase
    
    # YubiKey recipient (hardware security)
    - type: yubikey
      secretRef:
        name: yubikey-piv
        key: public-key
```

#### spec.writeMode
| Field | Type | Required | Description | Values | Default |
|-------|------|----------|-------------|--------|---------|
| `writeMode` | string | ✗ | File writing behavior | `"overwrite"`, `"append"` | `"overwrite"` |

| Value | Description |
|-------|-------------|
| `overwrite` | Replace file content completely |
| `append` | Add content to end of existing file |

#### spec.schedule
| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `schedule` | string | ✗ | Cron expression for recurring commits | - |

Supports standard cron syntax (5 fields) and special descriptors:

**Cron Syntax:** `minute hour day month weekday`

```yaml
schedule: "0 2 * * *"        # Daily at 2 AM
schedule: "*/15 * * * *"     # Every 15 minutes
schedule: "0 */6 * * *"      # Every 6 hours
schedule: "0 9 * * MON"      # Every Monday at 9 AM
```

**Special Descriptors:**
```yaml
schedule: "@hourly"   # Every hour
schedule: "@daily"    # Every day at midnight
schedule: "@weekly"   # Every Sunday at midnight
schedule: "@monthly"  # First day of month at midnight
```

**Example:**
```yaml
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  suspend: false
  maxExecutionHistory: 10
```

#### spec.suspend
| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `suspend` | boolean | ✗ | Suspend scheduled execution | `false` |

When set to `true`, prevents scheduled executions. Set to `false` to resume.

#### spec.maxExecutionHistory
| Field | Type | Required | Description | Default | Range |
|-------|------|----------|-------------|---------|-------|
| `maxExecutionHistory` | int | ✗ | Number of execution records to keep | `10` | 1-100 |

## PullRequest Resource

### Overview
The `PullRequest` resource creates GitHub pull requests with files generated from Kubernetes resources.

### Resource Definition

```yaml
apiVersion: gco.galos.one/v1
kind: PullRequest
metadata:
  name: string
  namespace: string  # optional, defaults to "default"
spec:
  repository: string              # required - GitHub repository URL
  baseBranch: string             # optional - Base branch for PR (default: "main")
  headBranch: string             # optional - Head branch name (auto-generated if not specified)
  authSecretRef: string          # required - GitHub authentication secret
  title: string                  # required - Pull request title
  body: string                   # optional - Pull request description
  files: []FileSpec             # optional - Static files to include
  resourceReferences: []ResourceReferenceSpec  # optional - Kubernetes resource references
  writeMode: string             # optional - "overwrite" (default) or "append"
  encryption: EncryptionSpec   # optional - File encryption configuration
status:
  conditions: []Condition      # Status conditions
  pullRequestURL: string      # URL of created pull request
```

### Complete Example

```yaml
apiVersion: gco.galos.one/v1
kind: PullRequest
metadata:
  name: example-pr
  namespace: default
spec:
  repository: "https://github.com/user/repo.git"
  baseBranch: "main"
  authSecretRef: "github-token"
  title: "Automated configuration update"
  body: |
    This pull request contains automated updates from the Kubernetes cluster.
    
    ## Changes
    - Updated application configuration
    - Exported production secrets (encrypted)
    
    ## Review Notes
    Please verify configuration values before merging.
    
  # Files and encryption (same as GitCommit)
  files:
    - path: "config/metadata.yaml"
      content: |
        generated_at: "2023-10-01T10:00:00Z"
        source: "kubernetes-cluster"
        
  resourceReferences:
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "production"
      strategy: "dump"
      output:
        path: "config/app-config.yaml"
        
  encryption:
    enabled: true
    recipients:
      - type: ssh
        secretRef:
          name: ssh-keys
          key: id_rsa.pub
```

### Field Specifications

#### spec.repository
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `repository` | string | ✓ | GitHub repository URL (HTTPS format) |

#### spec.baseBranch
| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `baseBranch` | string | ✗ | Base branch for the pull request | `"main"` |

#### spec.headBranch
| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `headBranch` | string | ✗ | Head branch name for the pull request | Auto-generated with timestamp |

#### spec.authSecretRef
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `authSecretRef` | string | ✓ | Name of Kubernetes Secret containing GitHub token |

The referenced Secret must contain:
```yaml
apiVersion: v1
kind: Secret
type: Opaque
data:
  token: <base64-encoded-github-token>
```

#### spec.title
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | ✓ | Pull request title |

#### spec.body
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `body` | string | ✗ | Pull request description (supports Markdown) |

#### Additional Fields

PullRequest resources support the same field specifications as GitCommit for:
- `spec.files` - Static files to include in the pull request
- `spec.resourceReferences` - Kubernetes resource references 
- `spec.encryption` - File encryption configuration
- `spec.writeMode` - File writing behavior

## Status Fields

Both GitCommit and PullRequest resources include status information:

### Common Status Fields

```yaml
status:
  conditions:
    - type: string              # Condition type
      status: string            # True, False, Unknown
      reason: string            # Machine-readable reason
      message: string           # Human-readable message
      lastTransitionTime: string # RFC3339 timestamp
  phase: string                 # Current phase (Pending, Processing, Completed, Failed)
```

### GitCommit Status

```yaml
status:
  lastCommitHash: "abc123..."   # SHA of the last successful commit
  repositoryURL: "https://github.com/user/repo/commit/abc123"
```

### PullRequest Status

```yaml
status:
  pullRequestURL: "https://github.com/user/repo/pull/123"  # URL of created PR
  headBranch: "auto-update-20231001-100000"               # Generated branch name
```

## Validation Rules

### Required Fields Validation
- All required fields must be present and non-empty
- String fields cannot be empty when required
- Arrays can be empty but not null

### Format Validation
- Repository URLs must be valid HTTPS URLs
- Field paths must use valid dot notation
- File paths must be relative (no leading `/`, no `../`)
- Encryption recipient types must be valid

### Business Logic Validation
- Referenced Secrets must exist and be accessible
- Referenced Kubernetes resources must exist when strategies are applied
- Field paths must exist in target resources for `single-field` strategy
- Output paths must be valid for the target repository

### Security Validation
- Path traversal protection (no `../` sequences)
- File paths restricted to repository boundaries
- Sensitive data handling in logs (credentials are redacted)

## API Evolution

### Backward Compatibility
- Field additions are backward compatible
- Optional fields may be added in future versions
- Required fields will not be removed within the same API version

### Version Support
- `v1` is the current stable version
- Future versions (`v2`, etc.) may introduce breaking changes
- Migration guides will be provided for major version upgrades

### Deprecated Features
Currently no deprecated features. Deprecation notices will be provided at least one version before removal.