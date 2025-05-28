# Release Process

This guide documents the complete release process for the Git Change Operator, including version management, testing, building, and distribution.

## Overview

The Git Change Operator follows semantic versioning and uses an automated release pipeline with manual approval gates for quality assurance.

## Release Workflow


### Version Planning
- **Major Releases** (x.0.0) - Breaking API changes, major features
- **Minor Releases** (x.y.0) - New features, backwards compatible  
- **Patch Releases** (x.y.z) - Bug fixes, security updates

### 2. Release Preparation

#### Create Release Branch
```bash
# For minor/major releases
git checkout main
git pull origin main
git checkout -b release/v1.2.x
git push origin release/v1.2.x
```

#### Update Version Files
```bash
# Update VERSION file
echo "1.2.0" > VERSION

# Update chart version (if applicable)
sed -i 's/version: .*/version: 1.2.0/' charts/git-change-operator/Chart.yaml
sed -i 's/appVersion: .*/appVersion: 1.2.0/' charts/git-change-operator/Chart.yaml

# Update documentation
sed -i 's/git-change-operator:.*/git-change-operator:1.2.0/' docs/user-guide/installation.md
```

## Next Steps

For contributors working on releases:

1. [Building Guide](building.md) - Build release artifacts
2. [Testing Guide](testing.md) - Comprehensive testing procedures  

For users installing releases:

1. [Installation Guide](../user-guide/installation.md) - Install the operator
2. [Quick Start](../user-guide/quick-start.md) - Get started quickly