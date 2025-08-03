# Embedded Helm Chart

The git-change-operator Docker image includes the Helm chart for easy deployment without needing to clone the repository.

## Extracting the Chart

```bash
# Extract the chart from the image
docker create --name temp-container ghcr.io/mihaigalos/git-change-operator:latest
docker cp temp-container:/helm/git-change-operator ./chart
docker rm temp-container

# Use the extracted chart with Helm
helm install git-change-operator ./chart --namespace git-change-operator-system --create-namespace
```

The Helm chart is located at `/helm/git-change-operator` in the Docker image.