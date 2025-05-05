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

.PHONY: help fmt vet build run clean test test-unit setup-test-env test-integration test-all docker-build docker-push install uninstall deploy undeploy helm-lint helm-template helm-package helm-install helm-uninstall helm-deploy