# Git Change Operator - Justfile
# Run `just --list` to see all available commands

# Extract version info from Helm chart for consistent tagging
chart_version := `grep '^version:' helm/git-change-operator/Chart.yaml | cut -d' ' -f2`
app_version := `grep '^appVersion:' helm/git-change-operator/Chart.yaml | cut -d' ' -f2 | tr -d '"'`
img := "ghcr.io/mihaigalos/git-change-operator:" + app_version + "-" + chart_version
img_latest := "ghcr.io/mihaigalos/git-change-operator:latest"
setup_envtest_index := "https://raw.githubusercontent.com/kubernetes-sigs/controller-tools/HEAD/envtest-releases.yaml"
setup_envtest_k8s_version := "1.35.0"

# Display this help message
[group('help')]
help:
    @just --list

# === Development ===

# Format and lint Go code
[group('dev')]
check:
    go fmt ./...
    go vet ./...

# Generate CRDs and DeepCopy code
# Note: RBAC is manually maintained in config/rbac/rbac.yaml as namespace-scoped Role
# (controller-gen only generates ClusterRole, which we don't use)
[group('dev')]
codegen:
    controller-gen crd paths="./api/v1" output:crd:artifacts:config=config/crd/bases
    controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the manager binary
[group('dev')]
build: check
    go build -o bin/manager main.go

# Run the manager locally
[group('dev')]
run: check
    go run ./main.go

# Clean up generated files and binaries
[group('dev')]
clean:
    chmod -R +w bin/ 2>/dev/null || true
    rm -rf bin/
    go clean

# === Testing ===

# Set up test environment (kubebuilder binaries and tools)
[group('test')]
setup-test-env:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Installing tools..."
    command -v setup-envtest &>/dev/null || go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
    command -v controller-gen &>/dev/null || go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
    mkdir -p ./bin/kubebuilder
    setup-envtest use {{setup_envtest_k8s_version}} --bin-dir ./bin/kubebuilder
    test -f test/resources/id_rsa_4096 || ssh-keygen -t rsa -b 4096 -f test/resources/id_rsa_4096 -N "" -C "test-key-for-git-change-operator"

# Run integration tests
[group('test')]
test-integration: setup-test-env check
    #!/usr/bin/env bash
    set -euo pipefail
    KUBEBUILDER_ASSETS=$(setup-envtest use {{setup_envtest_k8s_version}} -p path --bin-dir $(pwd)/bin/kubebuilder)
    KUBEBUILDER_ASSETS=$KUBEBUILDER_ASSETS go test -v ./test/integration/...

# Run all tests (unit + integration)
[group('test')]
test: setup-test-env check
    #!/usr/bin/env bash
    set -euo pipefail
    KUBEBUILDER_ASSETS=$(setup-envtest use {{setup_envtest_k8s_version}} -p path --bin-dir $(pwd)/bin/kubebuilder)
    go test -v ./test/unit/...
    KUBEBUILDER_ASSETS=$KUBEBUILDER_ASSETS go test -v ./test/integration/...

# === Docker ===

# Build Docker image using BuildKit with secret mounting
[group('docker')]
docker-build:
    #!/usr/bin/env bash
    set -euo pipefail
    
    # Enable Docker BuildKit for secret mounting support
    export DOCKER_BUILDKIT=1
    
    # Load corporate config if available
    if [ -f corporate-config.env ]; then
        set -a; source ./corporate-config.env; set +a
    fi
    
    # Prepare certificate secret mount if certificate exists
    SSL_CERT_PATH=$(eval echo "${SSL_CERT_FILE:-}")
    if [ -n "$SSL_CERT_PATH" ] && [ -f "$SSL_CERT_PATH" ]; then
        CERT_SECRET_ARG="--secret id=corporate_cert,src=$SSL_CERT_PATH"
        echo "🔒 Using corporate certificate from: $SSL_CERT_PATH (mounted as secret - not embedded)"
    else
        CERT_SECRET_ARG=""
        echo "ℹ️  No corporate certificate configured - using system CAs only"
    fi
    
    # Generate git reference for image labels
    GIT_REFERENCE=$(git config --get remote.origin.url | sed -E 's/^(git@|https:\/\/)([^:\/]+)[:\/](.+)\.git$/https:\/\/\2\/\3/')/commit/$(git rev-parse HEAD)
    
    echo "🔨 Building with distroless base image (no shell, minimal attack surface)"
    
    # Build with BuildKit - certificate passed as secret (not in image layers)
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
        --build-arg GIT_REFERENCE="$GIT_REFERENCE" \
        $CERT_SECRET_ARG \
        -t {{img}} -t {{img_latest}} .
    
    echo "✅ Image built with distroless base - NO shell, NO corporate cert in layers"

# Push Docker image
[group('docker')]
docker-push:
    docker push {{img}}
    docker push {{img_latest}}

# Build and push Docker image
[group('docker')]
docker-publish: docker-build docker-push

# Verify Docker image security (no leaked secrets, distroless base)
[group('docker')]
docker-verify image=img:
    ./scripts/verify-image-security.sh {{image}}

# === Deployment ===

# Install operator to cluster (via Kustomize)
[group('deploy')]
install:
    kubectl apply -k config/

# Uninstall operator from cluster
[group('deploy')]
uninstall:
    kubectl delete -k config/

# Complete deployment workflow (build, push, install)
[group('deploy')]
deploy: docker-publish install

# === Kind Cluster ===

# Create Kind cluster
[group('kind')]
kind-create:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "🚀 Creating Kind cluster..."
    kind delete cluster --name git-change-operator 2>/dev/null || true
    kind create cluster --name git-change-operator --config kind-config.yaml
    echo "✅ Cluster created!"

# Deploy operator to Kind cluster
[group('kind')]
kind-deploy:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    echo "📦 Deploying operator..."
    kubectl cluster-info --context kind-git-change-operator
    helm upgrade --install git-change-operator helm/git-change-operator \
        --namespace git-change-operator --create-namespace \
        --set crds.install=true \
        --kube-context kind-git-change-operator
    echo "✅ Deployed!"

# Setup GitHub token secret
[group('kind')]
kind-setup-token:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    echo "🔑 Setting up GitHub token..."
    if [ -f "token" ]; then
        GITHUB_TOKEN=$(cat token)
    else
        echo "Enter GitHub token:"; read -s GITHUB_TOKEN
    fi
    [ -z "$GITHUB_TOKEN" ] && echo "❌ No token" && exit 1
    kubectl create secret generic git-credentials \
        --from-literal=token=$GITHUB_TOKEN \
        --namespace git-change-operator \
        --context kind-git-change-operator \
        --dry-run=client -o yaml | kubectl apply -f -
    echo "✅ Token configured!"

# Load local image into Kind cluster
[group('kind')]
kind-load-image:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    export DOCKER_API_VERSION=1.43
    echo "📦 Loading image..."
    docker save {{img}} -o /tmp/operator-image.tar
    kind load image-archive /tmp/operator-image.tar --name git-change-operator
    rm -f /tmp/operator-image.tar
    echo "✅ Image loaded!"

# Restart operator in Kind
[group('kind')]
kind-restart:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    echo "🔄 Restarting operator..."
    kubectl scale deployment git-change-operator-controller-manager --replicas=0 -n git-change-operator --context kind-git-change-operator
    sleep 2
    kubectl scale deployment git-change-operator-controller-manager --replicas=1 -n git-change-operator --context kind-git-change-operator
    echo "✅ Restarted!"

# Build and test in Kind (build + verify + load + restart)
[group('kind')]
kind-dev: docker-build docker-verify kind-load-image kind-restart
    @echo "🎉 Ready! Monitor: kubectl logs -f deployment/git-change-operator-controller-manager -n git-change-operator --context kind-git-change-operator"

# Show Kind cluster status
[group('kind')]
kind-status:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    echo "📊 Status"; echo "=========="
    kubectl cluster-info --context kind-git-change-operator || echo "❌ Not accessible"
    echo ""; echo "🏷️  Nodes:"
    kubectl get nodes --context kind-git-change-operator || echo "❌ No nodes"
    echo ""; echo "🎯 Pods:"
    kubectl get pods -n git-change-operator --context kind-git-change-operator || echo "❌ Not deployed"
    echo ""; echo "📝 GitCommits:"
    kubectl get gitcommit -n git-change-operator --context kind-git-change-operator || echo "None found"

# Test GitChangeOperator functionality in Kind
[group('kind')]
kind-test:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="/opt/homebrew/bin:$PATH"
    echo "🧪 Testing Git Change Operator in Kind..."
    echo ""
    echo "📋 Reloading CRDs..."
    kubectl apply -f config/crd/bases/gco.galos.one_gitchangeoperators.yaml --context kind-git-change-operator
    kubectl apply -f config/crd/bases/gco.galos.one_gitcommits.yaml --context kind-git-change-operator
    kubectl apply -f config/crd/bases/gco.galos.one_pullrequests.yaml --context kind-git-change-operator
    echo ""
    echo "1️⃣ Checking operator pod..."
    kubectl get pods -n git-change-operator --context kind-git-change-operator -l control-plane=controller-manager
    echo ""
    echo "2️⃣ Checking GitChangeOperator CR..."
    kubectl get gitchangeoperator -n git-change-operator --context kind-git-change-operator || echo "⚠️  No GitChangeOperator CR found"
    echo ""
    echo "3️⃣ Checking metrics service..."
    kubectl get svc -n git-change-operator --context kind-git-change-operator || echo "⚠️  No services found"
    echo ""
    echo "4️⃣ Creating test GitCommit to public repo..."
    cat << 'EOF' | kubectl apply --context kind-git-change-operator -f -
    apiVersion: gco.galos.one/v1
    kind: GitCommit
    metadata:
      name: test-commit-{{chart_version}}
      namespace: git-change-operator
    spec:
      repository: https://github.com/mihaigalos/test.git
      branch: test-kind-{{chart_version}}
      commitMessage: "Test from Kind cluster v{{chart_version}} - distroless image"
      authSecretRef: git-credentials
      authSecretKey: token
      files:
      - path: test-kind-{{chart_version}}.txt
        content: |
          # Test from Kind Cluster
          
          Version: {{chart_version}}
          Image: {{img}}
          Security:
          - Distroless base (no shell)
          - BuildKit secrets (cert not in layers)
          - Non-root user (UID 65532)
    EOF
    echo ""
    echo "5️⃣ Monitoring GitCommit status (5s)..."
    sleep 5
    kubectl get gitcommit test-commit-{{chart_version}} -n git-change-operator --context kind-git-change-operator || true
    echo ""
    echo "6️⃣ Checking operator logs (last 30 lines via crictl)..."
    CONTAINER_ID=$(docker exec git-change-operator-control-plane crictl ps | grep 'git-change-operator-controller-manager' | awk '{print $1}')
    if [ -n "$CONTAINER_ID" ]; then
        docker exec git-change-operator-control-plane crictl logs --tail 30 $CONTAINER_ID | tail -35
    else
        echo "⚠️  Could not find running manager container"
    fi
    echo ""
    echo "✅ Test complete!"
    echo ""
    echo "📝 View details: kubectl describe gitcommit test-commit-{{chart_version}} -n git-change-operator --context kind-git-change-operator"
    echo "📝 Watch logs: CONTAINER_ID=\$(docker exec git-change-operator-control-plane crictl ps | grep manager | awk '{print \$1}') && docker exec git-change-operator-control-plane crictl logs -f \$CONTAINER_ID"

# Complete Kind setup (create + deploy + token + dev + test)
[group('kind')]
kind-up: kind-create kind-deploy kind-setup-token kind-dev kind-test kind-status
    @echo ""; echo "🎉 Kind cluster ready!"
    @echo "Apply example: kubectl apply -f examples/gitcommit_example.yaml --context kind-git-change-operator"

# Destroy Kind cluster
[group('kind')]
kind-down:
    #!/usr/bin/env bash
    echo "🧹 Deleting cluster..."
    kind delete cluster --name git-change-operator || echo "Already deleted"
    echo "✅ Cleaned up!"

# === Helm ===

# Lint Helm chart
[group('helm')]
helm-lint:
    helm lint helm/git-change-operator

# Preview Helm manifests
[group('helm')]
helm-template:
    helm template git-change-operator helm/git-change-operator

# Package Helm chart
[group('helm')]
helm-package:
    helm package helm/git-change-operator -d helm/

# Install via Helm
[group('helm')]
helm-install:
    helm upgrade --install git-change-operator helm/git-change-operator \
        --create-namespace --namespace git-change-operator-system

# Uninstall via Helm
[group('helm')]
helm-uninstall:
    helm uninstall git-change-operator --namespace git-change-operator-system

# Deploy via Helm (build + push + package + install)
[group('helm')]
helm-deploy: docker-publish helm-package helm-install

# === Documentation ===

# Setup documentation environment
[group('docs')]
docs-setup:
    #!/usr/bin/env bash
    python3 -m venv docs/.venv
    docs/.venv/bin/pip install --upgrade pip
    docs/.venv/bin/pip install -r docs/mkdocs/requirements.txt

# Serve documentation locally
[group('docs')]
docs-serve: docs-setup
    #!/usr/bin/env bash
    cp README.md README.md.bak
    sed -i.tmp 's|](docs/|](|g' README.md && rm README.md.tmp
    docs/.venv/bin/mkdocs serve || true
    mv README.md.bak README.md 2>/dev/null || true

# Build documentation
[group('docs')]
docs-build: docs-setup
    #!/usr/bin/env bash
    cp README.md README.md.bak
    sed -i.tmp 's|](docs/|](|g' README.md && rm README.md.tmp
    docs/.venv/bin/mkdocs build
    mv README.md.bak README.md 2>/dev/null || true

# Deploy documentation to GitHub Pages
[group('docs')]
docs-deploy: docs-setup
    docs/.venv/bin/mkdocs gh-deploy --force

# Deploy versioned documentation
[group('docs')]
docs-version version: docs-setup
    #!/usr/bin/env bash
    cp README.md README.md.bak
    sed -i.tmp 's|](docs/|](|g' README.md && rm README.md.tmp
    source docs/.venv/bin/activate && mike deploy --push --update-aliases {{version}} latest
    mv README.md.bak README.md 2>/dev/null || true

# Clean documentation artifacts
[group('docs')]
docs-clean:
    rm -rf site/ docs/.venv/

# === Legacy Aliases ===

# Legacy alias for manifests (use 'codegen' instead)
manifests: codegen

# Legacy alias for vet (use 'check' instead)
vet: check

# Legacy alias for generate (use 'codegen' instead)
generate: codegen

# Legacy alias for test-unit (use 'test' instead)
test-unit: test

# Legacy alias for kind-build-and-test (use 'kind-dev' instead)
kind-build-and-test: kind-dev

# Legacy alias for kind-full-demo (use 'kind-up' instead)
kind-full-demo: kind-up

# Legacy alias for kind-destroy (use 'kind-down' instead)
kind-destroy: kind-down
