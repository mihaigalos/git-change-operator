# Write Modes

Write modes control how the Git Change Operator handles existing file content when writing files to your Git repository. This applies to both GitCommit and PullRequest resources.

## Overview

The Git Change Operator supports two write modes:

1. **[Overwrite Mode](#overwrite-mode)** - Replace existing file content (default)
2. **[Append Mode](#append-mode)** - Add content to existing files

## Overwrite Mode

**Default behavior** - Replaces the entire content of existing files.

### Configuration

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: overwrite-example
spec:
  # ... repository and auth config
  
  files:
    - path: "config/app.properties"
      writeMode: "overwrite"  # This is the default
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
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: append-example
spec:
  # ... repository and auth config  
  
  files:
    - path: "logs/deployment.log"
      writeMode: "append"
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

Write modes can be configured per-file and also apply to resource references through their output strategy:

### Overwrite with Resource References

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: config-export
spec:
  # ... repository and auth config
  
  resourceRefs:
    - apiVersion: "v1"
      kind: "ConfigMap"
      name: "app-config"
      namespace: "default"
      strategy:
        type: "dump"
        path: "exported/app-config.yaml"
        writeMode: "overwrite"
```

**Result**: Replaces `exported/app-config.yaml` with current ConfigMap state.

### Append with Resource References

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit  
metadata:
  name: log-collection
spec:
  # ... repository and auth config
  
  resourceRefs:
    - apiVersion: "v1"
      kind: "ConfigMap"
      name: "application-logs"
      namespace: "default"
      strategy:
        type: "single-field"
        path: "aggregated/all-logs.txt"
        writeMode: "append"
        fieldRef:
          key: "latest.log"
```

**Result**: Adds the log content to the end of `aggregated/all-logs.txt`.

## Advanced Examples

### Mixed Content Types

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: mixed-content
spec:
  repository: "https://github.com/user/config-repo.git"
  branch: "main"
  commitMessage: "Update configurations and logs"
  authSecretRef: "git-credentials"
  
  files:
    # Static timestamp with append mode
    - path: "activity/timestamps.log"
      writeMode: "append"
      content: |
        {{ .Timestamp }} - GitCommit reconciliation started
        
  resourceRefs:
    # Append ConfigMap data to same log file
    - apiVersion: "v1"
      kind: "ConfigMap"
      name: "audit-log"
      namespace: "default"
      strategy:
        type: "single-field"
        path: "activity/timestamps.log"  # Same file as above
        writeMode: "append"
        fieldRef:
          key: "audit.log"
```

### Per-File Write Mode Control

Each file can have its own write mode within a single GitCommit:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: mixed-write-modes
spec:
  # ... repository and auth config
  
  files:
    # Overwrite configuration file
    - path: "config/app.yaml"
      writeMode: "overwrite"  # Replace entire config
      content: "# Fresh config"
      
    # Append to log file
    - path: "logs/activity.log"
      writeMode: "append"     # Add to existing logs
      content: "New log entry"
      
    # Default behavior (overwrite)
    - path: "status/health.txt"
      content: "System healthy"  # writeMode defaults to "overwrite"
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
# Good: Clear intent with appropriate mode per file
files:
  - path: "config/database.yaml"
    writeMode: "overwrite"           # Explicit overwrite for config
    content: "host: db.example.com"
    
  - path: "logs/deployment.log"
    writeMode: "append"              # Explicit append for logs
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
files:
  - path: "logs/audit.log" 
    writeMode: "overwrite"
    content: |
      # Audit Log Started
      2023-10-01 00:00:00 - Baseline established
```

```yaml  
# Step 2: Switch to append mode
files:
  - path: "logs/audit.log"
    writeMode: "append"
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