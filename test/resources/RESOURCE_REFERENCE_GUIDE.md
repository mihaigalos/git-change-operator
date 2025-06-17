# Resource Reference Feature

The Git Change Operator now supports referencing arbitrary Kubernetes resources and committing their content to Git repositories with flexible output strategies.

## Overview

Instead of specifying static file content, you can now reference existing Kubernetes resources (like Secrets, ConfigMaps, etc.) and have the operator extract and commit their data using various strategies.

## Configuration

### ResourceRef Structure

```yaml
resourceRefs:
- apiVersion: "v1"           # API version of the resource
  kind: "Secret"             # Kind of the resource
  name: "my-secret"          # Name of the resource
  namespace: "default"       # Namespace (optional, defaults to the GitCommit/PullRequest namespace)
  strategy:                  # Output strategy configuration
    type: "dump"             # Output type: dump, fields, or single-field
    path: "secrets/output"   # Base path for the output files
    writeMode: "overwrite"   # Write mode: overwrite or append
    fieldRef:                # Field reference (required for single-field type)
      key: "fieldname"       # Field key to extract
      fileName: "custom.txt" # Custom filename (optional)
```

## Output Strategies

### 1. Dump Strategy

Outputs the entire resource as YAML.

```yaml
strategy:
  type: "dump"
  path: "secrets/my-secret"
  writeMode: "overwrite"
```

**Output**: Creates `secrets/my-secret.yaml` with the complete resource definition.

### 2. Fields Strategy

Extracts all fields from the resource's `data` section as separate files.

```yaml
strategy:
  type: "fields"  
  path: "secrets/individual"
  writeMode: "overwrite"
```

**Output**: Creates separate files for each data field:
- `secrets/individual/alice`
- `secrets/individual/bob`
- `secrets/individual/config.json`

### 3. Single-Field Strategy

Extracts a specific field from the resource.

```yaml
strategy:
  type: "single-field"
  path: "configs/alice-password"
  writeMode: "overwrite"
  fieldRef:
    key: "alice"
    fileName: "password.txt"  # Optional custom filename
```

**Output**: Creates `configs/alice-password/password.txt` with the content of the `alice` field.

## Write Modes

### Overwrite Mode (default)
Replaces the entire file content.

```yaml
writeMode: "overwrite"
```

### Append Mode
Appends content to existing files.

```yaml
writeMode: "append"
```

## Examples

### GitCommit with Secret Dump

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: secret-dump
spec:
  repository: "https://github.com/example/repo.git"
  branch: "main"
  commitMessage: "Add secret configuration"
  authSecretRef: "git-token"
  resourceRefs:
  - apiVersion: "v1"
    kind: "Secret"
    name: "app-config"
    strategy:
      type: "dump"
      path: "config/app-secret"
```

### PullRequest with Multiple Strategies

```yaml
apiVersion: gco.galos.one/v1
kind: PullRequest
metadata:
  name: config-update
spec:
  repository: "https://github.com/example/repo.git"
  baseBranch: "main"
  headBranch: "feature/config-update"
  title: "Update application configuration"
  authSecretRef: "git-token"
  resourceRefs:
  - apiVersion: "v1"
    kind: "Secret"
    name: "app-config"
    strategy:
      type: "fields"
      path: "config/individual"
  - apiVersion: "v1"
    kind: "ConfigMap"
    name: "app-settings"
    strategy:
      type: "single-field"
      path: "config/app.properties"
      fieldRef:
        key: "application.properties"
```

## Supported Resources

The operator can reference any Kubernetes resource that the service account has access to. Common examples:

- **Secrets**: For sensitive configuration data
- **ConfigMaps**: For application configuration
- **Custom Resources**: Any CRD with data fields

## RBAC Requirements

The operator needs appropriate RBAC permissions to access the referenced resources:

```yaml
- apiGroups: ["*"]
  resources: ["*"] 
  verbs: ["get", "list", "watch"]
```

## Testing

Use the example resources in `test/resources/` to test the functionality:

1. Apply the test secret: `kubectl apply -f test/resources/test-commitme-secret.yaml`
2. Apply example GitCommits: `kubectl apply -f test/resources/example-gitcommit-strategies.yaml`
3. Apply example PullRequests: `kubectl apply -f test/resources/example-pullrequest-strategies.yaml`

## Troubleshooting

- **Resource not found**: Ensure the referenced resource exists and the namespace is correct
- **Permission denied**: Check RBAC permissions for the operator service account
- **Field not found**: Verify the field key exists in the resource's data section
- **Invalid API version**: Ensure the apiVersion and kind are correct for the target resource