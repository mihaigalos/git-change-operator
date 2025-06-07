# Authentication

This guide covers all authentication methods supported by the Git Change Operator for secure access to Git repositories and Git hosting providers like GitHub, GitLab, and Bitbucket.

## Overview

The Git Change Operator supports multiple authentication methods:

- **Personal Access Tokens** - GitHub, GitLab, Bitbucket tokens
- **SSH Keys** - Public/private key authentication
- **Basic Authentication** - Username/password (not recommended for production)
- **App Authentication** - GitHub App authentication (coming soon)
- **OAuth** - OAuth-based authentication (coming soon)

## Personal Access Tokens

### GitHub Personal Access Token

GitHub Personal Access Tokens (PAT) are the recommended method for GitHub authentication.

#### Creating a GitHub Token

1. Go to [GitHub Settings → Developer settings → Personal access tokens](https://github.com/settings/tokens)
2. Click "Generate new token"
3. Select appropriate scopes:
   - `repo` - Full repository access (for private repos)
   - `public_repo` - Public repository access (for public repos)
   - `workflow` - Update GitHub Actions workflows (if needed)
   - `read:org` - Read organization membership (if using organization repos)

#### Token Permissions by Use Case

**Public Repository Access:**
```yaml
# Minimal permissions for public repos
scopes:
  - public_repo
```

**Private Repository Access:**
```yaml
# Full repository access for private repos  
scopes:
  - repo
```

**Organization Repository with Workflows:**
```yaml
# Organization repos with GitHub Actions
scopes:
  - repo
  - workflow
  - read:org
```

#### Storing GitHub Tokens

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-credentials
  namespace: default
type: Opaque
data:
  token: Z2hwX3Rva2VuX2hlcmU=  # base64 encoded GitHub PAT
  # Optional: store username for reference
  username: bXl1c2VybmFtZQ==     # base64 encoded username
```

#### Using GitHub Tokens in Resources

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: github-commit
spec:
  repository: "https://github.com/myorg/private-repo.git"
  branch: "main"
  message: "Update from cluster"
  
  # GitHub token authentication
  credentials:
    secretName: github-credentials
    tokenKey: token  # Key in secret containing the token
    
  resourceRef:
    apiVersion: v1
    kind: ConfigMap
    name: app-config
    namespace: default
    path: "config.yaml"
```

### GitLab Personal Access Token

#### Creating a GitLab Token

1. Go to [GitLab Settings → Access Tokens](https://gitlab.com/-/profile/personal_access_tokens)
2. Create a new token with appropriate scopes:
   - `api` - Full API access
   - `read_repository` - Read repository access
   - `write_repository` - Write repository access

#### GitLab Token Configuration

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gitlab-credentials
  namespace: default
type: Opaque
data:
  token: Z2xwYXRfdG9rZW5faGVyZQ==  # base64 encoded GitLab token
  username: Z2l0bGFidXNlcg==         # base64 encoded username

---
apiVersion: git.galos.one/v1
kind: PullRequest
metadata:
  name: gitlab-mr
spec:
  repository: "https://gitlab.com/myorg/project.git"
  baseBranch: "main"
  headBranch: "automated-update"
  title: "Automated update from cluster"
  
  # GitLab token authentication
  credentials:
    secretName: gitlab-credentials
    tokenKey: token
  
  # GitLab-specific provider
  provider: "gitlab"
```

### Bitbucket App Password

#### Creating Bitbucket App Password

1. Go to [Bitbucket Settings → App passwords](https://bitbucket.org/account/settings/app-passwords/)
2. Create a new app password with permissions:
   - `Repositories: Read` - Read repository access
   - `Repositories: Write` - Write repository access
   - `Pull requests: Write` - Create pull requests

#### Bitbucket Configuration

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bitbucket-credentials
  namespace: default
type: Opaque
data:
  username: Yml0YnVja2V0dXNlcg==  # base64 encoded Bitbucket username
  password: YXBwX3Bhc3N3b3Jk      # base64 encoded app password

---
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: bitbucket-commit
spec:
  repository: "https://bitbucket.org/myworkspace/repo.git"
  
  credentials:
    secretName: bitbucket-credentials
    usernameKey: username
    passwordKey: password
  
  provider: "bitbucket"
```

## SSH Key Authentication

SSH keys provide secure authentication without exposing passwords or tokens.

### Generating SSH Keys

```bash
# Generate SSH key pair
ssh-keygen -t ed25519 -C "git-change-operator@mycompany.com" -f git-operator-key

# This creates:
# - git-operator-key (private key)  
# - git-operator-key.pub (public key)
```

### Adding SSH Key to Git Provider

#### GitHub SSH Key

1. Copy the public key content:
   ```bash
   cat git-operator-key.pub
   ```

2. Go to [GitHub Settings → SSH and GPG keys](https://github.com/settings/keys)

3. Click "New SSH key" and paste the public key

#### GitLab SSH Key

1. Copy the public key content
2. Go to [GitLab Settings → SSH Keys](https://gitlab.com/-/profile/keys)
3. Add the public key

### Creating SSH Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-ssh-credentials
  namespace: default
type: kubernetes.io/ssh-auth
data:
  ssh-privatekey: |
    LS0tLS1CRUdJTiBPUEVOU1NIIFBSSVZBVEUgS0VZLS0tLS0=
    # ... base64 encoded private key content ...
    LS0tLS1FTkQgT1BFTlNTSCBQUklWQVRFIEtFWS0tLS0t
stringData:
  # Optional: known_hosts for host verification
  known_hosts: |
    github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
    gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
```

### Using SSH Authentication

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: ssh-commit
spec:
  # Use SSH URL format
  repository: "git@github.com:myorg/private-repo.git"
  
  credentials:
    secretName: git-ssh-credentials
    # SSH secrets use standard keys
    privateKeyKey: ssh-privatekey  # Default for ssh-auth type
    knownHostsKey: known_hosts     # Optional
```

## Basic Authentication

**Note:** Basic authentication with username/password is not recommended for production use due to security concerns. Use Personal Access Tokens instead.

### Basic Auth Configuration

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: basic-auth-credentials
  namespace: default
type: Opaque
data:
  username: bXl1c2VybmFtZQ==  # base64 encoded username
  password: bXlwYXNzd29yZA==  # base64 encoded password

---
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: basic-auth-commit
spec:
  repository: "https://github.com/myorg/repo.git"
  
  credentials:
    secretName: basic-auth-credentials
    usernameKey: username
    passwordKey: password
```

## Multi-Repository Authentication

### Per-Repository Credentials

Use different credentials for different repositories:

```yaml
# Production repository credentials
apiVersion: v1
kind: Secret
metadata:
  name: prod-repo-creds
  namespace: production
type: Opaque
data:
  token: cHJvZF90b2tlbl9oZXJl

---
# Staging repository credentials  
apiVersion: v1
kind: Secret
metadata:
  name: staging-repo-creds
  namespace: staging
type: Opaque
data:
  token: c3RhZ2luZ190b2tlbl9oZXJl

---
# Production GitCommit
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: prod-commit
  namespace: production
spec:
  repository: "https://github.com/myorg/prod-config.git"
  credentials:
    secretName: prod-repo-creds
    tokenKey: token

---
# Staging GitCommit
apiVersion: git.galos.one/v1
kind: GitCommit  
metadata:
  name: staging-commit
  namespace: staging
spec:
  repository: "https://github.com/myorg/staging-config.git"
  credentials:
    secretName: staging-repo-creds
    tokenKey: token
```

### Default Credentials

Configure default credentials at the operator level:

```yaml
# Default credentials secret
apiVersion: v1
kind: Secret
metadata:
  name: default-git-credentials
  namespace: git-change-operator-system
type: Opaque
data:
  token: ZGVmYXVsdF90b2tlbl9oZXJl

---
# Configure operator to use default credentials
apiVersion: apps/v1
kind: Deployment
metadata:
  name: git-change-operator-controller-manager
  namespace: git-change-operator-system
spec:
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: DEFAULT_GIT_TOKEN
          valueFrom:
            secretKeyRef:
              name: default-git-credentials
              key: token
        - name: DEFAULT_GIT_USERNAME
          value: "git-change-operator"
```

### Organization-Wide Credentials

Use organization credentials for multiple repositories:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: org-credentials
  namespace: default
type: Opaque
data:
  token: b3JnX3Rva2VuX2hlcmU=  # Token with access to all org repos

---
# Multiple GitCommits using same credentials
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: repo1-commit
spec:
  repository: "https://github.com/myorg/repo1.git"
  credentials:
    secretName: org-credentials
    tokenKey: token

---
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: repo2-commit
spec:
  repository: "https://github.com/myorg/repo2.git"
  credentials:
    secretName: org-credentials
    tokenKey: token
```

## Advanced Authentication

### Token Rotation

Implement automatic token rotation:

```yaml
# Primary token
apiVersion: v1
kind: Secret
metadata:
  name: github-token-primary
  namespace: default
  annotations:
    rotation.io/expires: "2024-12-31"
type: Opaque
data:
  token: cHJpbWFyeV90b2tlbg==

---
# Backup token (for rotation)
apiVersion: v1
kind: Secret
metadata:
  name: github-token-backup
  namespace: default
  annotations:
    rotation.io/expires: "2025-01-31"
type: Opaque
data:
  token: YmFja3VwX3Rva2Vu

---
# GitCommit with fallback credentials
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: rotating-auth-commit
spec:
  repository: "https://github.com/myorg/repo.git"
  
  # Primary credentials
  credentials:
    secretName: github-token-primary
    tokenKey: token
  
  # Fallback credentials (used if primary fails)
  fallbackCredentials:
    secretName: github-token-backup
    tokenKey: token
```

### Cross-Cluster Authentication

Share credentials across clusters:

```yaml
# External secret for cross-cluster sync
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: git-credentials-sync
  namespace: default
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: cluster-secret-store
    kind: ClusterSecretStore
  target:
    name: git-credentials
    creationPolicy: Owner
  data:
  - secretKey: token
    remoteRef:
      key: git-tokens
      property: github-token
```

### Environment-Specific Authentication

Use different authentication per environment:

```yaml
# Base secret template
apiVersion: v1
kind: Secret
metadata:
  name: git-creds-template
type: Opaque
stringData:
  token: ${GIT_TOKEN}
  username: ${GIT_USERNAME}

---
# Production overlay
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: production
resources:
- ../base
replacements:
- source:
    kind: Secret
    name: production-git-token
    fieldPath: data.token
  targets:
  - select:
      kind: Secret
      name: git-creds-template
    fieldPaths:
    - stringData.token

---
# Staging overlay  
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: staging
resources:
- ../base
replacements:
- source:
    kind: Secret
    name: staging-git-token
    fieldPath: data.token
  targets:
  - select:
      kind: Secret
      name: git-creds-template
    fieldPaths:
    - stringData.token
```

## Security Best Practices

### Token Security

1. **Use minimal permissions** - Only grant necessary scopes
2. **Rotate tokens regularly** - Set expiration dates and rotate
3. **Monitor token usage** - Track API calls and access patterns
4. **Revoke unused tokens** - Remove tokens that are no longer needed
5. **Use separate tokens** for different environments

### Secret Management

1. **Use Kubernetes secrets** - Store credentials securely
2. **Enable secret encryption** at rest
3. **Limit secret access** with RBAC
4. **Audit secret usage** - Monitor secret access
5. **Use external secret managers** when possible

### RBAC Configuration

```yaml
# Minimal RBAC for reading credentials
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: default
  name: git-credentials-reader
rules:
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["git-credentials", "github-token"]
  verbs: ["get"]

---
# Bind to operator service account
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: git-credentials-binding
  namespace: default
subjects:
- kind: ServiceAccount
  name: git-change-operator-controller-manager
  namespace: git-change-operator-system
roleRef:
  kind: Role
  name: git-credentials-reader
  apiGroup: rbac.authorization.k8s.io
```

### Network Security

```yaml
# Network policy to restrict operator access
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
  - Egress
  egress:
  # Allow Git hosting providers
  - to: []
    ports:
    - protocol: TCP
      port: 443  # HTTPS
    - protocol: TCP
      port: 22   # SSH
  # Allow Kubernetes API
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443
```

## Provider-Specific Configuration

### GitHub Enterprise

```yaml
apiVersion: git.galos.one/v1
kind: GitCommit
metadata:
  name: github-enterprise-commit
spec:
  repository: "https://github.enterprise.com/myorg/repo.git"
  
  # GitHub Enterprise configuration
  provider: "github"
  providerConfig:
    baseURL: "https://github.enterprise.com/api/v3"
    uploadURL: "https://github.enterprise.com/api/uploads"
  
  credentials:
    secretName: github-enterprise-token
    tokenKey: token
```

## Troubleshooting Authentication

### Common Issues

#### Token Permission Errors

```bash
# Error: Insufficient permissions
# Check token scopes
curl -H "Authorization: token $GITHUB_TOKEN" \
     -H "Accept: application/vnd.github.v3+json" \
     https://api.github.com/user

# Verify repository access
curl -H "Authorization: token $GITHUB_TOKEN" \
     https://api.github.com/repos/myorg/myrepo
```

#### SSH Key Issues

```bash
# Test SSH connection
ssh -T git@github.com

# Debug SSH connection
ssh -vT git@github.com

# Check SSH key fingerprint
ssh-keygen -lf ~/.ssh/id_ed25519.pub
```

#### Certificate Issues

```bash
# Skip SSL verification (not recommended for production)
git config --global http.sslVerify false

# Use custom CA certificate
git config --global http.sslCAInfo /path/to/ca-cert.pem
```

### Debug Commands

```bash
# Check secret content
kubectl get secret git-credentials -o yaml

# Test authentication manually
kubectl exec -it <operator-pod> -- git ls-remote https://github.com/myorg/repo.git

# Check operator logs for auth errors
kubectl logs -n git-change-operator-system -l control-plane=controller-manager --tail=100
```

### Validation Steps

```bash
# 1. Verify secret exists and has correct keys
kubectl describe secret git-credentials

# 2. Check RBAC permissions
kubectl auth can-i get secrets --as=system:serviceaccount:git-change-operator-system:git-change-operator-controller-manager

# 3. Test repository access
git clone <repository-url>

# 4. Verify token permissions
curl -H "Authorization: token <token>" https://api.github.com/user/repos

# 5. Check network connectivity
kubectl exec <operator-pod> -- nslookup github.com
```

## Next Steps

After setting up authentication:

1. [GitCommit Resources](gitcommit.md) - Create your first GitCommit
2. [PullRequest Resources](pullrequest.md) - Automate pull request creation
3. [Configuration](configuration.md) - Fine-tune operator settings
4. [Examples](../examples/) - See real-world authentication examples

For enterprise authentication patterns, see our [Corporate Setup Guide](../examples/corporate-setup.md).