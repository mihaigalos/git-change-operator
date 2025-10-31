# Git Change Operator Helm Chart

This Helm chart deploys the Git Change Operator, a Kubernetes operator for managing Git commits and pull requests.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+

## Installation

### Add the repository

```bash
helm repo add git-change-operator https://raw.githubusercontent.com/mihaigalos/git-change-operator/helm-chart/
helm repo update
```

### Install from local directory

```bash
# Clone the repository
git clone https://github.com/mihaigalos/git-change-operator.git
cd git-change-operator

# Install the chart
helm install git-change-operator helm/git-change-operator \
  --create-namespace \
  --namespace git-change-operator-system
```

### Install with custom values

```bash
helm install git-change-operator helm/git-change-operator \
  --create-namespace \
  --namespace git-change-operator-system \
  --set image.repository=your-registry/git-change-operator \
  --set image.tag=v0.1.0 \
  --set resources.limits.memory=256Mi
```

## Security Considerations

Please see [../security.md].


## Configuration

The following table lists the configurable parameters and their default values:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `git-change-operator` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `replicaCount` | Number of replicas | `1` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `resources.requests.cpu` | CPU request | `10m` |
| `resources.requests.memory` | Memory request | `64Mi` |
| `serviceAccount.create` | Create service account | `true` |
| `rbac.create` | Create RBAC resources | `true` |
| `metrics.enabled` | Enable metrics endpoint | `true` |
| `metrics.serviceMonitor.enabled` | Create ServiceMonitor for Prometheus | `false` |
| `ingress.enabled` | Enable ingress for metrics endpoint | `false` |
| `ingress.className` | Ingress class name | `""` |
| `ingress.hosts[0].host` | Hostname for the ingress | `git-change-operator-metrics.local` |
| `operator.logLevel` | Log level (debug, info, warn, error) | `info` |
| `operator.leaderElect` | Enable leader election | `true` |

## Usage

After installation, you can create GitCommit and PullRequest resources:

### Example GitCommit

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: example-commit
  namespace: default
spec:
  repository: "https://github.com/your-org/your-repo.git"
  branch: "main"
  commitMessage: "Update configuration"
  authSecretRef: "git-auth-secret"
  files:
    - path: "config/app.yaml"
      content: |
        app:
          name: my-app
          version: "1.1.0"
```

### Example PullRequest

```yaml
apiVersion: gco.galos.one/v1
kind: PullRequest
metadata:
  name: example-pr
  namespace: default
spec:
  repository: "https://github.com/your-org/your-repo.git"
  baseBranch: "main"
  headBranch: "feature/new-feature"
  title: "Add new feature"
  body: "This PR adds a new feature to the application"
  authSecretRef: "git-auth-secret"
  files:
    - path: "features/new-feature.yaml"
      content: |
        feature:
          name: new-feature
          enabled: true
```

### Authentication Secret

Create a secret with your Git authentication credentials:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-auth-secret
  namespace: default
type: Opaque
stringData:
  token: "your-github-token"  # For GitHub
  # OR
  username: "your-username"   # For basic auth
  password: "your-password"
```

## Monitoring

### Prometheus ServiceMonitor

If you have Prometheus Operator installed, you can enable ServiceMonitor:

```bash
helm upgrade git-change-operator helm/git-change-operator \
  --set metrics.serviceMonitor.enabled=true
```

### Ingress for Metrics

To expose the metrics endpoint via ingress (useful for external Prometheus instances):

```bash
helm upgrade git-change-operator helm/git-change-operator \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=metrics.git-change-operator.example.com \
  --set ingress.className=nginx
```

For HTTPS with cert-manager:

```bash
helm upgrade git-change-operator helm/git-change-operator \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=metrics.git-change-operator.example.com \
  --set ingress.className=nginx \
  --set ingress.annotations."cert-manager\.io/cluster-issuer"=letsencrypt-prod \
  --set ingress.tls[0].secretName=git-change-operator-metrics-tls \
  --set ingress.tls[0].hosts[0]=metrics.git-change-operator.example.com
```

## Uninstallation

```bash
helm uninstall git-change-operator --namespace git-change-operator-system
```

## Development

### Lint the chart

```bash
make helm-lint
```

### Generate templates

```bash
make helm-template
```

### Package the chart

```bash
make helm-package
```

## License

This project is licensed under the MIT License - see the [LICENSE](../../LICENSE) file for details.