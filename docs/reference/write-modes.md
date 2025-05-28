# Write Modes

Write modes control how the Git Change Operator handles existing file content when writing files to your Git repository.

## Overview

The Git Change Operator supports two write modes:

1. **[Overwrite Mode](#overwrite-mode)** - Replace existing file content (default)
2. **[Append Mode](#append-mode)** - Add content to existing files

## Overwrite Mode

**Default behavior** - Replaces the entire content of existing files.

### Configuration

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: overwrite-example
spec:
  # ... repository and auth config
  writeMode: "overwrite"  # This is the default
  
  files:
    - path: "config/app.properties"
      content: |
        server.port=8080
        debug=true
```

### Behavior

- **New files**: Creates the file with specified content
- **Existing files**: Completely replaces existing content
- **Empty content**: Creates or overwrites with empty file

### Use Cases

- **Configuration updates**: Replace entire config files
- **Resource exports**: Clean exports of Kubernetes resources  
- **Template generation**: Generate fresh templates
- **Documentation**: Update complete documentation files

### Example

**Initial file content** (`config/app.properties`):
```properties
server.port=3000
database.url=localhost
debug=false
```

**After GitCommit with overwrite mode**:
```properties
server.port=8080
debug=true
```

*Note: The `database.url` line is removed because overwrite mode replaces the entire content.*

## Append Mode

Adds new content to the end of existing files.

### Configuration

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: append-example
spec:
  # ... repository and auth config  
  writeMode: "append"
  
  files:
    - path: "logs/deployment.log"
      content: |
        2023-10-01 10:00:00 - Deployment started
        2023-10-01 10:05:00 - Application healthy
```

### Behavior

- **New files**: Creates the file with specified content
- **Existing files**: Adds content to the end of the file
- **Multiple appends**: Each reconciliation adds more content

### Use Cases

- **Log aggregation**: Collect logs from multiple sources
- **Audit trails**: Build chronological records
- **Configuration merging**: Add settings to existing configs
- **Report generation**: Accumulate data over time

### Example

**Initial file content** (`logs/deployment.log`):
```
2023-09-30 15:30:00 - Previous deployment completed
2023-09-30 15:35:00 - System stable
```

**After GitCommit with append mode**:
```
2023-09-30 15:30:00 - Previous deployment completed  
2023-09-30 15:35:00 - System stable
2023-10-01 10:00:00 - Deployment started
2023-10-01 10:05:00 - Application healthy
```

## Resource References and Write Modes

Write modes apply to both static files and resource references:

### Overwrite with Resource References

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: config-export
spec:
  # ... repository and auth config
  writeMode: "overwrite"
  
  resourceReferences:
    - name: "app-config"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "dump"
      output:
        path: "exported/app-config.yaml"
```

**Result**: Replaces `exported/app-config.yaml` with current ConfigMap state.

### Append with Resource References

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit  
metadata:
  name: log-collection
spec:
  # ... repository and auth config
  writeMode: "append"
  
  resourceReferences:
    - name: "application-logs"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "single-field"
      field: "latest.log"
      output:
        path: "aggregated/all-logs.txt"
```

**Result**: Adds the log content to the end of `aggregated/all-logs.txt`.

## Advanced Examples

### Mixed Content Types

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: mixed-content
spec:
  repository:
    url: "https://github.com/user/config-repo.git"
  auth:
    secretName: "git-credentials"
  commit:
    author: "Operator <operator@example.com>"
    message: "Update configurations and logs"
    
  writeMode: "append"
  
  files:
    # Static timestamp
    - path: "activity/timestamps.log"
      content: |
        {{ .Timestamp }} - GitCommit reconciliation started
        
  resourceReferences:
    # Append ConfigMap data to log file
    - name: "audit-log"
      apiVersion: "v1"
      kind: "ConfigMap"
      namespace: "default"
      strategy: "single-field"  
      field: "audit.log"
      output:
        path: "activity/timestamps.log"  # Same file as above
```

### Per-File Write Mode Control

Currently, write mode applies to all files in a GitCommit. For different behaviors, use separate GitCommit resources:

```yaml
# Overwrite configuration files
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: config-update
spec:
  # ... repository and auth config
  writeMode: "overwrite"
  files:
    - path: "config/app.yaml"
      content: "# Fresh config"
      
---
# Append to log files  
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: log-update
spec:
  # ... repository and auth config
  writeMode: "append" 
  files:
    - path: "logs/activity.log"
      content: "New log entry"
```

## Best Practices

### When to Use Overwrite Mode

✅ **Good for:**
- Configuration files that should be completely replaced
- Exported Kubernetes resources (`dump` strategy)
- Generated documentation or templates
- Files where you want clean, predictable content

❌ **Avoid for:**
- Log files or audit trails
- Files where you need to preserve existing data
- Collaborative files that others might modify

### When to Use Append Mode

✅ **Good for:**
- Log aggregation and audit trails
- Building chronological records
- Accumulating data over time
- Files where existing content should be preserved

❌ **Avoid for:**
- Configuration files (can create duplicates)
- Files that need clean, structured content
- Cases where file size could grow indefinitely

### File Management

```yaml
# Good: Clear intent with appropriate mode
files:
  - path: "config/database.yaml"      # Overwrite mode (default)
    content: "host: db.example.com"
    
  - path: "logs/deployment.log"       # Should use append mode
    content: "Deployment completed"
```

Consider implementing log rotation or cleanup strategies for append mode files to prevent unlimited growth.

## Error Handling

### Common Issues

| Issue | Cause | Solution |
|-------|--------|----------|
| File conflicts | Multiple resources writing to same path with different modes | Use separate GitCommit resources or different paths |
| Large files | Append mode causing excessive file growth | Implement cleanup strategy or use overwrite mode |
| Permission denied | Git repository doesn't allow file modifications | Check repository permissions and authentication |

### Validation

The operator validates:
- Write mode is either "overwrite" or "append"
- File paths are valid for the target repository
- Authentication allows write access

See [Error Handling](error-handling.md) for detailed troubleshooting information.

## Migration Between Write Modes

### From Overwrite to Append

**Before changing to append mode**, ensure existing files contain the baseline content you want to preserve.

```yaml
# Step 1: Final overwrite with baseline content
writeMode: "overwrite"
files:
  - path: "logs/audit.log" 
    content: |
      # Audit Log Started
      2023-10-01 00:00:00 - Baseline established
```

```yaml  
# Step 2: Switch to append mode
writeMode: "append"
files:
  - path: "logs/audit.log"
    content: |
      2023-10-01 10:00:00 - First append entry
```

### From Append to Overwrite

**Backup existing content** before switching, as overwrite mode will replace all accumulated content.

```bash
# Backup existing file
git show HEAD:logs/audit.log > backup-audit.log

# Then switch GitCommit to overwrite mode
```