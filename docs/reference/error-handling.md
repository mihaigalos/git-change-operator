# Error Handling

This guide covers common errors, troubleshooting steps, and best practices for diagnosing issues with the Git Change Operator.

## Common Errors

### Authentication Errors

#### Git Authentication Failed
```
Error: authentication required
```

**Causes:**
- Invalid credentials in referenced Secret
- Incorrect Secret name or namespace
- Missing Secret fields (`username`, `password`)

**Solutions:**
1. Verify Secret exists and contains correct fields:
   ```bash
   kubectl get secret git-credentials -o yaml
   ```

2. Check Secret data format:
   ```yaml
   apiVersion: v1
   kind: Secret
   type: Opaque
   data:
     username: <base64-encoded-username>
     password: <base64-encoded-token>  # Use personal access token for GitHub
   ```

3. Ensure the operator has permission to read the Secret:
   ```bash
   kubectl auth can-i get secrets --as=system:serviceaccount:git-change-operator-system:git-change-operator-controller-manager
   ```

#### GitHub Token Issues
```
Error: 401 Unauthorized - Bad credentials
```

**Solutions:**
- Use a Personal Access Token instead of password
- Ensure token has required permissions: `repo`, `pull_request`
- Check token expiration date
- Verify token scope includes target repository

### Repository Errors

#### Repository Not Found
```
Error: repository not found
```

**Causes:**
- Incorrect repository URL
- Private repository without proper credentials
- Repository has been moved or deleted

**Solutions:**
1. Verify repository URL format:
   ```yaml
   repository:
     url: "https://github.com/user/repo.git"  # Include .git suffix
   ```

2. Check repository accessibility:
   ```bash
   git ls-remote https://github.com/user/repo.git
   ```

#### Branch Not Found
```
Error: branch 'main' not found
```

**Solutions:**
- Verify branch exists in repository
- Check default branch name (might be `master` instead of `main`)
- Create branch if needed or use existing branch name

#### Non-Fast-Forward Push Errors
```
Error: Updates were rejected because the tip of your current branch is behind
```

**Causes:**
- Repository has changes not present in operator's local copy
- Multiple operators writing to same repository simultaneously
- Manual commits made to target branch

**Solutions:**
1. The operator automatically handles this by pulling latest changes before pushing
2. If issues persist, check for conflicting file modifications
3. Consider using different branches for different operators

### Resource Reference Errors

#### Resource Not Found
```
Error: configmap "my-config" not found
```

**Solutions:**
1. Verify resource exists:
   ```bash
   kubectl get configmap my-config -n default
   ```

2. Check namespace specification:
   ```yaml
   resourceReferences:
     - name: "my-config"
       namespace: "correct-namespace"  # Must match actual namespace
   ```

3. Verify API version and kind:
   ```bash
   kubectl api-resources | grep configmap
   ```

#### Field Not Found
```
Error: field "spec.nonexistent" not found in resource
```

**Causes:**
- Invalid field path in `single-field` strategy
- Field doesn't exist in the resource
- Typo in field name

**Solutions:**
1. Inspect resource structure:
   ```bash
   kubectl get configmap my-config -o yaml
   ```

2. Use correct field path:
   ```yaml
   strategy: "single-field"
   field: "data.config.yaml"  # Correct path
   ```

3. Switch to `fields` or `dump` strategy if unsure about field structure

### Permission Errors

#### RBAC Permissions
```
Error: configmaps is forbidden: User "system:serviceaccount:git-change-operator-system:git-change-operator-controller-manager" cannot get resource "configmaps"
```

**Solutions:**
1. Check existing ClusterRole:
   ```bash
   kubectl get clusterrole git-change-operator-manager-role -o yaml
   ```

2. Update ClusterRole to include required permissions:
   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: git-change-operator-manager-role
   rules:
   - apiGroups: [""]
     resources: ["configmaps", "secrets"]
     verbs: ["get", "list", "watch"]
   - apiGroups: ["apps"]
     resources: ["deployments"]
     verbs: ["get", "list", "watch"]
   ```

#### Cross-Namespace Access
```
Error: secrets "my-secret" is forbidden: User cannot get resource "secrets" in API group "" in the namespace "other-namespace"
```

**Solutions:**
1. Use Role/RoleBinding for namespace-specific permissions:
   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: Role
   metadata:
     namespace: other-namespace
     name: git-change-operator-role
   rules:
   - apiGroups: [""]
     resources: ["secrets", "configmaps"]  
     verbs: ["get", "list", "watch"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: RoleBinding
   metadata:
     name: git-change-operator-binding
     namespace: other-namespace
   subjects:
   - kind: ServiceAccount
     name: git-change-operator-controller-manager
     namespace: git-change-operator-system
   roleRef:
     kind: Role
     name: git-change-operator-role
     apiGroup: rbac.authorization.k8s.io
   ```

### File System Errors

#### Invalid File Paths
```
Error: invalid file path "../../../etc/passwd"
```

**Causes:**
- Path traversal attempts (security protection)
- Invalid characters in file paths
- Paths outside repository root

**Solutions:**
- Use relative paths within repository: `config/app.yaml`
- Avoid `../` path traversal
- Use forward slashes `/` even on Windows

#### Large File Issues
```
Error: file size exceeds limit
```

**Solutions:**
- Break large files into smaller chunks
- Use external storage for large binary files
- Consider Git LFS for large file management

## Troubleshooting Steps

### 1. Check Resource Status

```bash
# Check GitCommit status
kubectl get gitcommit my-commit -o yaml

# Look for status conditions
kubectl get gitcommit my-commit -o jsonpath='{.status.conditions}'
```

### 2. Review Controller Logs

```bash
# Get controller pod name
kubectl get pods -n git-change-operator-system

# Check logs
kubectl logs -n git-change-operator-system deployment/git-change-operator-controller-manager

# Follow logs in real-time
kubectl logs -f -n git-change-operator-system deployment/git-change-operator-controller-manager
```

### 3. Validate Configuration

```bash
# Dry-run validation
kubectl apply --dry-run=client -f gitcommit.yaml

# Check YAML syntax
yamllint gitcommit.yaml
```

### 4. Test Resource Access

```bash
# Test resource accessibility
kubectl get configmap my-config -n target-namespace

# Check resource content
kubectl get configmap my-config -o yaml

# Verify field paths
kubectl get configmap my-config -o jsonpath='{.data.config\.yaml}'
```

### 5. Debug Authentication

```bash
# Check Secret content
kubectl get secret git-credentials -o yaml

# Decode Secret values
kubectl get secret git-credentials -o jsonpath='{.data.username}' | base64 -d

# Test git access manually
git ls-remote https://username:token@github.com/user/repo.git
```

## Status Conditions

The operator reports status through Kubernetes conditions:

### Ready Condition
```yaml
status:
  conditions:
  - type: Ready
    status: "True"
    lastTransitionTime: "2023-10-01T10:00:00Z"
    reason: "CommitSuccessful"
    message: "Successfully committed to repository"
```

### Failed Condition
```yaml
status:
  conditions:
  - type: Ready
    status: "False" 
    lastTransitionTime: "2023-10-01T10:00:00Z"
    reason: "AuthenticationFailed"
    message: "Failed to authenticate with git repository"
```

### Common Reasons

| Reason | Meaning | Action |
|--------|---------|---------|
| `CommitSuccessful` | Operation completed successfully | None |
| `AuthenticationFailed` | Git authentication failed | Check credentials |
| `RepositoryNotFound` | Repository doesn't exist or inaccessible | Verify URL and permissions |
| `ResourceNotFound` | Referenced Kubernetes resource not found | Check resource name and namespace |
| `FieldNotFound` | Field path invalid for single-field strategy | Verify field path |
| `PermissionDenied` | Insufficient RBAC permissions | Update ClusterRole/Role |
| `InvalidConfiguration` | Invalid GitCommit specification | Review YAML configuration |

## Monitoring and Alerting

### Metrics

The operator exposes Prometheus metrics for monitoring:

```
# Successful reconciliations
gitchange_operator_reconciliations_total{status="success"}

# Failed reconciliations  
gitchange_operator_reconciliations_total{status="error"}

# Active GitCommit resources
gitchange_operator_active_resources{type="GitCommit"}
```

### Alert Examples

```yaml
# Alert on reconciliation failures
- alert: GitChangeOperatorFailures
  expr: rate(gitchange_operator_reconciliations_total{status="error"}[5m]) > 0
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "Git Change Operator experiencing failures"
    description: "{{ $value }} reconciliation failures per second"

# Alert on authentication issues
- alert: GitChangeOperatorAuthFailures  
  expr: gitchange_operator_reconciliations_total{reason="AuthenticationFailed"} > 0
  labels:
    severity: critical
  annotations:
    summary: "Git Change Operator authentication failing"
    description: "Check git credentials and permissions"
```

## Best Practices

### Error Prevention

1. **Validate configuration before applying:**
   ```bash
   kubectl apply --dry-run=client -f gitcommit.yaml
   ```

2. **Use specific resource references:**
   ```yaml
   resourceReferences:
     - name: "exact-resource-name"
       namespace: "specific-namespace"
       apiVersion: "v1"
       kind: "ConfigMap"
   ```

3. **Test authentication separately:**
   ```bash
   git clone https://github.com/user/repo.git
   ```

4. **Monitor resource changes:**
   ```bash
   kubectl get events --field-selector involvedObject.name=my-gitcommit
   ```

### Recovery Procedures

1. **Reset failed GitCommit:**
   ```bash
   kubectl delete gitcommit my-commit
   kubectl apply -f gitcommit.yaml
   ```

2. **Update credentials:**
   ```bash
   kubectl delete secret git-credentials
   kubectl create secret generic git-credentials \
     --from-literal=username=myuser \
     --from-literal=password=mytoken
   ```

3. **Force reconciliation:**
   ```bash
   kubectl annotate gitcommit my-commit reconcile.gco.galos.one/trigger="$(date)"
   ```

### Log Analysis

Common log patterns to watch for:

```
# Successful operations
"Successfully committed" level=info

# Authentication issues  
"authentication failed" level=error

# Resource access problems
"resource not found" level=error

# Git operation failures
"failed to push" level=error
```

Use log aggregation tools (Fluentd, Logstash) to centralize and analyze operator logs for patterns and trends.