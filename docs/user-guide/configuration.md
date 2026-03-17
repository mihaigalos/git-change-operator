# Configuration

This guide covers all configuration options for the Git Change Operator, from basic setup to advanced enterprise configurations.

## Overview

The Git Change Operator can be configured at multiple levels:
- **Operator-level** configuration via the `GitChangeOperator` CR (runtime configuration)
- **Bootstrap-level** configuration via Helm values or Kustomize overlays (deployment time)
- **Resource-level** configuration via GitCommit and PullRequest specs
- **Cluster-level** configuration via ConfigMaps and other Kubernetes resources

## Operator Configuration

### GitChangeOperator Custom Resource (Runtime Configuration)

The recommended way to configure the operator at runtime is using the `GitChangeOperator` CR. This allows dynamic reconfiguration without redeploying Helm:

```yaml
apiVersion: gco.galos.one/v1
kind: GitChangeOperator
metadata:
  name: git-change-operator-config
  namespace: git-change-operator-system
spec:
  replicaCount: 2
  image:
    repository: ghcr.io/mihaigalos/git-change-operator
    tag: v1.2.0
    pullPolicy: IfNotPresent
  metrics:
    enabled: true
    service:
      type: ClusterIP
      port: 8080
    serviceMonitor:
      enabled: true
      interval: "30s"
      scrapeTimeout: "10s"
  ingress:
    enabled: true
    ingressClassName: nginx
    hosts:
      - host: git-change-operator.example.com
        paths:
          - path: /metrics
            pathType: Prefix
```

The operator reconciles changes to this CR and manages:
- Metrics Service
- ServiceMonitor (Prometheus)
- Ingress

See the [CRD Reference](../reference/crd-spec.md#gitchangeoperator-resource) for complete configuration options.

### Command Line Flags

The operator controller supports various command-line flags:

```bash
git-change-operator \
  --metrics-bind-addr=:8080 \
  --leader-elect=true \
  --zap-log-level=info \
  --reconcile-interval=30s \
  --max-concurrent-reconciles=10
```

### Environment Variables

Configure the operator using environment variables:

```yaml
# config/manager/manager.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
spec:
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: RECONCILE_INTERVAL
          value: "30s"
        - name: MAX_CONCURRENT_RECONCILES  
          value: "10"
        - name: LOG_LEVEL
          value: "info"
        - name: ENABLE_WEBHOOKS
          value: "true"
        - name: GIT_TIMEOUT
          value: "300s"
        - name: DEFAULT_BRANCH
          value: "main"
```

### Configuration Options

#### Core Settings

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--metrics-bind-addr` | `METRICS_BIND_ADDR` | `:8080` | Address for metrics server |
| `--leader-elect` | `ENABLE_LEADER_ELECTION` | `true` | Enable leader election |
| `--zap-log-level` | `LOG_LEVEL` | `info` | Log level (debug, info, error) |
| `--reconcile-interval` | `RECONCILE_INTERVAL` | `30s` | Default reconciliation interval |
| `--max-concurrent-reconciles` | `MAX_CONCURRENT_RECONCILES` | `10` | Max concurrent reconciliations |

#### Git Settings

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--git-timeout` | `GIT_TIMEOUT` | `300s` | Timeout for Git operations |
| `--default-branch` | `DEFAULT_BRANCH` | `main` | Default Git branch |
| `--git-user-name` | `GIT_USER_NAME` | `git-change-operator` | Default Git user name |
| `--git-user-email` | `GIT_USER_EMAIL` | `operator@example.com` | Default Git user email |

#### Resource Settings

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--enable-webhooks` | `ENABLE_WEBHOOKS` | `true` | Enable admission webhooks |
| `--webhook-port` | `WEBHOOK_PORT` | `9443` | Webhook server port |
| `--cert-dir` | `WEBHOOK_CERT_DIR` | `/tmp/k8s-webhook-server/serving-certs` | Webhook certificate directory |

## Resource Configuration

### GitCommit Configuration

#### Basic Configuration

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: example-commit
  namespace: default
spec:
  # Required fields
  repository: "https://github.com/myorg/config-repo.git"
  branch: "main"
  message: "Update configuration"
  
  # Optional configuration
  reconcileInterval: "60s"
  retryPolicy:
    maxRetries: 3
    backoff: "30s"
```

#### Advanced Configuration

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: advanced-commit
spec:
  repository: "https://github.com/myorg/config-repo.git"
  branch: "feature/automated-updates"
  message: |
    Automated configuration update
    
    Updated by: git-change-operator
    Cluster: {{ .cluster.name }}
    Timestamp: {{ .timestamp }}
  
  # Authentication configuration
  credentials:
    secretName: git-credentials
    usernameKey: username  # default: username
    passwordKey: token     # default: password
  
  # Git configuration
  author:
    name: "Git Change Operator"
    email: "operator@mycompany.com"
  committer:
    name: "Automation System"
    email: "automation@mycompany.com"
  
  # Resource reference configuration
  resourceRef:
    apiVersion: v1
    kind: ConfigMap
    name: app-config
    namespace: default
    path: "config/application.yaml"
    strategy: "template"
    template: |
      # Application Configuration
      # Generated on: {{ .timestamp }}
      app:
        name: {{ .metadata.name }}
        namespace: {{ .metadata.namespace }}
        data:
      {{ range $key, $value := .data }}
        {{ $key }}: {{ $value | quote }}
      {{ end }}
  
  # Write mode configuration
  writeMode: "overwrite"  # overwrite, append, merge
  
  # File configuration
  fileMode: "0644"
  directoryMode: "0755"
  createDirs: true
  
  # Reconciliation configuration
  reconcileInterval: "300s"
  suspend: false
  
  # Retry configuration
  retryPolicy:
    maxRetries: 5
    backoff: "60s"
    maxBackoff: "600s"
    backoffMultiplier: 2.0
```

### PullRequest Configuration

#### Basic Configuration

```yaml
apiVersion: gco.galos.one/v1
kind: PullRequest
metadata:
  name: example-pr
spec:
  repository: "https://github.com/myorg/config-repo.git"
  baseBranch: "main"
  headBranch: "automated-update"
  title: "Automated configuration update"
  body: "This PR contains automated updates from the Kubernetes cluster"
```

#### Advanced Configuration

```yaml
apiVersion: gco.galos.one/v1
kind: PullRequest
metadata:
  name: advanced-pr
spec:
  repository: "https://github.com/myorg/config-repo.git"
  baseBranch: "main"
  headBranch: "config-update-{{ .timestamp }}"
  
  # PR metadata
  title: "Configuration update from {{ .cluster.name }}"
  body: |
    # Automated Configuration Update
    
    This pull request contains automated configuration updates from the Kubernetes cluster.
    
    ## Changes
    {{ range .changes }}
    - Updated {{ .resource.kind }}/{{ .resource.name }} in namespace {{ .resource.namespace }}
    {{ end }}
    
    ## Validation
    - [ ] Configuration syntax is valid
    - [ ] No sensitive data is exposed
    - [ ] Changes are backwards compatible
    
    Generated by: git-change-operator
    Cluster: {{ .cluster.name }}
    Timestamp: {{ .timestamp }}
  
  # Labels and assignees
  labels:
    - "automated"
    - "configuration"
    - "cluster-sync"
  assignees:
    - "devops-team"
  reviewers:
    - "config-reviewers"
  
  # PR options
  draft: false
  maintainerCanModify: true
  
  # Authentication
  credentials:
    secretName: github-token
    tokenKey: token
  
  # Resource references (same as GitCommit)
  resourceRef:
    apiVersion: v1
    kind: Secret
    name: database-config
    namespace: production
    path: "config/database.yaml"
    strategy: "template"
```

## Authentication Configuration

### Git Credentials

#### Username/Password Authentication

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-credentials
  namespace: default
type: Opaque
data:
  username: dXNlcm5hbWU=  # base64 encoded username
  password: cGFzc3dvcmQ=  # base64 encoded password/token
```

#### SSH Key Authentication

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-ssh-key
  namespace: default
type: kubernetes.io/ssh-auth
data:
  ssh-privatekey: LS0tLS1CRUdJTi... # base64 encoded SSH private key
```

#### GitHub Token Authentication

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-token
  namespace: default
type: Opaque
data:
  token: Z2hwX3Rva2Vu...  # base64 encoded GitHub personal access token
```

### Authentication Strategies

#### Per-Resource Authentication

```yaml
# GitCommit with specific credentials
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: secure-commit
spec:
  repository: "https://github.com/private/repo.git"
  credentials:
    secretName: private-repo-creds
    usernameKey: username
    passwordKey: token
```

#### Default Authentication

Configure default credentials via environment variables:

```yaml
# In controller deployment
env:
- name: DEFAULT_GIT_USERNAME
  valueFrom:
    secretKeyRef:
      name: default-git-creds
      key: username
- name: DEFAULT_GIT_PASSWORD
  valueFrom:
    secretKeyRef:
      name: default-git-creds
      key: password
```

## Cluster-Level Configuration

### ConfigMap Configuration

Create a ConfigMap for operator-wide settings:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: git-change-operator-config
  namespace: git-change-operator-system
data:
  config.yaml: |
    git:
      timeout: "300s"
      defaultBranch: "main"
      defaultAuthor:
        name: "Git Change Operator"
        email: "operator@example.com"
    
    reconciliation:
      interval: "30s"
      maxConcurrent: 10
      retryPolicy:
        maxRetries: 3
        backoff: "30s"
    
    webhooks:
      enabled: true
      port: 9443
      certDir: "/tmp/k8s-webhook-server/serving-certs"
    
    features:
      enableMetrics: true
      enableProfiling: false
      enableTracing: false
```

Mount the ConfigMap in the controller:

```yaml
# config/manager/manager.yaml
spec:
  template:
    spec:
      containers:
      - name: manager
        args:
        - --config=/etc/config/config.yaml
        volumeMounts:
        - name: config
          mountPath: /etc/config
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: git-change-operator-config
```

### RBAC Configuration

#### Minimal RBAC

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: git-change-operator-manager-role
rules:
# GitCommit and PullRequest resources
- apiGroups: ["gco.galos.one"]
  resources: ["gitcommits", "pullrequests"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["gco.galos.one"]
  resources: ["gitcommits/status", "pullrequests/status"]
  verbs: ["get", "update", "patch"]

# Resources to read for references (minimal example)
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "configmaps"]
  verbs: ["get", "list", "watch"]
```

#### Extended RBAC for All Resources

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: git-change-operator-extended-role
rules:
# Core resources
- apiGroups: [""]
  resources: ["*"]
  verbs: ["get", "list", "watch"]

# Apps resources
- apiGroups: ["apps"]
  resources: ["*"]
  verbs: ["get", "list", "watch"]

# Custom resources (add as needed)
- apiGroups: ["networking.k8s.io"]
  resources: ["*"]
  verbs: ["get", "list", "watch"]

# Operator's own resources
- apiGroups: ["gco.galos.one"]
  resources: ["*"]
  verbs: ["*"]
```

## Performance Configuration

### Resource Limits

Configure appropriate resource limits:

```yaml
# config/manager/manager.yaml
spec:
  template:
    spec:
      containers:
      - name: manager
        resources:
          limits:
            cpu: 500m
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 128Mi
```

### Scaling Configuration

#### Horizontal Pod Autoscaling

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: git-change-operator-hpa
  namespace: git-change-operator-system
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: git-change-operator-controller-manager
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

#### Vertical Pod Autoscaling

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: git-change-operator-vpa
  namespace: git-change-operator-system
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: git-change-operator-controller-manager
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: manager
      minAllowed:
        cpu: 100m
        memory: 128Mi
      maxAllowed:
        cpu: 1000m
        memory: 1Gi
```

## Monitoring Configuration

### Metrics Configuration

Enable Prometheus metrics:

```yaml
# config/prometheus/monitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: git-change-operator-metrics
  namespace: git-change-operator-system
spec:
  endpoints:
  - path: /metrics
    port: http
    interval: 30s
    scheme: http
  selector:
    matchLabels:
      control-plane: controller-manager
```

### Custom Metrics

Configure custom metrics in the operator:

```yaml
# ConfigMap configuration
data:
  config.yaml: |
    metrics:
      enabled: true
      port: 8080
      path: "/metrics"
      interval: "30s"
      customMetrics:
        - name: "git_operations_total"
          help: "Total number of Git operations"
          type: "counter"
        - name: "git_operation_duration_seconds"
          help: "Duration of Git operations in seconds"
          type: "histogram"
        - name: "resource_references_total"
          help: "Total number of resource references processed"
          type: "counter"
```

## Logging Configuration

### Log Levels

Configure different log levels:

```yaml
env:
- name: LOG_LEVEL
  value: "info"  # debug, info, warn, error
- name: LOG_FORMAT
  value: "json"  # json, console
- name: LOG_CALLER
  value: "true"
- name: LOG_STACKTRACE_LEVEL
  value: "error"
```

### Structured Logging

Configure structured logging format:

```yaml
data:
  config.yaml: |
    logging:
      level: "info"
      format: "json"
      caller: true
      stacktraceLevel: "error"
      fields:
        service: "git-change-operator"
        version: "v1.0.0"
      outputs:
        - "stdout"
        - "/var/log/git-change-operator.log"
```

## Security Configuration

### Pod Security Standards

Configure Pod Security Standards:

```yaml
# Namespace configuration
apiVersion: v1
kind: Namespace
metadata:
  name: git-change-operator-system
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

### Security Context

Configure security context for the controller:

```yaml
# config/manager/manager.yaml
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        fsGroup: 65532
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: manager
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 65532
          runAsGroup: 65532
```

### Network Policies

Restrict network access:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: git-change-operator-netpol
  namespace: git-change-operator-system
spec:
  podSelector:
    matchLabels:
      control-plane: controller-manager
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: git-change-operator-system
    ports:
    - protocol: TCP
      port: 8080  # metrics
    - protocol: TCP
      port: 9443  # webhooks
  egress:
  - {}  # Allow all egress (for Git operations)
```

## Environment-Specific Configuration

### Development Environment

```yaml
# dev/config.yaml
git:
  timeout: "60s"
  defaultBranch: "dev"

reconciliation:
  interval: "10s"  # Faster reconciliation for development
  maxConcurrent: 5

logging:
  level: "debug"
  format: "console"

features:
  enableMetrics: true
  enableProfiling: true  # Enable profiling in dev
  enableTracing: true
```

### Production Environment

```yaml
# prod/config.yaml
git:
  timeout: "300s"
  defaultBranch: "main"

reconciliation:
  interval: "60s"  # Slower reconciliation for stability
  maxConcurrent: 20

logging:
  level: "info"
  format: "json"

features:
  enableMetrics: true
  enableProfiling: false  # Disable profiling in production
  enableTracing: false

security:
  enableWebhooks: true
  requireTLS: true
  minTLSVersion: "1.2"
```

### Enterprise Environment

```yaml
# enterprise/config.yaml
git:
  timeout: "600s"
  defaultBranch: "main"
  proxy:
    http: "http://proxy.company.com:8080"
    https: "http://proxy.company.com:8080"
    noProxy: "localhost,127.0.0.1,.company.com"

reconciliation:
  interval: "300s"  # Conservative reconciliation
  maxConcurrent: 50

logging:
  level: "info"
  format: "json"
  auditLog:
    enabled: true
    path: "/var/log/audit/git-change-operator.log"

security:
  enableWebhooks: true
  requireTLS: true
  minTLSVersion: "1.3"
  certificateAuthority: "/etc/ssl/certs/ca-certificates.crt"

compliance:
  enableAuditLogging: true
  retentionPeriod: "2555d"  # 7 years
  encryptionAtRest: true
```

## Configuration Validation

### Webhook Validation

The operator includes admission webhooks to validate configurations:

```yaml
# Automatic validation for GitCommit resources
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionWebhook
metadata:
  name: vgitcommit.kb.io
webhooks:
- name: vgitcommit.kb.io
  clientConfig:
    service:
      name: webhook-service
      namespace: git-change-operator-system
      path: /validate-git-galos-one-v1-gitcommit
  rules:
  - operations: ["CREATE", "UPDATE"]
    apiGroups: ["gco.galos.one"]
    apiVersions: ["v1"]
    resources: ["gitcommits"]
```

### Configuration Testing

Test your configuration before applying:

```bash
# Validate GitCommit resource
kubectl apply --dry-run=server -f gitcommit.yaml

# Validate with webhook
kubectl apply --dry-run=server -f gitcommit.yaml --validate=true

# Test configuration parsing
git-change-operator --config=config.yaml --validate-config
```

## Troubleshooting Configuration

### Common Configuration Issues

#### Invalid Git Repository URLs
```yaml
# ❌ Invalid
repository: "github.com/user/repo"

# ✅ Valid
repository: "https://github.com/user/repo.git"
repository: "git@github.com:user/repo.git"
```

#### Missing Required Fields
```yaml
# ❌ Missing required fields
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: incomplete
spec:
  repository: "https://github.com/user/repo.git"
  # Missing: branch, message

# ✅ Complete
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: complete
spec:
  repository: "https://github.com/user/repo.git"
  branch: "main"
  message: "Update configuration"
```

#### Resource Reference Errors
```yaml
# ❌ Invalid field path
resourceRef:
  fieldPath: "data.nonexistent"

# ✅ Valid field path
resourceRef:
  fieldPath: "data.config"
```

### Configuration Debugging

Enable debug logging to troubleshoot configuration issues:

```bash
# Enable debug logging
kubectl patch deployment git-change-operator-controller-manager \
  -n git-change-operator-system \
  --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/env/0/value", "value": "debug"}]'

# View debug logs
kubectl logs -n git-change-operator-system -l control-plane=controller-manager --tail=100 -f
```

## Next Steps

After configuring the operator:

1. [GitCommit Resources](gitcommit.md) - Learn about GitCommit resources
2. [PullRequest Resources](pullrequest.md) - Understand PullRequest automation
3. [Authentication](authentication.md) - Set up secure authentication
4. [Examples](../examples/) - See real-world configuration examples

For advanced configuration scenarios, see our [Enterprise Setup Guide](../examples/corporate-setup.md).