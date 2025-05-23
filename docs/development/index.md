# Development

Welcome to the Git Change Operator development documentation. This section is for contributors and developers who want to understand the internals, contribute code, or extend the operator.

## Getting Started

### [Architecture Overview](architecture.md)
Understand the high-level architecture and design principles of the Git Change Operator.

**Topics covered:**
- Controller architecture
- Resource processing flow
- Git operations workflow
- GitHub API integration

### [Contributing Guidelines](contributing.md)
Learn how to contribute to the Git Change Operator project.

**What you'll find:**
- Code style guidelines
- Pull request process
- Issue reporting
- Development setup

### [Building from Source](building.md)
Instructions for building and developing the operator locally.

**Steps included:**
- Development environment setup
- Building the operator
- Running tests
- Local development workflow

## Development Topics

### [Testing](testing.md)
Comprehensive guide to testing the Git Change Operator.

**Test types:**
- Unit tests
- Integration tests
- End-to-end tests
- Test environment setup

### [Release Process](releases.md)
Documentation of the release process and versioning strategy.

**Includes:**
- Version management
- Release preparation
- Deployment procedures
- Documentation updates

## Project Structure

```
git-change-operator/
├── api/                    # API definitions (CRDs)
│   └── v1/
├── controllers/            # Controller implementations
├── config/                 # Kubernetes configurations
├── docs/                   # Documentation
├── helm/                   # Helm charts
├── test/                   # Test files and resources
└── Makefile               # Build and development tasks
```

## Development Environment

### Prerequisites

- **Go 1.21+** - Programming language
- **kubectl** - Kubernetes CLI
- **Docker** - Container runtime
- **Kind/Minikube** - Local Kubernetes cluster
- **Make** - Build automation

### Quick Development Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/mihaigalos/git-change-operator.git
   cd git-change-operator
   ```

2. **Install dependencies:**
   ```bash
   make setup-test-env
   ```

3. **Run tests:**
   ```bash
   make test-all
   ```

4. **Build and run locally:**
   ```bash
   make build
   make run
   ```

## Contributing

We welcome contributions! Please see our [Contributing Guide](contributing.md) for:

- Code of conduct
- Development workflow
- Coding standards
- Testing requirements
- Documentation guidelines

## Communication

- **Issues:** [GitHub Issues](https://github.com/mihaigalos/git-change-operator/issues)
- **Discussions:** [GitHub Discussions](https://github.com/mihaigalos/git-change-operator/discussions)
- **Pull Requests:** [GitHub Pull Requests](https://github.com/mihaigalos/git-change-operator/pulls)

## Resources

- [Kubernetes Controller Runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [Kubebuilder Documentation](https://book.kubebuilder.io/)
- [Git Go Library](https://github.com/go-git/go-git)
- [GitHub Go API](https://github.com/google/go-github)