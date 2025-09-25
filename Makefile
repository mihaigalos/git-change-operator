SHELL := /bin/bash

# Extract version info from Helm chart for consistent tagging
CHART_VERSION := $(shell grep '^version:' helm/git-change-operator/Chart.yaml | cut -d' ' -f2)
APP_VERSION := $(shell grep '^appVersion:' helm/git-change-operator/Chart.yaml | cut -d' ' -f2 | tr -d '"')
IMG ?= ghcr.io/mihaigalos/git-change-operator:$(APP_VERSION)-$(CHART_VERSION)
IMG_LATEST ?= ghcr.io/mihaigalos/git-change-operator:latest

KUBEBUILDER_ASSETS ?= $(shell pwd)/bin/kubebuilder/k8s/1.34.1-darwin-arm64
SETUP_ENVTEST_INDEX ?= https://raw.githubusercontent.com/kubernetes-sigs/controller-tools/HEAD/envtest-releases.yaml

# Go proxy configuration (can be overridden via environment variables)
GOPROXY_ARG ?= $(GOPROXY)
GOSUMDB_ARG ?= $(GOSUMDB)
GONOPROXY_ARG ?= $(GONOPROXY)
GONOSUMDB_ARG ?= $(GONOSUMDB)

# Alpine repository configuration for corporate environments
APK_MAIN_REPO_ARG ?= $(APK_MAIN_REPO)
APK_COMMUNITY_REPO_ARG ?= $(APK_COMMUNITY_REPO)

all: build

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development
fmt: ## Format Go code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

manifests: ## Generate CRDs and RBAC manifests
	controller-gen crd:crdVersions=v1 rbac:roleName=manager-role paths=./... output:crd:artifacts:config=config/crd/bases/v1 output:rbac:artifacts:config=config/rbac

generate: ## Generate Go code (deepcopy, etc.)
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

build: fmt vet ## Build the manager binary
	go build -o bin/manager main.go

run: fmt vet ## Run the manager locally
	go run ./main.go

clean: ## Clean up generated files and binaries
	chmod -R +w bin/ 2>/dev/null || true
	rm -rf bin/
	go clean

##@ Testing
test: test-unit ## Run unit tests (default)

test-unit: fmt vet ## Run unit tests only
	go test -v ./test/*unit_test.go

setup-test-env: ## Set up test environment (install tools and kubebuilder binaries)
	@echo "Installing setup-envtest tool..."
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	@echo "Installing controller-gen tool..."
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@echo "Creating kubebuilder binary directory..."
	mkdir -p ./bin/kubebuilder
	@echo "Downloading kubebuilder binaries using proxy..."
	setup-envtest use --index "$(SETUP_ENVTEST_INDEX)" --bin-dir ./bin/kubebuilder
	@echo "Setup complete! KUBEBUILDER_ASSETS will be set to: $(KUBEBUILDER_ASSETS)"

test-integration: setup-test-env fmt vet ## Run integration tests (requires kubebuilder setup)
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test -v ./test/suite_test.go ./test/gitcommit_controller_test.go ./test/pullrequest_controller_test.go

test-all: setup-test-env fmt vet ## Run all tests (unit + integration)
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test -v ./test/...

##@ Build and Deploy
# Have a look at docs/examples/corporate-setup.md
# Copy corporate-config.env.example to corporate-config.env and customize
# Then run: source corporate-config.env
docker-build: ## Build docker image
	@if [ -f corporate-config.env ]; then \
		echo "Found corporate-config.env, sourcing it..."; \
		source corporate-config.env; \
	else \
		echo "corporate-config.env not found, proceeding without it"; \
	fi
	@if [ -n "${SSL_CERT_FILE}" ] && [ -f "${SSL_CERT_FILE}" ]; then \
		echo "Reading corporate certificate content from ${SSL_CERT_FILE}..."; \
		CERT_CONTENT=$$(cat "${SSL_CERT_FILE}"); \
	else \
		echo "No corporate certificate configured or found, using system certificates"; \
		CERT_CONTENT=""; \
	fi; \
	echo "Building Docker image with proxy configuration..."; \
	GIT_REFERENCE=$$(git config --get remote.origin.url | sed -E 's/^(git@|https:\/\/)([^:\/]+)[:\/](.+)\.git$$/https:\/\/\2\/\3/')/commit/$$(git rev-parse HEAD); \
	docker build --progress=plain --network=host \
		--build-arg HTTP_PROXY="${HTTP_PROXY}" \
		--build-arg HTTPS_PROXY="${HTTPS_PROXY}" \
		--build-arg NO_PROXY="${NO_PROXY}" \
		--build-arg http_proxy="${http_proxy}" \
		--build-arg https_proxy="${https_proxy}" \
		--build-arg no_proxy="${no_proxy}" \
		$(if $(GOPROXY_ARG),--build-arg GOPROXY="$(GOPROXY_ARG)") \
		$(if $(GOSUMDB_ARG),--build-arg GOSUMDB="$(GOSUMDB_ARG)") \
		$(if $(GONOPROXY_ARG),--build-arg GONOPROXY="$(GONOPROXY_ARG)") \
		$(if $(GONOSUMDB_ARG),--build-arg GONOSUMDB="$(GONOSUMDB_ARG)") \
		$(if $(APK_MAIN_REPO_ARG),--build-arg APK_MAIN_REPO="$(APK_MAIN_REPO_ARG)") \
		$(if $(APK_COMMUNITY_REPO_ARG),--build-arg APK_COMMUNITY_REPO="$(APK_COMMUNITY_REPO_ARG)") \
		--build-arg CORPORATE_CA_CERT="$$CERT_CONTENT" \
		--build-arg GIT_REFERENCE="$$GIT_REFERENCE" \
		-t ${IMG} -t ${IMG_LATEST} .

docker-push: ## Push docker image
	docker push ${IMG}
	docker push ${IMG_LATEST}

install: ## Install CRDs and RBAC to the cluster
	kubectl apply -k config/

uninstall: ## Uninstall CRDs and RBAC from the cluster
	kubectl delete -k config/

deploy: docker-build docker-push install ## Build, push and deploy to cluster

undeploy: uninstall ## Undeploy from cluster

##@ Kind Development
kind-create: # Create Kind cluster with corporate proxy support (hidden)
	@echo "🚀 Creating Kind cluster with corporate CA support..."
	@if [ ! -f "~/certs/zscaler.pem" ]; then \
		echo "⚠️  Corporate CA certificate not found at ~/certs/zscaler.pem"; \
		echo "   Continuing without corporate CA (cluster may have network issues)"; \
	fi
	kind create cluster --name git-change-operator --config kind-config.yaml
	@echo "✅ Kind cluster created successfully!"
	@echo "   Updating PATH to use homebrew kubectl..."
	@echo "   Run: export PATH=\"/opt/homebrew/bin:\$$PATH\""

kind-deploy: # Deploy git-change-operator to Kind cluster with interactive token setup (hidden)
	@echo "🔧 Deploying git-change-operator to Kind cluster..."
	@echo "   Ensuring PATH uses homebrew kubectl..."
	@export PATH="/opt/homebrew/bin:$$PATH"; \
	echo "✅ Cluster status:"; \
	kubectl cluster-info --context kind-git-change-operator; \
	echo; \
	echo "📦 Installing git-change-operator with conditional CRDs..."; \
	helm upgrade --install git-change-operator helm/git-change-operator \
		--namespace git-change-operator --create-namespace \
		--set crds.install=true \
		--kube-context kind-git-change-operator; \
	echo "✅ Operator deployed successfully!"; \
	echo; \
	echo "🔑 Now let's set up your GitHub token for testing..."

kube-setup-token: # Interactively create GitHub token secret for any Kubernetes context (hidden)
	@echo "🔑 Setting up GitHub authentication..."
	@export PATH="/opt/homebrew/bin:$$PATH"; \
	KUBE_CONTEXT=$${KUBE_CONTEXT:-$$(kubectl config current-context)}; \
	KUBE_NAMESPACE=$${KUBE_NAMESPACE:-git-change-operator}; \
	echo "Using context: $$KUBE_CONTEXT"; \
	echo "Using namespace: $$KUBE_NAMESPACE"; \
	echo; \
	if [ -f "token" ]; then \
		echo "📄 Reading GitHub token from token file..."; \
		GITHUB_TOKEN=$$(cat token); \
	else \
		echo "Please enter your GitHub personal access token:"; \
		read -s GITHUB_TOKEN; \
	fi; \
	if [ -z "$$GITHUB_TOKEN" ]; then \
		echo "❌ No token provided, skipping secret creation"; \
		exit 1; \
	fi; \
	echo; \
	echo "Creating git-credentials secret in $$KUBE_NAMESPACE namespace..."; \
	kubectl create secret generic git-credentials \
		--from-literal=token=$$GITHUB_TOKEN \
		--namespace $$KUBE_NAMESPACE \
		--context $$KUBE_CONTEXT \
		--dry-run=client -o yaml | kubectl apply -f -; \
	echo "✅ GitHub token secret created successfully!"

kind-setup-token: # Create GitHub token secret for Kind cluster (hidden)
	@echo "🔑 Setting up GitHub token for Kind cluster..."
	@KUBE_CONTEXT="kind-git-change-operator" KUBE_NAMESPACE="git-change-operator" $(MAKE) kube-setup-token

kind-patch-operator: # Patch operator deployment to disable it for CRD-only testing (hidden)
	@echo "🔧 Patching operator deployment for testing..."
	@export PATH="/opt/homebrew/bin:$$PATH"; \
	echo "Scaling down operator deployment for CRD-only testing..."; \
	kubectl scale deployment git-change-operator-controller-manager --replicas=0 -n git-change-operator --context kind-git-change-operator; \
	echo "✅ Operator deployment scaled to 0. CRDs are available for testing without operator processing."

kind-demo: # Create a demo GitCommit resource (hidden)
	@echo "🎯 Creating demo GitCommit resource..."
	@export PATH="/opt/homebrew/bin:$$PATH"; \
	echo "apiVersion: gco.galos.one/v1" > /tmp/demo-gitcommit.yaml; \
	echo "kind: GitCommit" >> /tmp/demo-gitcommit.yaml; \
	echo "metadata:" >> /tmp/demo-gitcommit.yaml; \
	echo "  name: demo-commit-$$(date +%s)" >> /tmp/demo-gitcommit.yaml; \
	echo "  namespace: git-change-operator" >> /tmp/demo-gitcommit.yaml; \
	echo "spec:" >> /tmp/demo-gitcommit.yaml; \
	echo "  repository: \"https://github.com/mihaigalos/test\"" >> /tmp/demo-gitcommit.yaml; \
	echo "  branch: \"main\"" >> /tmp/demo-gitcommit.yaml; \
	echo "  commitMessage: \"Demo commit from Kind cluster - $$(date)\"" >> /tmp/demo-gitcommit.yaml; \
	echo "  authSecretRef: \"git-credentials\"" >> /tmp/demo-gitcommit.yaml; \
	echo "  files:" >> /tmp/demo-gitcommit.yaml; \
	echo "  - path: \"demo-$$(date +%Y%m%d-%H%M%S).txt\"" >> /tmp/demo-gitcommit.yaml; \
	echo "    content: |" >> /tmp/demo-gitcommit.yaml; \
	echo "      Hello from Kind cluster!" >> /tmp/demo-gitcommit.yaml; \
	echo "      Created at: $$(date)" >> /tmp/demo-gitcommit.yaml; \
	echo "      Cluster: kind-git-change-operator" >> /tmp/demo-gitcommit.yaml; \
	echo; \
	echo "📄 Demo GitCommit manifest:"; \
	cat /tmp/demo-gitcommit.yaml; \
	echo; \
	echo "🚀 Applying GitCommit resource..."; \
	kubectl apply -f /tmp/demo-gitcommit.yaml --context kind-git-change-operator; \
	echo "✅ Demo GitCommit created! Check status with:"; \
	echo "   kubectl get gitcommit -n git-change-operator --context kind-git-change-operator"; \
	echo; \
	echo "ℹ️  Note: Operator pod is not running, so GitCommit won't be processed."; \
	echo "   To build and run the operator: make docker-build kind-load-image kind-restart-operator"

kind-load-image: # Load local Docker image into Kind cluster (hidden)
	@echo "📦 Loading local operator image into Kind cluster..."
	@export PATH="/opt/homebrew/bin:$$PATH"; \
	kind load docker-image ${IMG} --name git-change-operator; \
	echo "✅ Image loaded into Kind cluster"

kind-restart-operator: # Restart operator deployment to pick up new image (hidden)
	@echo "🔄 Restarting operator deployment..."
	@export PATH="/opt/homebrew/bin:$$PATH"; \
	kubectl scale deployment git-change-operator-controller-manager --replicas=0 -n git-change-operator --context kind-git-change-operator; \
	sleep 2; \
	kubectl scale deployment git-change-operator-controller-manager --replicas=1 -n git-change-operator --context kind-git-change-operator; \
	echo "✅ Operator deployment restarted"

kind-build-and-test: docker-build kind-load-image kind-restart-operator # Build operator image and test in Kind (hidden)
	@echo "🎉 Operator built and loaded into Kind cluster!"
	@echo "   Monitor operator startup: kubectl logs -f deployment/git-change-operator-controller-manager -n git-change-operator --context kind-git-change-operator"

kind-status: # Show Kind cluster and operator status (hidden)
	@echo "📊 Kind Cluster Status"
	@echo "======================"
	@export PATH="/opt/homebrew/bin:$$PATH"; \
	echo "🔗 Cluster Info:"; \
	kubectl cluster-info --context kind-git-change-operator || echo "❌ Cluster not accessible"; \
	echo; \
	echo "🏷️  Nodes:"; \
	kubectl get nodes --context kind-git-change-operator || echo "❌ Cannot get nodes"; \
	echo; \
	echo "📦 System Pods:"; \
	kubectl get pods -n kube-system --context kind-git-change-operator || echo "❌ Cannot get system pods"; \
	echo; \
	echo "🎯 Operator Pods:"; \
	kubectl get pods -n git-change-operator --context kind-git-change-operator || echo "❌ Operator not deployed"; \
	echo; \
	echo "📋 CRDs:"; \
	kubectl get crd --context kind-git-change-operator | grep "gco.galos.one" || echo "❌ CRDs not installed"; \
	echo; \
	echo "🔑 Secrets:"; \
	kubectl get secrets -n git-change-operator --context kind-git-change-operator || echo "❌ No secrets found"; \
	echo; \
	echo "🎯 GitCommits:"; \
	kubectl get gitcommit -n git-change-operator --context kind-git-change-operator || echo "📝 No GitCommit resources found"; \
	echo; \
	echo "🔍 GitCommit Status Details:"; \
	GITCOMMIT_NAME=$$(kubectl get gitcommit -n git-change-operator --context kind-git-change-operator -o name 2>/dev/null | head -1); \
	if [ -n "$$GITCOMMIT_NAME" ]; then \
		echo "Checking commit phase and SHA for: $$GITCOMMIT_NAME"; \
		echo "⏳ Monitoring commit status (polling for up to 10 seconds)..."; \
		ATTEMPTS=0; \
		MAX_ATTEMPTS=10; \
		while [ $$ATTEMPTS -lt $$MAX_ATTEMPTS ]; do \
			COMMIT_PHASE=$$(kubectl get $$GITCOMMIT_NAME -n git-change-operator --context kind-git-change-operator -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown"); \
			COMMIT_SHA=$$(kubectl get $$GITCOMMIT_NAME -n git-change-operator --context kind-git-change-operator -o jsonpath='{.status.commitSHA}' 2>/dev/null || echo "Not available"); \
			echo "  📋 Attempt $$((ATTEMPTS + 1))/$$MAX_ATTEMPTS - Phase: $$COMMIT_PHASE, SHA: $$COMMIT_SHA"; \
			if [ "$$COMMIT_PHASE" = "Committed" ] && [ -n "$$COMMIT_SHA" ] && [ "$$COMMIT_SHA" != "Not available" ]; then \
				echo "  ✅ Commit successfully completed with SHA: $$COMMIT_SHA"; \
				break; \
			elif [ "$$COMMIT_PHASE" = "Failed" ]; then \
				echo "  ❌ Commit failed"; \
				ERROR_MSG=$$(kubectl get $$GITCOMMIT_NAME -n git-change-operator --context kind-git-change-operator -o jsonpath='{.status.message}' 2>/dev/null); \
				if [ -n "$$ERROR_MSG" ]; then \
					echo "  💬 Message: $$ERROR_MSG"; \
				fi; \
				break; \
			elif [ "$$COMMIT_PHASE" = "Running" ]; then \
				echo "  🔄 Commit is running, waiting..."; \
			elif [ "$$COMMIT_PHASE" = "Pending" ]; then \
				echo "  ⏸️ Commit is pending, waiting to start..."; \
			else \
				echo "  ❓ Unknown status: $$COMMIT_PHASE, waiting..."; \
			fi; \
			ATTEMPTS=$$((ATTEMPTS + 1)); \
			if [ $$ATTEMPTS -lt $$MAX_ATTEMPTS ]; then \
				sleep 3; \
			fi; \
		done; \
		if [ $$ATTEMPTS -eq $$MAX_ATTEMPTS ] && [ "$$COMMIT_PHASE" != "Committed" ] && [ "$$COMMIT_PHASE" != "Failed" ]; then \
			echo "  ⏰ Timeout: Commit still processing after 10 seconds. Check logs for details."; \
		fi; \
	else \
		echo "No GitCommit resources found to check"; \
	fi

kind-full-demo: kind-destroy kind-create kind-deploy kind-load-image kind-setup-token kind-demo kind-status ## Complete Kind demo workflow
	@echo ""
	@echo "🎉 Complete Kind Demo Workflow Finished!"
	@echo "========================================"
	@echo ""
	@echo "✅ Kind cluster created with corporate proxy support"
	@echo "✅ git-change-operator deployed with conditional CRDs"
	@echo "✅ GitHub token secret configured"  
	@echo "✅ Demo GitCommit resource created"
	@echo ""
	@echo "🔍 Next steps:"
	@echo "   • Check operator logs: kubectl logs -n git-change-operator deployment/git-change-operator-controller-manager --context kind-git-change-operator"
	@echo "   • Monitor GitCommit status: kubectl get gitcommit -n git-change-operator -w --context kind-git-change-operator"
	@echo "   • Clean up when done: make kind-destroy"

kind-destroy: ## Delete Kind cluster and clean up
	@echo "🧹 Cleaning up Kind cluster..."
	kind delete cluster --name git-change-operator || echo "⚠️  Cluster already deleted"
	@echo "✅ Kind cluster cleaned up!"

##@ Helm
helm-lint: ## Lint the Helm chart
	helm lint helm/git-change-operator

helm-template: ## Generate Kubernetes manifests from Helm chart
	helm template git-change-operator helm/git-change-operator

helm-package: ## Package the Helm chart
	helm package helm/git-change-operator -d helm/

helm-install: ## Install the operator using Helm
	helm upgrade --install git-change-operator helm/git-change-operator --create-namespace --namespace git-change-operator-system

helm-uninstall: ## Uninstall the operator using Helm
	helm uninstall git-change-operator --namespace git-change-operator-system

helm-deploy: docker-build docker-push helm-package helm-install ## Build, push, package and deploy using Helm

##@ Documentation
docs-prepare: # Prepare README for MkDocs by temporarily removing docs/ prefix from links (hidden)
	@echo "Backing up README.md and creating MkDocs-compatible version..."
	@cp README.md README.md.bak
	@sed -i.tmp 's|](docs/|](|g' README.md && rm README.md.tmp
	@echo "README.md prepared for MkDocs (backup at README.md.bak)"

docs-restore: # Restore original README.md after docs operations (hidden)
	@if [ -f README.md.bak ]; then \
		echo "Restoring original README.md..."; \
		mv README.md.bak README.md; \
		echo "README.md restored"; \
	else \
		echo "No backup found, skipping restore"; \
	fi

docs-venv: # Create Python virtual environment for documentation (hidden)
	python3 -m venv docs/.venv
	@echo "Virtual environment created at docs/.venv"
	@echo "To activate manually: source docs/.venv/bin/activate"

docs-deps: docs-venv # Install documentation dependencies (hidden)
	docs/.venv/bin/pip install --upgrade pip
	docs/.venv/bin/pip install -r docs/mkdocs/requirements.txt

docs-serve: docs-deps docs-prepare ## Serve documentation locally for development
	docs/.venv/bin/mkdocs serve; $(MAKE) docs-restore

docs-serve-versioned: docs-deps docs-prepare ## Serve versioned documentation locally. Overwrite with make docs-serve-versioned VERSION=1.0.0
	source docs/.venv/bin/activate && mike serve --dev-addr=127.0.0.1:8001; $(MAKE) docs-restore

docs-build: docs-deps docs-prepare # Build documentation for production (hidden)
	docs/.venv/bin/mkdocs build && $(MAKE) docs-restore

docs-deploy: docs-deps # Deploy documentation to GitHub Pages (hidden)
	docs/.venv/bin/mkdocs gh-deploy --force

docs-version-deploy: docs-deps docs-prepare # Deploy a new version of documentation (usage: make docs-version-deploy VERSION=1.1.0) (hidden)
ifndef VERSION
	$(error VERSION is required. Usage: make docs-version-deploy VERSION=1.1.0)
endif
	source docs/.venv/bin/activate && mike deploy --push --update-aliases $(VERSION) latest && $(MAKE) docs-restore

docs-version-set-default: docs-deps ## Set default version for documentation (usage: make docs-version-set-default VERSION=1.0.0)
ifndef VERSION
	$(error VERSION is required. Usage: make docs-version-set-default VERSION=1.0.0)
endif
	docs/.venv/bin/mike set-default $(VERSION)

docs-version-list: docs-deps ## List all deployed documentation versions
	docs/.venv/bin/mike list

docs-clean: ## Clean built documentation and virtual environment
	rm -rf site/
	rm -rf docs/.venv/

.PHONY: help fmt vet build run clean test test-unit setup-test-env test-integration test-all docker-build docker-push install uninstall deploy undeploy kube-setup-token kind-create kind-deploy kind-setup-token kind-patch-operator kind-demo kind-load-image kind-restart-operator kind-build-and-test kind-status kind-destroy kind-destroy kind-full-demo helm-lint helm-template helm-package helm-install helm-uninstall helm-deploy docs-venv docs-deps docs-serve docs-serve-versioned docs-build docs-deploy docs-version-deploy docs-version-set-default docs-version-list docs-clean