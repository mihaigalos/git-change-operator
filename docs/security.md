# Security Considerations

> [!WARNING]
> The default RBAC configuration grants broad read permissions (`"*"`) across all API groups and resources. This is **NOT recommended for production environments**.

## Production Deployment

For production deployments, use the production values file with specific RBAC permissions:

```bash
# Add the Helm repository
helm repo add git-change-operator https://raw.githubusercontent.com/mihaigalos/git-change-operator/helm-chart/
helm repo update

# Install with production security configuration
helm install git-change-operator git-change-operator/git-change-operator \
  --create-namespace \
  --namespace git-change-operator-system \
  --values https://raw.githubusercontent.com/mihaigalos/git-change-operator/main/helm/git-change-operator/values-production.yaml
```

## Custom RBAC Configuration

The operator supports configurable RBAC permissions through Helm values. You can customize the permissions by setting:

```yaml
rbac:
  additionalReadPermissions:
    # Disable wildcard permissions for production
    enableWildcard: false
    
    # Grant only specific permissions needed
    specificPermissions:
      - apiGroups: [""]
        resources: ["configmaps", "secrets", "pods"]
      - apiGroups: ["apps"] 
        resources: ["deployments", "replicasets"]
      - apiGroups: ["networking.k8s.io"]
        resources: ["ingresses"]
```

## Principle of Least Privilege

The operator only needs **read access** to resources that GitCommit and PullRequest resources reference. Follow these guidelines:

1. **Start minimal**: Begin with no additional permissions
2. **Add incrementally**: Add specific permissions only as needed
3. **Audit regularly**: Review and remove unused permissions
4. **Use production values**: Always use the production configuration for production deployments

## Production Values File

The included `values-production.yaml` provides a secure baseline configuration:

- Disables wildcard RBAC permissions
- Includes only essential resource permissions
- Sets appropriate resource limits
- Configures security contexts

## Authentication Security

When configuring Git authentication:

1. **Use Kubernetes Secrets**: Store credentials securely in Kubernetes Secrets or use an operator like the [SealedSecrets Operator](https://github.com/bitnami-labs/sealed-secrets) to store them encrypted in git and unseal them on the cluster
2. **Limit scope**: Use deploy keys or tokens with minimal required permissions
3. **Rotate regularly**: Implement regular credential rotation
4. **Audit access**: Monitor and audit Git repository access

## Additional Resources

- [Helm Chart Configuration](https://github.com/mihaigalos/git-change-operator/tree/main/helm/git-change-operator)