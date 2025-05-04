
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
VERSION ?= 0.0.0-dev
ENVTEST_K8S_VERSION = 1.26.1

REGISTRY ?= quay.io/zncdatadev
PROJECT_NAME = hive-operator

# Build variables
BUILD_TIMESTAMP ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
BUILD_COMMIT ?= $(shell git rev-parse HEAD)

# Image URL to use all building/pushing image targets
IMG ?= $(REGISTRY)/$(PROJECT_NAME):$(VERSION)

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
# More info on the usage of ANSI control characters for terminal formatting: `
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# TODO(user): To use a different vendor for e2e tests, modify the setup under 'tests/e2e'.
# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# Prometheus and CertManager are installed by default; skip with:
# - PROMETHEUS_INSTALL_SKIP=true
# - CERT_MANAGER_INSTALL_SKIP=true
.PHONY: test-e2e
test-e2e: manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	@command -v kind >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@kind get clusters | grep -q 'kind' || { \
		echo "No Kind cluster is running. Please start a Kind cluster before running the e2e tests."; \
		exit 1; \
	}
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

LDFLAGS = "-X github.com/zncdatadev/$(PROJECT_NAME)/internal/util/version.BuildVersion=$(VERSION) \
	-X github.com/zncdatadev/$(PROJECT_NAME)/internal/util/version.GitCommit=$(BUILD_COMMIT) \
	-X github.com/zncdatadev/$(PROJECT_NAME)/internal/util/version.BuildTime=$(BUILD_TIMESTAMP)"

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -ldflags $(LDFLAGS) -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build --build-arg LDFLAGS=$(LDFLAGS) -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64
BUILDX_METADATA_FILE ?= docker-digests.json	# The file to store the digests of the images built by buildx
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	- $(CONTAINER_TOOL) buildx create --name $(PROJECT_NAME)-builder
	$(CONTAINER_TOOL) buildx use $(PROJECT_NAME)-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --build-arg LDFLAGS=$(LDFLAGS) --tag ${IMG} --metadata-file ${BUILDX_METADATA_FILE} -f Dockerfile .
	- $(CONTAINER_TOOL) buildx rm $(PROJECT_NAME)-builder

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	# add --server-side=true to fix "is invalid: metadata.annotations: Too long: must have at most 262144 bytes"
	# ref: https://stackoverflow.com/a/70083579
	$(KUSTOMIZE) build config/crd | kubectl apply --server-side=true -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	# add --server-side=true to fix "is invalid: metadata.annotations: Too long: must have at most 262144 bytes"
	# ref: https://stackoverflow.com/a/70083579
	$(KUSTOMIZE) build config/default | kubectl apply --server-side=true -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
HELM = $(LOCALBIN)/helm
KIND = $(LOCALBIN)/kind

## Tool Versions
KUSTOMIZE_VERSION ?= v5.6.0
CONTROLLER_TOOLS_VERSION ?= v0.17.1
ENVTEST_VERSION ?= release-0.20
GOLANGCI_LINT_VERSION ?= v2.0.2
HELM_VERSION ?= v3.17.0
KIND_VERSION ?= v0.26.0

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary.
$(HELM): $(LOCALBIN)
	$(call go-install-tool,$(HELM),helm.sh/helm/v3/cmd/helm,$(HELM_VERSION))

.PHONY: kind
kind: $(KIND) ## Download helm locally if necessary.
$(KIND): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

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

HELM_DEPENDS ?= commons-operator listener-operator secret-operator
TEST_NAMESPACE = kubedoop-operators

.PHONY: helm-install-depends
helm-install-depends: helm ## Install the helm chart depends.
	$(HELM) repo add kubedoop https://zncdatadev.github.io/kubedoop-helm-charts/
ifneq ($(strip $(HELM_DEPENDS)),)
	for dep in $(HELM_DEPENDS); do \
		$(HELM) upgrade --install --create-namespace --namespace $(TEST_NAMESPACE) --wait $$dep kubedoop/$$dep --version $(VERSION); \
	done
endif

## helm uninstall depends
.PHONY: helm-uninstall-depends
helm-uninstall-depends: helm ## Uninstall the helm chart depends.
ifneq ($(strip $(HELM_DEPENDS)),)
	for dep in $(HELM_DEPENDS); do \
		$(HELM) uninstall --namespace $(TEST_NAMESPACE) $$dep; \
	done
endif

##@ Chainsaw-E2E

# Tool Versions
# KIND_K8S_VERSION refers to the version of k8s to be used by kind.
# The version only effects e2e tests.
# When run `make kind-create`, the version of k8s will be used to create the kind cluster,
# and the target kubeconfig file will be named as `./kind-kubeconfig-$(KIND_K8S_VERSION)`.
# So if you want to use the target cluster, to run `export KUBECONFIG=./kind-kubeconfig-$(KIND_K8S_VERSION)`.
KIND_K8S_VERSION ?= 1.26.15
CHAINSAW_VERSION ?= v0.2.12
PRODUCT_VERSION ?= 3.1.3

KIND_IMAGE ?= kindest/node:v${KIND_K8S_VERSION}
KIND_KUBECONFIG ?= ./kind-kubeconfig-$(KIND_K8S_VERSION)
KIND_CLUSTER_NAME ?= ${PROJECT_NAME}-$(KIND_K8S_VERSION)
KIND_CONFIG ?= test/e2e/kind-config.yaml

CHAINSAW = $(LOCALBIN)/chainsaw

# Create a kind cluster
.PHONY: kind-create
kind-create: kind ## Create a kind cluster.
	$(KIND) create cluster --config $(KIND_CONFIG) --image $(KIND_IMAGE) --name $(KIND_CLUSTER_NAME) --kubeconfig $(KIND_KUBECONFIG) --wait 120s

.PHONY: kind-delete
kind-delete: kind ## Delete a kind cluster.
	$(KIND) delete cluster --name $(KIND_CLUSTER_NAME)

# chainsaw

# Use `grep 0.2.6 > /dev/null` instead of `grep -q 0.2.6`. It will not be able to determine the version number,
# although the execution in the shell is normal, but in the makefile does fail to understand the mechanism in the makefile
# The operation ends by using `touch` to change the time of the file so that its timestamp is further back than the directory,
# so that no subsequent logic is performed after the `chainsaw` check is successful in relying on the `$(CHAINSAW)` target.
.PHONY: chainsaw
chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(LOCALBIN)
	@{ \
	set -xe ;\
	if test -x $(LOCALBIN)/chainsaw && ! $(LOCALBIN)/chainsaw version | grep $(CHAINSAW_VERSION:v%=%) > /dev/null; then \
		echo "$(LOCALBIN)/chainsaw version is not expected $(CHAINSAW_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/chainsaw; \
	fi; \
	if test ! -s $(LOCALBIN)/chainsaw; then \
		mkdir -p $(dir $(CHAINSAW)) ;\
		TMP=$(shell mktemp -d) ;\
		OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
		curl -sSL https://github.com/kyverno/chainsaw/releases/download/$(CHAINSAW_VERSION)/chainsaw_$${OS}_$${ARCH}.tar.gz | tar -xz -C $$TMP ;\
		mv $$TMP/chainsaw $(CHAINSAW) ;\
		rm -rf $$TMP ;\
		chmod +x $(CHAINSAW) ;\
		touch $(CHAINSAW) ;\
	fi; \
	}

.PHONY: chainsaw-setup
chainsaw-setup: ## Run the chainsaw setup
	make docker-build
	$(KIND) --name $(KIND_CLUSTER_NAME) load docker-image $(IMG)
	KUBECONFIG=$(KIND_KUBECONFIG) make helm-install-depends
	KUBECONFIG=$(KIND_KUBECONFIG) make deploy

.PHONY: chainsaw-test
chainsaw-test: chainsaw ## Run the chainsaw test
	echo "product_version: $(PRODUCT_VERSION)" | KUBECONFIG=$(KIND_KUBECONFIG) $(CHAINSAW) test --cluster cluster-1=$(KIND_KUBECONFIG) --test-dir ./test/e2e/ --values -

.PHONY: chainsaw-cleanup
chainsaw-cleanup: ## Run the chainsaw cleanup
	KUBECONFIG=$(KIND_KUBECONFIG) make helm-uninstall-depends
	KUBECONFIG=$(KIND_KUBECONFIG) make undeploy
