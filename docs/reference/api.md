# Full API Reference

Complete API documentation for the Git Change Operator Custom Resource Definitions (CRDs).

## API Group and Versions

- **API Group**: `gco.galos.one`
- **Version**: `v1`
- **Scope**: Namespaced

## GitCommit Resource

### Resource Definition

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: string
  namespace: string  # optional, defaults to "default"
spec:
  repository: RepositorySpec
  auth: AuthSpec
  commit: CommitSpec
  files: []FileSpec           # optional
  resourceReferences: []ResourceReferenceSpec  # optional
  writeMode: string           # optional, "overwrite" (default) or "append"
status:
  conditions: []Condition
  lastCommitHash: string      # optional
```

### RepositorySpec

Git repository configuration.

```yaml
repository:
  url: string      # required - Git repository URL (HTTPS)
  branch: string   # optional - Target branch (default: "main")
```

| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `url` | string | ✓ | Git repository URL (HTTPS format) | - |
| `branch` | string | ✗ | Target branch for commits | `"main"` |

**Examples:**
```yaml
# Basic repository
repository:
  url: "https://github.com/user/repo.git"

# Custom branch
repository:
  url: "https://github.com/user/config.git"
  branch: "development"
```

### AuthSpec

Authentication configuration for Git operations.

```yaml
auth:
  secretName: string   # required - Name of Secret containing credentials
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `secretName` | string | ✓ | Name of Kubernetes Secret containing git credentials |

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
auth:
  secretName: "git-credentials"
```

### CommitSpec

Git commit configuration.

```yaml
commit:
  author: string   # required - Git commit author
  message: string  # required - Commit message
```

| Field | Type | Required | Description | Format |
|-------|------|----------|-------------|--------|
| `author` | string | ✓ | Git commit author | `"Name <email@domain.com>"` |
| `message` | string | ✓ | Git commit message | Free text |

**Example:**
```yaml
commit:
  author: "Git Change Operator <operator@example.com>"
  message: "Automated update from Kubernetes cluster"
```

### FileSpec

Static file content specification.

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

### ResourceReferenceSpec

Kubernetes resource reference for dynamic content extraction.

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

#### Extraction Strategies

| Strategy | Description | Use Case | Output |
|----------|-------------|----------|--------|
| `dump` | Export entire resource as YAML | Resource backup, migration | Complete YAML file |
| `fields` | Extract all data fields as separate files | Configuration management | Multiple files |
| `single-field` | Extract specific field | Credential extraction | Single file |

#### Field Path Syntax

For `single-field` strategy, use dot notation:

```yaml
# Simple field
field: "password"

# Nested field
field: "spec.database.host"

# Array element
field: "data.config.yaml"
```

### OutputSpec

Output configuration for resource references.

```yaml
output:
  path: string   # required - Output path in repository
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | ✓ | Relative path in Git repository |

**Path Behavior by Strategy:**
- `dump`: Path to output file (e.g., `"exported/resource.yaml"`)
- `fields`: Directory path (e.g., `"config/"`) - files created for each field
- `single-field`: Full file path (e.g., `"secrets/password.txt"`)

### Status

GitCommit resource status information.

```yaml
status:
  conditions:
    - type: string
      status: string
      lastTransitionTime: string
      reason: string
      message: string
  lastCommitHash: string
```

#### Conditions

| Type | Status | Reason | Description |
|------|--------|--------|-------------|
| `Ready` | `True` | `CommitSuccessful` | Operation completed successfully |
| `Ready` | `False` | `AuthenticationFailed` | Git authentication failed |
| `Ready` | `False` | `RepositoryNotFound` | Repository not accessible |
| `Ready` | `False` | `ResourceNotFound` | Referenced Kubernetes resource not found |
| `Ready` | `False` | `FieldNotFound` | Field path invalid |
| `Ready` | `False` | `PermissionDenied` | Insufficient permissions |

## PullRequest Resource

### Resource Definition

```yaml
apiVersion: gco.galos.one/v1
kind: PullRequest
metadata:
  name: string
  namespace: string
spec:
  repository: RepositorySpec
  auth: AuthSpec  
  pullRequest: PullRequestSpec
  files: []FileSpec           # optional
  resourceReferences: []ResourceReferenceSpec  # optional
  writeMode: string           # optional, "overwrite" (default) or "append"
status:
  conditions: []Condition
  pullRequestURL: string      # optional
```

### PullRequestSpec

GitHub pull request configuration.

```yaml
pullRequest:
  title: string         # required - PR title
  body: string         # optional - PR description  
  branchPrefix: string # optional - Branch name prefix
  baseBranch: string   # optional - Target branch for PR
```

| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `title` | string | ✓ | Pull request title | - |
| `body` | string | ✗ | Pull request description/body | `""` |
| `branchPrefix` | string | ✗ | Prefix for auto-generated branch names | `"git-change-operator"` |
| `baseBranch` | string | ✗ | Base branch to create PR against | `"main"` |

**Branch Naming:** Auto-generated branches use format: `{branchPrefix}-{timestamp}`

**Example:**
```yaml
pullRequest:
  title: "Automated configuration update"
  body: |
    This pull request contains automated updates from the Kubernetes cluster.
    
    Changes include:
    - Updated application configuration
    - New environment variables
    - Security credential rotation
  branchPrefix: "config-update"
  baseBranch: "main"
```

### Authentication for PullRequest

PullRequest resources require GitHub authentication:

```yaml
apiVersion: v1
kind: Secret
type: Opaque
data:
  token: <base64-encoded-github-token>
```

**Required Token Permissions:**
- `repo` - Repository access
- `pull_request` - Create and manage pull requests

## Complete Examples

### GitCommit with Multiple Strategies

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: comprehensive-commit
  namespace: default
spec:
  repository:
    url: "https://github.com/user/config-repo.git"
    branch: "main"
    
  auth:
    secretName: "git-credentials"
    
  commit:
    author: "Kubernetes Operator <k8s@example.com>"
    message: "Automated update: configuration and secrets"
    
  writeMode: "overwrite"
  
  files:
    - path: "static/timestamp.txt"
      content: "Last updated: 2023-10-01T10:00:00Z"
      
  resourceReferences:
    # Complete resource backup
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "production"
      strategy: "dump"
      output:
        path: "backups/app-config.yaml"
        
    # Individual configuration files
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "production"
      strategy: "fields"
      output:
        path: "config/"
        
    # Extract database password
    - name: "db-credentials"
      apiVersion: "v1"
      kind: "Secret"
      namespace: "production"
      strategy: "single-field"
      field: "password"
      output:
        path: "secrets/db-password"
        
    # Custom resource export
    - name: "my-app"
      apiVersion: "apps.example.com/v1"
      kind: "Application"
      namespace: "default"
      strategy: "single-field"
      field: "spec.configuration"
      output:
        path: "apps/my-app-config.json"
```

### PullRequest Example

```yaml
apiVersion: gco.galos.one/v1
kind: PullRequest
metadata:
  name: config-update-pr
  namespace: default
spec:
  repository:
    url: "https://github.com/user/config-repo.git"
    
  auth:
    secretName: "github-token"
    
  pullRequest:
    title: "Automated Configuration Update - $(date)"
    body: |
      ## Automated Update from Kubernetes
      
      This PR contains configuration updates exported from the production cluster.
      
      ### Changes:
      - Application configuration refresh
      - Database connection settings
      - Environment-specific variables
      
      ### Review Notes:
      - All changes are automatically generated
      - Verify configuration values before merging
      - Test in staging environment if needed
      
    branchPrefix: "auto-config-update"
    baseBranch: "main"
    
  writeMode: "overwrite"
  
  files:
    - path: "deployment/metadata.yaml"
      content: |
        generated_at: "2023-10-01T10:00:00Z"
        source: "kubernetes-cluster-production"
        operator_version: "v1.0.0"
        
  resourceReferences:
    - name: "production-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "production"
      strategy: "fields"
      output:
        path: "config/production/"
```

## Validation Rules

### Required Fields Validation
- All required fields must be present
- String fields cannot be empty when required
- Arrays can be empty but not null

### Format Validation
- Repository URLs must be valid HTTPS URLs
- Author must follow format: `"Name <email>"`
- Field paths must use valid dot notation
- File paths must be relative (no leading `/`, no `../`)

### Business Logic Validation
- Referenced Secrets must exist and be accessible
- Referenced Kubernetes resources must exist when `strategy` is applied
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
- Required fields will not be removed in the same API version

### Version Support
- `v1` is the current stable version
- Future versions (`v2`, etc.) may introduce breaking changes
- Migration guides will be provided for major version upgrades

### Deprecated Features
Currently no deprecated features. Deprecation notices will be provided at least one version before removal.