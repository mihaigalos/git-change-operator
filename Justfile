# Git Change Operator - Justfile
# Run `just --list` to see all available commands

# Extract version info from Helm chart for consistent tagging
chart_version := `grep '^version:' helm/git-change-operator/Chart.yaml | cut -d' ' -f2`
app_version := `grep '^appVersion:' helm/git-change-operator/Chart.yaml | cut -d' ' -f2 | tr -d '"'`
img := "ghcr.io/mihaigalos/git-change-operator:" + app_version + "-" + chart_version
img_latest := "ghcr.io/mihaigalos/git-change-operator:latest"

# Test variables
setup_envtest_index := "https://raw.githubusercontent.com/kubernetes-sigs/controller-tools/HEAD/envtest-releases.yaml"

# Display this help message
help:
    @just --list

# === Development ===

# Format Go code
fmt:
    go fmt ./...

# Run go vet
vet:
    go vet ./...

# Generate CRDs and RBAC manifests
manifests:
    controller-gen crd:crdVersions=v1 rbac:roleName=manager-role paths=./... output:crd:artifacts:config=config/crd/bases/v1 output:rbac:artifacts:config=config/rbac

# Generate Go code (deepcopy, etc.)
generate:
    controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the manager binary
build: fmt vet
    go build -o bin/manager main.go

# Run the manager locally
run: fmt vet
    go run ./main.go

# Clean up generated files and binaries
clean:
    chmod -R +w bin/ 2>/dev/null || true
    rm -rf bin/
    go clean

# === Testing ===

# Run unit tests (default)
test: test-unit

# Run unit tests only
test-unit: fmt vet
    go test -v ./test/unit/...

# Set up test environment (install tools and kubebuilder binaries)
setup-test-env:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Installing setup-envtest tool..."
    go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
    echo "Installing controller-gen tool..."
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
    echo "Creating kubebuilder binary directory..."
    mkdir -p ./bin/kubebuilder
    echo "Downloading kubebuilder binaries..."
    setup-envtest use --index "{{setup_envtest_index}}" --bin-dir ./bin/kubebuilder
    echo "Generating test SSH key pair if not present..."
    test -f test/resources/id_rsa_4096 || ssh-keygen -t rsa -b 4096 -f test/resources/id_rsa_4096 -N "" -C "test-key-for-git-change-operator"

# Run integration tests (requires kubebuilder setup)
test-integration: setup-test-env fmt vet
    #!/usr/bin/env bash
    set -euo pipefail
    KUBEBUILDER_ASSETS=$(setup-envtest use -p path --bin-dir $(pwd)/bin/kubebuilder)
    echo "Using KUBEBUILDER_ASSETS: $KUBEBUILDER_ASSETS"
    KUBEBUILDER_ASSETS=$KUBEBUILDER_ASSETS go test -v ./test/integration/...

# Run all tests (unit + integration)
test-all: setup-test-env fmt vet
    #!/usr/bin/env bash
    set -euo pipefail
    KUBEBUILDER_ASSETS=$(setup-envtest use -p path --bin-dir $(pwd)/bin/kubebuilder)
    echo "Using KUBEBUILDER_ASSETS: $KUBEBUILDER_ASSETS"
    echo "Running unit tests..."
    go test -v ./test/unit/...
    echo "Running integration tests..."
    KUBEBUILDER_ASSETS=$KUBEBUILDER_ASSETS go test -v ./test/integration/...

# === Build and Deploy ===

# Build docker image
docker-build:
    #!/usr/bin/env bash
    set -euo pipefail
    # Source corporate config if present
    if [ -f corporate-config.env ]; then
        echo "Found corporate-config.env, sourcing it..."
        set -a
        source ./corporate-config.env
        set +a
    else
        echo "corporate-config.env not found, proceeding without it"
    fi
    
    # Read corporate certificate if configured
    SSL_CERT_PATH=$(eval echo "${SSL_CERT_FILE:-}")
    if [ -n "$SSL_CERT_PATH" ] && [ -f "$SSL_CERT_PATH" ]; then
        echo "Reading corporate certificate content from $SSL_CERT_PATH..."
        CERT_CONTENT=$(cat "$SSL_CERT_PATH")
    else
        echo "No corporate certificate configured or found, using system certificates"
        CERT_CONTENT=""
    fi
    
    echo "Building Docker image with proxy configuration..."
    GIT_REFERENCE=$(git config --get remote.origin.url | sed -E 's/^(git@|https:\/\/)([^:\/]+)[:\/](.+)\.git$/https:\/\/\2\/\3/')/commit/$(git rev-parse HEAD)
    
    docker build --progress=plain --network=host \
        --build-arg HTTP_PROXY="${HTTP_PROXY:-}" \
        --build-arg HTTPS_PROXY="${HTTPS_PROXY:-}" \
        --build-arg NO_PROXY="${NO_PROXY:-}" \
        --build-arg http_proxy="${http_proxy:-}" \
        --build-arg https_proxy="${https_proxy:-}" \
        --build-arg no_proxy="${no_proxy:-}" \
        ${GOPROXY:+--build-arg GOPROXY="$GOPROXY"} \
        ${GOSUMDB:+--build-arg GOSUMDB="$GOSUMDB"} \
        ${GONOPROXY:+--build-arg GONOPROXY="$GONOPROXY"} \
        ${GONOSUMDB:+--build-arg GONOSUMDB="$GONOSUMDB"} \
        ${APK_MAIN_REPO:+--build-arg APK_MAIN_REPO="$APK_MAIN_REPO"} \
        ${APK_COMMUNITY_REPO:+--build-arg APK_COMMUNITY_REPO="$APK_COMMUNITY_REPO"} \
        --build-arg CORPORATE_CA_CERT="$CERT_CONTENT" \
        --build-arg GIT_REFERENCE="$GIT_REFERENCE" \
        -t {{img}} -t {{img_latest}} .

# Push docker image
docker-push:
    docker push {{img}}
    docker push {{img_latest}}

# Install CRDs and RBAC to the cluster
install:
    kubectl apply -k config/

# Uninstall CRDs and RBAC from the cluster
uninstall:
    kubectl delete -k config/

# Build, push and deploy to cluster
deploy: docker-build docker-push install

# Undeploy from cluster
undeploy: uninstall

# === Kind Development ===

# Create Kind cluster with corporate proxy support
kind-create:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "🚀 Creating Kind cluster with corporate CA support..."
    if [ ! -f "~/certs/zscaler.pem" ]; then
        echo "⚠️  Corporate CA certificate not found at ~/certs/zscaler.pem"
        echo "   Continuing without corporate CA (cluster may have network issues)"
    fi
    # Delete existing cluster if present
    kind delete cluster --name git-change-operator 2>/dev/null || true
    kind create cluster --name git-change-operator --config kind-config.yaml
    echo "✅ Kind cluster created successfully!"

# Deploy git-change-operator to Kind cluster
kind-deploy:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    echo "🔧 Deploying git-change-operator to Kind cluster..."
    echo "✅ Cluster status:"
    kubectl cluster-info --context kind-git-change-operator
    echo ""
    echo "📦 Installing git-change-operator with conditional CRDs..."
    helm upgrade --install git-change-operator helm/git-change-operator \
        --namespace git-change-operator --create-namespace \
        --set crds.install=true \
        --kube-context kind-git-change-operator
    echo "✅ Operator deployed successfully!"

# Interactively create GitHub token secret
kube-setup-token context="kind-git-change-operator" namespace="git-change-operator":
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    echo "🔑 Setting up GitHub authentication..."
    echo "Using context: {{context}}"
    echo "Using namespace: {{namespace}}"
    echo ""
    
    if [ -f "token" ]; then
        echo "📄 Reading GitHub token from token file..."
        GITHUB_TOKEN=$(cat token)
    else
        echo "Please enter your GitHub personal access token:"
        read -s GITHUB_TOKEN
    fi
    
    if [ -z "$GITHUB_TOKEN" ]; then
        echo "❌ No token provided, skipping secret creation"
        exit 1
    fi
    
    echo ""
    echo "Creating git-credentials secret in {{namespace}} namespace..."
    kubectl create secret generic git-credentials \
        --from-literal=token=$GITHUB_TOKEN \
        --namespace {{namespace}} \
        --context {{context}} \
        --dry-run=client -o yaml | kubectl apply -f -
    echo "✅ GitHub token secret created successfully!"

# Create GitHub token secret for Kind cluster
kind-setup-token:
    @just kube-setup-token kind-git-change-operator git-change-operator

# Load local Docker image into Kind cluster
kind-load-image:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    export DOCKER_API_VERSION=1.43
    echo "📦 Loading local operator image into Kind cluster..."
    docker save {{img}} -o /tmp/operator-image.tar
    kind load image-archive /tmp/operator-image.tar --name git-change-operator
    rm -f /tmp/operator-image.tar
    echo "✅ Image loaded into Kind cluster"

# Restart operator deployment to pick up new image
kind-restart-operator:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    echo "🔄 Restarting operator deployment..."
    kubectl scale deployment git-change-operator-controller-manager --replicas=0 -n git-change-operator --context kind-git-change-operator
    sleep 2
    kubectl scale deployment git-change-operator-controller-manager --replicas=1 -n git-change-operator --context kind-git-change-operator
    echo "✅ Operator deployment restarted"

# Build operator image and test in Kind
kind-build-and-test: docker-build kind-load-image kind-restart-operator
    @echo "🎉 Operator built and loaded into Kind cluster!"
    @echo "   Monitor operator startup: kubectl logs -f deployment/git-change-operator-controller-manager -n git-change-operator --context kind-git-change-operator"

# Show Kind cluster and operator status
kind-status:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    echo "📊 Kind Cluster Status"
    echo "======================"
    
    echo "🔗 Cluster Info:"
    kubectl cluster-info --context kind-git-change-operator || echo "❌ Cluster not accessible"
    echo ""
    
    echo "🏷️  Nodes:"
    kubectl get nodes --context kind-git-change-operator || echo "❌ Cannot get nodes"
    echo ""
    
    echo "🎯 Operator Pods:"
    kubectl get pods -n git-change-operator --context kind-git-change-operator || echo "❌ Operator not deployed"
    echo ""
    
    echo "🎯 GitCommits:"
    kubectl get gitcommit -n git-change-operator --context kind-git-change-operator || echo "📝 No GitCommit resources found"
    echo ""
    
    # Check detailed status if GitCommits exist
    GITCOMMIT_NAME=$(kubectl get gitcommit -n git-change-operator --context kind-git-change-operator -o name 2>/dev/null | head -1)
    if [ -n "$GITCOMMIT_NAME" ]; then
        echo "🔍 GitCommit Status Details:"
        kubectl get $GITCOMMIT_NAME -n git-change-operator --context kind-git-change-operator -o yaml | grep -A 10 "^status:"
    fi

# Complete Kind demo workflow
kind-full-demo: kind-create kind-deploy kind-setup-token kind-build-and-test kind-status
    @echo ""
    @echo "🎉 Complete Kind Demo Workflow Finished!"
    @echo "========================================"
    @echo ""
    @echo "✅ Kind cluster created"
    @echo "✅ git-change-operator deployed"
    @echo "✅ GitHub token configured"
    @echo "✅ Operator image built and loaded"
    @echo ""
    @echo "🔍 Next steps:"
    @echo "   • Apply a GitCommit: kubectl apply -f examples/gitcommit_example.yaml --context kind-git-change-operator"
    @echo "   • Monitor logs: kubectl logs -f deployment/git-change-operator-controller-manager -n git-change-operator --context kind-git-change-operator"
    @echo "   • Clean up: just kind-destroy"

# Delete Kind cluster
kind-destroy:
    #!/usr/bin/env bash
    echo "🧹 Cleaning up Kind cluster..."
    kind delete cluster --name git-change-operator || echo "⚠️  Cluster already deleted"
    echo "✅ Kind cluster cleaned up!"

# === Helm ===

# Lint the Helm chart
helm-lint:
    helm lint helm/git-change-operator

# Generate Kubernetes manifests from Helm chart
helm-template:
    helm template git-change-operator helm/git-change-operator

# Package the Helm chart
helm-package:
    helm package helm/git-change-operator -d helm/

# Install the operator using Helm
helm-install:
    helm upgrade --install git-change-operator helm/git-change-operator --create-namespace --namespace git-change-operator-system

# Uninstall the operator using Helm
helm-uninstall:
    helm uninstall git-change-operator --namespace git-change-operator-system

# Build, push, package and deploy using Helm
helm-deploy: docker-build docker-push helm-package helm-install

# === Documentation ===

# Create Python virtual environment for documentation
docs-venv:
    #!/usr/bin/env bash
    python3 -m venv docs/.venv
    echo "Virtual environment created at docs/.venv"

# Install documentation dependencies
docs-deps: docs-venv
    #!/usr/bin/env bash
    docs/.venv/bin/pip install --upgrade pip
    docs/.venv/bin/pip install -r docs/mkdocs/requirements.txt

# Prepare README for MkDocs
docs-prepare:
    #!/usr/bin/env bash
    echo "Backing up README.md and creating MkDocs-compatible version..."
    cp README.md README.md.bak
    sed -i.tmp 's|](docs/|](|g' README.md && rm README.md.tmp
    echo "README.md prepared for MkDocs (backup at README.md.bak)"

# Restore original README.md
docs-restore:
    #!/usr/bin/env bash
    if [ -f README.md.bak ]; then
        echo "Restoring original README.md..."
        mv README.md.bak README.md
        echo "README.md restored"
    else
        echo "No backup found, skipping restore"
    fi

# Serve documentation locally for development
docs-serve: docs-deps docs-prepare
    #!/usr/bin/env bash
    docs/.venv/bin/mkdocs serve || true
    just docs-restore

# Serve versioned documentation locally
docs-serve-versioned: docs-deps docs-prepare
    #!/usr/bin/env bash
    source docs/.venv/bin/activate && mike serve --dev-addr=127.0.0.1:8001 || true
    just docs-restore

# Build documentation for production
docs-build: docs-deps docs-prepare
    #!/usr/bin/env bash
    docs/.venv/bin/mkdocs build
    just docs-restore

# Deploy documentation to GitHub Pages
docs-deploy: docs-deps
    docs/.venv/bin/mkdocs gh-deploy --force

# Deploy a new version of documentation
docs-version-deploy version: docs-deps docs-prepare
    #!/usr/bin/env bash
    source docs/.venv/bin/activate && mike deploy --push --update-aliases {{version}} latest
    just docs-restore

# Set default version for documentation
docs-version-set-default version: docs-deps
    docs/.venv/bin/mike set-default {{version}}

# List all deployed documentation versions
docs-version-list: docs-deps
    docs/.venv/bin/mike list

# Clean built documentation and virtual environment
docs-clean:
    rm -rf site/
    rm -rf docs/.venv/
