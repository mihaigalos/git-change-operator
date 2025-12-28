# Kind Full Demo

This guide provides a complete end-to-end demonstration of the Git Change Operator using a local Kind (Kubernetes in Docker) cluster. This is the fastest way to see the operator in action and understand its capabilities.

## Prerequisites

Before starting, ensure you have:

- Docker installed and running
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/) installed
- [Helm](https://helm.sh/docs/intro/install/) installed
- A GitHub personal access token with repository permissions
- The git-change-operator source code cloned locally

## Corporate Environment Support

If you're behind a corporate proxy or firewall, the demo includes built-in support for corporate CA certificates. Place your corporate CA certificate at `~/certs/zscaler.pem` (or adjust the path in `kind-config.yaml`).

## One-Command Demo

The simplest way to run the complete demo is with our comprehensive Makefile target:

```bash
just kind-full-demo
```

This single command will:
1. Create a Kind cluster with proper configuration
2. Build and load the operator Docker image
3. Deploy the operator using Helm
4. Create a test GitCommit resource
5. Verify the commit was created on GitHub

## Step-by-Step Breakdown

If you want to understand each step or run them individually, here's what the full demo does:

### 1. Create Kind Cluster

```bash
just kind-create
```

This creates a Kind cluster with:
- Corporate proxy/CA certificate support
- Proper networking configuration
- Extended timeouts for corporate environments

### 2. Build and Deploy Operator

```bash
just kind-deploy
```

This will:
- Build the operator Docker image with version tags
- Load the image into the Kind cluster
- Install/upgrade the Helm chart with conditional CRD installation
- Verify all pods are running

### 3. Test the Operator

The demo creates a sample GitCommit resource that will:
- Create a real commit on GitHub
- Demonstrate the operator's core functionality
- Show the reconciliation process in action

### 4. Verify Results

You can verify the demo worked by:

```bash
# Check operator logs
kubectl logs -n git-change-operator-system deployment/git-change-operator-controller-manager

# Check GitCommit resource status
kubectl get gitcommits -o yaml

# Verify the commit exists on GitHub (check the output for the commit URL)
```

## Expected Output

When successful, you should see:
- Kind cluster created and ready
- Operator pods running in `git-change-operator-system` namespace
- GitCommit resource created and processed
- Real commit created on your GitHub repository
- Commit SHA and URL displayed in the resource status

## Configuration

The demo uses these key files:
- `kind-config.yaml` - Kind cluster configuration with corporate support
- `corporate-config.env` - Environment variables for corporate proxy
- `Makefile` - Automation workflow with hidden helper targets

## Troubleshooting

### Common Issues

**Docker image not found:**
```bash
# Verify image was built and loaded
docker images | grep git-change-operator
kind get clusters
```

**Operator pods not starting:**
```bash
# Check pod status and logs
kubectl get pods -n git-change-operator-system
kubectl describe pods -n git-change-operator-system
```

**GitHub authentication issues:**
```bash
# Verify your GitHub token has proper permissions
# Check the secret was created correctly
kubectl get secrets -n git-change-operator-system
```

**Corporate proxy issues:**
- Ensure `~/certs/zscaler.pem` exists and contains your CA certificate
- Check `corporate-config.env` has correct proxy settings
- Verify Docker can pull images through your proxy

### Clean Up

To remove the demo environment:

```bash
just kind-destroy
```

This removes the Kind cluster and cleans up all resources.

## Next Steps

After running the demo successfully:
1. Explore the [Configuration](configuration.md) guide for production setup
2. Learn about advanced features in the [Quick Start](quick-start.md) guide
3. Check the [Installation](installation.md) guide for production deployment options

The Kind demo provides a safe, local environment to experiment with the Git Change Operator before deploying it to production clusters.