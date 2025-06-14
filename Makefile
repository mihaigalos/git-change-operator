SHELL := /bin/bash

IMG ?= git-change-operator:latest
KUBEBUILDER_ASSETS ?= $(shell pwd)/bin/kubebuilder/k8s/1.34.1-darwin-arm64
SETUP_ENVTEST_INDEX ?= https://artifacts.rbi.tech/artifactory/raw-githubusercontent-com-raw-proxy/kubernetes-sigs/controller-tools/HEAD/envtest-releases.yaml

# Go proxy configuration (can be overridden via environment variables)
GOPROXY_ARG ?= $(GOPROXY)
GOSUMDB_ARG ?= $(GOSUMDB)
GONOPROXY_ARG ?= $(GONOPROXY)
GONOSUMDB_ARG ?= $(GONOSUMDB)

all: build

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development
fmt: ## Format Go code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

manifests: ## Generate CRDs and RBAC manifests
	controller-gen crd:crdVersions=v1 rbac:roleName=manager-role paths=./... output:crd:artifacts:config=config/crd output:rbac:artifacts:config=config/rbac

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
docker-build: ## Build docker image
	@if [ -f "/Users/mihai.galos/certs/zscaler.pem" ]; then \
		echo "Copying corporate certificate for build..."; \
		cp /Users/mihai.galos/certs/zscaler.pem ./zscaler.pem; \
	fi
	@echo "Building Docker image with proxy configuration..."
	@docker build --network=host \
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
		-t ${IMG} .
	@if [ -f "./zscaler.pem" ]; then \
		echo "Cleaning up certificate..."; \
		rm ./zscaler.pem; \
	fi

docker-push: ## Push docker image
	docker push ${IMG}

install: ## Install CRDs and RBAC to the cluster
	kubectl apply -k config/

uninstall: ## Uninstall CRDs and RBAC from the cluster
	kubectl delete -k config/

deploy: docker-build docker-push install ## Build, push and deploy to cluster

undeploy: uninstall ## Undeploy from cluster

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
docs-venv: ## Create Python virtual environment for documentation
	python3 -m venv docs/.venv
	@echo "Virtual environment created at docs/.venv"
	@echo "To activate manually: source docs/.venv/bin/activate"

docs-deps: docs-venv ## Install documentation dependencies
	docs/.venv/bin/pip install --upgrade pip
	docs/.venv/bin/pip install -r docs/mkdocs/requirements.txt

docs-serve: docs-deps ## Serve documentation locally for development
	docs/.venv/bin/mkdocs serve

docs-serve-versioned: docs-deps ## Serve versioned documentation locally
	docs/.venv/bin/mike serve --dev-addr=127.0.0.1:8001

docs-build: docs-deps ## Build documentation for production
	docs/.venv/bin/mkdocs build

docs-deploy: docs-deps ## Deploy documentation to GitHub Pages
	docs/.venv/bin/mkdocs gh-deploy --force

docs-version-deploy: docs-deps ## Deploy a new version of documentation (usage: make docs-version-deploy VERSION=1.1.0)
ifndef VERSION
	$(error VERSION is required. Usage: make docs-version-deploy VERSION=1.1.0)
endif
	docs/.venv/bin/mike deploy --push --update-aliases $(VERSION) latest

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

.PHONY: help fmt vet build run clean test test-unit setup-test-env test-integration test-all docker-build docker-push install uninstall deploy undeploy helm-lint helm-template helm-package helm-install helm-uninstall helm-deploy docs-venv docs-deps docs-serve docs-serve-versioned docs-build docs-deploy docs-version-deploy docs-version-set-default docs-version-list docs-clean