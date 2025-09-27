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

## File Encryption Security

The operator supports age-based encryption for protecting sensitive files before committing to Git repositories:

### Encryption Key Management

**🔐 Best Practices for Encryption Keys:**

1. **Use separate keys per environment**: Different keys for dev/staging/production
2. **Store keys securely**: Use Kubernetes Secrets with proper RBAC restrictions
3. **Rotate keys regularly**: Implement key rotation procedures
4. **Prefer hardware keys for maximum security**: YubiKey > SSH/age keys > passphrases
   - **YubiKey**: Hardware-backed keys that never leave the device (highest security)
   - **SSH/age keys**: File-based keys that provide good security
   - **Passphrases**: Shared secrets with lower security (use sparingly)
5. **Document key ownership**: Maintain records of who has access to which keys

### Secure Secret Configuration

```yaml
# Restrict access to encryption secrets
apiVersion: v1
kind: Secret
metadata:
  name: encryption-keys
  namespace: secure-namespace
  labels:
    encryption.gco.galos.one/purpose: "file-encryption"
type: Opaque
data:
  id_rsa.pub: <base64-encoded-ssh-public-key>
  
---
# Create RBAC to limit access to encryption secrets
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: encryption-secret-access
  namespace: secure-namespace
rules:
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["encryption-keys"]
  verbs: ["get"]
```

### Encryption Security Benefits

- **Repository Safety**: Sensitive files can be safely committed to public repositories
- **Compliance**: Meet security requirements for storing secrets in version control
- **Audit Trail**: Git history provides encryption timestamps and accountability
- **Access Control**: Only users with decryption keys can access sensitive content
- **Zero Trust**: Assume Git repositories may be compromised; encrypted files remain secure

### Security Considerations

⚠️ **Important Security Notes:**

- Encryption keys stored in Kubernetes Secrets are only as secure as your cluster's security
- Consider using external secret management systems (HashiCorp Vault, AWS Secrets Manager, etc.)
- Encrypted files are still visible in Git history; consider using separate repositories for highly sensitive data
- Test decryption processes regularly to ensure keys remain valid
- Implement backup and recovery procedures for encryption keys

## Additional Resources

- [Helm Chart Configuration](https://github.com/mihaigalos/git-change-operator/tree/main/helm/git-change-operator)