# Image URL to use all building/pushing image targets
TAG ?= latest
HELM_IMG ?= samirtahir91076/kube-jit-api
IMG ?= samirtahir91076/kube-jit-api:${TAG}

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development
.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	$(GOLANGCI_LINT) config verify

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary.
	go build -o bin/api cmd/main.go
	swag init -g cmd/main.go

.PHONY: run
run: fmt vet ## Run from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build --build-arg BUILD_SHA=$$(git rev-parse --short HEAD) -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name kube-jit-api-builder
	$(CONTAINER_TOOL) buildx use kube-jit-api-builder
	- $(CONTAINER_TOOL) buildx build --build-arg BUILD_SHA=$$(git rev-parse --short HEAD) --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm kube-jit-api-builder
	rm Dockerfile.cross

##@Test
test: ## Run unit tests
	UNIT_TEST="true" SMTP_PORT=1025 SMTP_HOST=localhost SMTP_FROM=api@example.com OAUTH_REDIRECT_URI=localhost ALLOWED_DOMAIN=foo.com OAUTH_CLIENT_SECRET=foo OAUTH_CLIENT_ID=foo OAUTH_PROVIDER=azure API_NAMESPACE=default HMAC_SECRET=a-valid-32-byte-hmac-secret-key go test -v ./... -coverprofile cover.out
	go tool cover -html=cover.out -o cover.html
##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

run-docker:
		docker run -p 8589:8589 $(IMG)

template:
		helm template -n kube-jit-api kube-jit-api chart/kube-jit-api --create-namespace -f ./sample/helm-values.yaml

deploy:
		helm install -n kube-jit-api kube-jit-api chart/kube-jit-api --create-namespace -f ./sample/helm-values.yaml

template-gke:
		helm template -n kube-jit-api kube-jit-api chart/kube-jit-api --create-namespace -f ./sample/google/gke-values.yaml --set image.repository=$(HELM_IMG) --set image.tag=$(TAG)

deploy-gke:
		helm install -n kube-jit-api kube-jit-api chart/kube-jit-api --create-namespace -f ./sample/google/gke-values.yaml --set image.repository=$(HELM_IMG) --set image.tag=$(TAG)

template-aks:
		helm template -n kube-jit-api kube-jit-api chart/kube-jit-api --create-namespace -f ./sample/azure/aks-values.yaml --set image.repository=$(HELM_IMG) --set image.tag=$(TAG)

deploy-aks:
		helm install -n kube-jit-api kube-jit-api chart/kube-jit-api --create-namespace -f ./sample/azure/aks-values.yaml --set image.repository=$(HELM_IMG) --set image.tag=$(TAG)

deploy-gh:
		helm install -n kube-jit-api kube-jit-api chart/kube-jit-api --create-namespace -f ./sample/github/github-values.yaml --set image.repository=$(HELM_IMG) --set image.tag=$(TAG)

delete:
		helm delete -n kube-jit-api kube-jit-api || true

get:
		kubectl -n kube-jit-api get all
	
kind-load:
		kind load docker-image $(IMG) --name kubejit

e2e: delete build deploy

e2e-gke: docker-build docker-push delete deploy-gke

e2e-aks: docker-build docker-push delete deploy-aks

e2e-gh: docker-build docker-push delete deploy-gh

.PHONY: openapi2-3
openapi2-3: ## Generate OpenAPI specification from Swagger
	docker run --rm -v ${PWD}:/usr/src/app mermade/swagger2openapi swagger2openapi --yaml --outfile openapi3.yaml docs/swagger.json

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.6.0
GOLANGCI_LINT_VERSION ?= v1.63.4

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef
