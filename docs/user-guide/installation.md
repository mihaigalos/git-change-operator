# Installation

This guide covers the installation and initial setup of the Git Change Operator in your Kubernetes cluster.

## Prerequisites

Before installing the Git Change Operator, ensure you have:

- **Kubernetes cluster** (v1.20+)
- **kubectl** configured to access your cluster
- **Cluster admin permissions** (for CRD installation)
- **Git repository** with write access
- **GitHub personal access token** (for PullRequest resources)

## Installation Methods

### Helm Installation from upstream (Recommended)

The easiest way to install the Git Change Operator is using Helm:

```bash
# Add the Helm repository
helm repo add git-change-operator https://raw.githubusercontent.com/mihaigalos/git-change-operator/helm-chart/
helm repo update

# Install the operator
helm install git-change-operator git-change-operator/git-change-operator \
  --namespace git-change-operator-system \
  --create-namespace
```

### Helm installation from the operator container

The git-change-operator Docker image includes the Helm chart for easy deployment without needing to clone the repository.

```bash
# Extract the chart from the image
docker create --name temp-container ghcr.io/mihaigalos/git-change-operator:latest
docker cp temp-container:/helm/git-change-operator ./chart
docker rm temp-container

# Use the extracted chart with Helm
helm install git-change-operator ./chart --namespace git-change-operator-system --create-namespace
```

The Helm chart is located at `/helm/git-change-operator` in the Docker image.

### kubectl Installation

You can install directly using kubectl:

```bash
# Install CRDs and RBAC
kubectl apply -k https://github.com/mihaigalos/git-change-operator/config

# Or clone and install locally
git clone https://github.com/mihaigalos/git-change-operator.git
cd git-change-operator
just install
```

### Development Installation

For development and testing:

```bash
# Clone the repository
git clone https://github.com/mihaigalos/git-change-operator.git
cd git-change-operator

# Install CRDs
just install

# Run locally (outside cluster)
just run
```

## Configuration

### Authentication Setup

Create a Kubernetes secret containing your git credentials:

```bash
kubectl create secret generic git-credentials \
  --namespace=git-change-operator-system \
  --from-literal=username=your-github-username \
  --from-literal=token=ghp_your_personal_access_token
```

For GitHub personal access tokens, you need the following scopes:
- `repo` (full repository access)
- `write:packages` (if using GitHub Container Registry)

### Operator Configuration

The operator can be configured through Helm values or environment variables:

```yaml
# values.yaml for Helm
operator:
  image:
    repository: git-change-operator
    tag: latest
  
  resources:
    limits:
      cpu: 200m
      memory: 256Mi
    requests:
      cpu: 100m
      memory: 128Mi
  
  # Leader election (for multiple replicas)
  leaderElection:
    enabled: true
```

## Verification

### Check Installation

Verify the operator is running:

```bash
# Check operator deployment
kubectl get deployment git-change-operator-controller-manager \
  -n git-change-operator-system

# Check CRDs are installed
kubectl get crd | grep gco.galos.one

# Expected output:
# gitcommits.gco.galos.one
# pullrequests.gco.galos.one
```

### Test Basic Functionality

Create a simple GitCommit resource to test:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: test-commit
  namespace: default
spec:
  repository: https://github.com/your-username/your-repo.git
  branch: main
  commitMessage: "Test commit from Git Change Operator"
  authSecretRef: git-credentials
  files:
    - path: test-file.txt
      content: |
        This is a test file created by the Git Change Operator.
        Installation successful!
```

Apply and check:

```bash
kubectl apply -f test-gitcommit.yaml
kubectl get gitcommits test-commit -o yaml
```

## Troubleshooting

### Common Issues

**CRDs not installed:**
```bash
# Manually install CRDs
kubectl apply -f config/crd/bases/
```

**Permission denied errors:**
```bash
# Check RBAC permissions
kubectl auth can-i create gitcommits --as=system:serviceaccount:git-change-operator-system:git-change-operator-controller-manager
```

**Git authentication failures:**
```bash
# Verify secret exists and has correct format
kubectl get secret git-credentials -o yaml
kubectl describe secret git-credentials
```

### Logs and Debugging

Check operator logs:

```bash
# Get operator logs
kubectl logs deployment/git-change-operator-controller-manager \
  -n git-change-operator-system

# Follow logs in real-time
kubectl logs -f deployment/git-change-operator-controller-manager \
  -n git-change-operator-system
```

### Uninstallation

To completely remove the operator:

```bash
# Using Helm
helm uninstall git-change-operator -n git-change-operator-system

# Using kubectl
kubectl delete -k config/

# Remove namespace
kubectl delete namespace git-change-operator-system
```

## Next Steps

After successful installation:

1. **[Configure authentication](authentication.md)** for your git repositories
2. **[Create your first GitCommit](../examples/basic-gitcommit.md)** resource
3. **[Set up PullRequest automation](../examples/pullrequest.md)**
4. **[Explore advanced features](../examples/advanced.md)**

## Support

If you encounter issues:

- Check the [troubleshooting guide](../reference/error-handling.md)
- Review [GitHub Issues](https://github.com/mihaigalos/git-change-operator/issues)
- Join [GitHub Discussions](https://github.com/mihaigalos/git-change-operator/discussions)