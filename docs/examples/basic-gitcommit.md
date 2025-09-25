# Basic GitCommit

This example demonstrates the most basic usage of the GitCommit resource to commit static files to a Git repository.

## Prerequisites

- Git Change Operator installed in the cluster
- A Git repository with write access
- Kubernetes Secret with Git credentials

## Setup Authentication

First, create a Secret with your Git credentials:

```bash
kubectl create secret generic git-credentials \
  --from-literal=username=your-username \
  --from-literal=password=your-token
```

For GitHub, use a Personal Access Token as the password with `repo` permissions.

## Basic GitCommit Example

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: basic-commit
  namespace: default
spec:
  repository:
    url: "https://github.com/your-username/config-repo.git"
    branch: "main"
    
  auth:
    secretName: "git-credentials"
    
  commit:
    author: "Git Change Operator <operator@example.com>"
    message: "Add basic configuration file"
    
  files:
    - path: "config/application.properties"
      content: |
        # Application Configuration
        server.port=8080
        server.host=0.0.0.0
        
        # Database Configuration
        database.url=jdbc:postgresql://localhost:5432/mydb
        database.username=app_user
        
        # Logging Configuration
        logging.level.root=INFO
        logging.level.com.example=DEBUG
```

## Apply the GitCommit

```bash
# Apply the GitCommit resource
kubectl apply -f basic-gitcommit.yaml

# Check the status
kubectl get gitcommit basic-commit -o yaml
```

## Expected Results

After applying the GitCommit, you should see:

1. **In your Git repository**: A new commit with the file `config/application.properties`
2. **GitCommit status**: Ready condition set to `True`

```yaml
status:
  conditions:
  - type: Ready
    status: "True"
    lastTransitionTime: "2023-10-01T10:00:00Z"
    reason: "CommitSuccessful" 
    message: "Successfully committed to repository"
  lastCommitHash: "abc123def456789..."
```

## Verify the Commit

You can verify the commit was created:

```bash
# Check the latest commit in your repository
git log --oneline -1

# View the created file
cat config/application.properties
```

## Multiple Files Example

You can commit multiple files in a single GitCommit:

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: multi-file-commit
spec:
  repository:
    url: "https://github.com/your-username/config-repo.git"
    
  auth:
    secretName: "git-credentials"
    
  commit:
    author: "Git Change Operator <operator@example.com>"
    message: "Add application configuration and deployment files"
    
  files:
    - path: "config/application.yaml"
      content: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: app-config
        data:
          app.properties: |
            server.port=8080
            debug=true
            
    - path: "deploy/deployment.yaml"
      content: |
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: my-app
        spec:
          replicas: 3
          selector:
            matchLabels:
              app: my-app
          template:
            metadata:
              labels:
                app: my-app
            spec:
              containers:
              - name: app
                image: my-app:latest
                ports:
                - containerPort: 8080
                
    - path: "docs/README.md"
      content: |
        # My Application
        
        This repository contains configuration and deployment files for my application.
        
        ## Files
        
        - `config/`: Application configuration
        - `deploy/`: Kubernetes deployment manifests
        - `docs/`: Documentation
```

## Directory Structure

The above example creates this structure in your Git repository:

```
├── config/
│   ├── application.properties
│   └── application.yaml
├── deploy/
│   └── deployment.yaml
└── docs/
    └── README.md
```

## Troubleshooting

### Common Issues

1. **Authentication Failed**
   ```bash
   # Check your Secret
   kubectl get secret git-credentials -o yaml
   
   # Verify credentials work
   git ls-remote https://username:token@github.com/user/repo.git
   ```

2. **Repository Not Found**
   ```yaml
   # Ensure URL is correct and includes .git
   repository:
     url: "https://github.com/your-username/your-repo.git"
   ```

3. **Permission Denied**
   - Verify your token has `repo` permissions
   - Check repository visibility (private repos need appropriate access)

### Check Operator Logs

```bash
# Get operator pod name
kubectl get pods -n git-change-operator-system

# Check logs
kubectl logs -f -n git-change-operator-system deployment/git-change-operator-controller-manager
```

## Next Steps

- [GitCommit with Resource References](gitcommit-resourcerefs.md) - Learn to export Kubernetes resources
- [PullRequest Creation](pullrequest.md) - Create pull requests instead of direct commits
- [Advanced Scenarios](advanced.md) - Complex configurations and use cases

## Cleanup

To remove the GitCommit resource:

```bash
kubectl delete gitcommit basic-commit
```

Note: This only removes the Kubernetes resource, not the Git commits that were already created.