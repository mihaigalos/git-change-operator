# CRD Specification

This page provides the complete Custom Resource Definition (CRD) specifications for the Git Change Operator.

## GitCommit

### Overview
The `GitCommit` resource enables automated git commits by reading data from Kubernetes resources and writing files to a git repository.

### Schema

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: example-gitcommit
spec:
  # Git repository configuration
  repository:
    url: "https://github.com/user/repo.git"
    branch: "main"  # optional, defaults to "main"
    
  # Authentication (required)
  auth:
    secretName: "git-credentials"
    
  # Commit configuration
  commit:
    author: "Git Change Operator <operator@example.com>"
    message: "Automated commit from Kubernetes"
    
  # Files to create/update
  files:
    - path: "config/app-config.yaml"
      content: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: static-config
        data:
          key: value
          
  # Resource references (optional)
  resourceReferences:
    - name: "my-configmap"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "dump"  # dump, fields, or single-field
      output:
        path: "exported/configmap.yaml"
        
    - name: "my-secret"
      apiVersion: "v1" 
      kind: "Secret"
      namespace: "default"
      strategy: "fields"
      output:
        path: "secrets/"
        
    - name: "database-password"
      apiVersion: "v1"
      kind: "Secret"
      namespace: "default"
      strategy: "single-field"
      field: "password"
      output:
        path: "db/password.txt"
        
  # Write mode (optional)
  writeMode: "overwrite"  # overwrite (default) or append
```

### Field Reference

#### spec.repository
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | ✓ | Git repository URL (HTTP/HTTPS) |
| `branch` | string | ✗ | Target branch (default: "main") |

#### spec.auth
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `secretName` | string | ✓ | Name of Secret containing git credentials |

The referenced Secret should contain:
```yaml
apiVersion: v1
kind: Secret
type: Opaque
data:
  username: <base64-encoded-username>
  password: <base64-encoded-token-or-password>
```

#### spec.commit
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `author` | string | ✓ | Git commit author in format "Name <email>" |
| `message` | string | ✓ | Commit message |

#### spec.files
Array of static files to include in the commit.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | ✓ | File path in repository |
| `content` | string | ✓ | File content |

#### spec.encryption
Optional configuration for encrypting files before committing to git.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | boolean | ✓ | Enable/disable encryption |
| `fileExtension` | string | ✗ | File extension for encrypted files (default: ".age") |
| `recipients` | array | ✗ | List of encryption recipients |

#### spec.encryption.recipients
Array of encryption recipients (keys/passphrases) used for file encryption.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | ✓ | Recipient type: "age", "ssh", or "passphrase" |
| `value` | string | ✗ | Direct recipient value (not recommended for secrets) |
| `secretRef` | object | ✗ | Reference to Kubernetes Secret containing the recipient |

#### spec.encryption.recipients.secretRef
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | ✓ | Name of the Secret |
| `key` | string | ✗ | Key within the Secret (default: uses recipient type as key) |

**Encryption Example:**
```yaml
spec:
  encryption:
    enabled: true
    fileExtension: ".encrypted"
    recipients:
      - type: age
        secretRef:
          name: age-keys
          key: public-key
      - type: ssh
        secretRef:
          name: ssh-keys
          key: id_rsa.pub
      - type: passphrase
        secretRef:
          name: passwords
          key: encryption-passphrase
```

#### spec.resourceReferences
Array of Kubernetes resource references to include in the commit.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | ✓ | Name of the Kubernetes resource |
| `apiVersion` | string | ✓ | API version of the resource |
| `kind` | string | ✓ | Kind of the resource |
| `namespace` | string | ✗ | Namespace (required for namespaced resources) |
| `strategy` | string | ✓ | Extraction strategy: "dump", "fields", or "single-field" |
| `field` | string | ✗ | Field name (required for "single-field" strategy) |
| `output.path` | string | ✓ | Output path for the extracted data |

#### spec.writeMode
| Value | Description |
|-------|-------------|
| `overwrite` | Replace file content (default) |
| `append` | Append to existing file content |

## PullRequest

### Overview
The `PullRequest` resource creates GitHub pull requests with files generated from Kubernetes resources.

### Schema

```yaml
apiVersion: gco.galos.one/v1
kind: PullRequest
metadata:
  name: example-pr
spec:
  # Git repository configuration
  repository:
    url: "https://github.com/user/repo.git"
    baseBranch: "main"  # branch to create PR against
    
  # Authentication (required)
  auth:
    secretName: "github-token"
    
  # Pull request configuration
  pullRequest:
    title: "Automated update from Kubernetes"
    body: |
      This pull request was automatically generated by the Git Change Operator.
      
      Changes include:
      - Updated configuration from cluster state
    branchPrefix: "auto-update"  # optional, defaults to "git-change-operator"
    
  # Files and resource references (same as GitCommit)
  files: []
  resourceReferences: []
  writeMode: "overwrite"
  
  # Encryption (same as GitCommit)
  encryption:
    enabled: true
    recipients:
      - type: ssh
        secretRef:
          name: ssh-keys
          key: id_rsa.pub
```

### Field Reference

#### spec.repository
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | ✓ | GitHub repository URL |
| `baseBranch` | string | ✗ | Base branch for PR (default: "main") |

#### spec.auth
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `secretName` | string | ✓ | Name of Secret containing GitHub token |

The referenced Secret should contain:
```yaml
apiVersion: v1
kind: Secret
type: Opaque
data:
  token: <base64-encoded-github-token>
```

#### spec.pullRequest
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | ✓ | Pull request title |
| `body` | string | ✗ | Pull request description |
| `branchPrefix` | string | ✗ | Prefix for auto-generated branch names |

The operator creates unique branch names using the format: `{branchPrefix}-{timestamp}`

#### Additional Fields

PullRequest resources support the same field specifications as GitCommit for:
- `spec.files` - Static files to include in the pull request
- `spec.resourceReferences` - Kubernetes resource references 
- `spec.encryption` - File encryption configuration
- `spec.writeMode` - File write mode behavior

Refer to the GitCommit specification above for detailed field documentation.

## Status Fields

Both GitCommit and PullRequest resources include status information:

```yaml
status:
  conditions:
    - type: "Ready"
      status: "True"
      lastTransitionTime: "2023-10-01T10:00:00Z"
      reason: "CommitSuccessful"
      message: "Successfully created commit abc123"
  lastCommitHash: "abc123def456"  # GitCommit only
  pullRequestURL: "https://github.com/user/repo/pull/123"  # PullRequest only
```

### Condition Types

| Type | Description |
|------|-------------|
| `Ready` | Resource is successfully processed |
| `Failed` | Resource processing failed |
| `InProgress` | Resource is being processed |