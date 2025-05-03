SHELL := /bin/bash

IMG ?= git-change-operator:latest
KUBEBUILDER_ASSETS ?= $(shell pwd)/bin/kubebuilder/k8s/1.34.1-darwin-arm64
SETUP_ENVTEST_INDEX ?= https://artifacts.rbi.tech/artifactory/raw-githubusercontent-com-raw-proxy/kubernetes-sigs/controller-tools/HEAD/envtest-releases.yaml

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
	docker build -t ${IMG} .

docker-push: ## Push docker image
	docker push ${IMG}

install: ## Install CRDs and RBAC to the cluster
	kubectl apply -k config/

uninstall: ## Uninstall CRDs and RBAC from the cluster
	kubectl delete -k config/

deploy: docker-build docker-push install ## Build, push and deploy to cluster

undeploy: uninstall ## Undeploy from cluster

.PHONY: help fmt vet build run clean test test-unit setup-test-env test-integration test-all docker-build docker-push install uninstall deploy undeploy