# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.0.0-dev

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
CHANNELS ?= stable
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
DEFAULT_CHANNEL ?= stable
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

REGISTRY ?= quay.io/zncdatadev
PROJECT_NAME = hive-operator

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# quay.io/zncdatadev/hive-operator-bundle:$VERSION and quay.io/zncdatadev/hive-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= $(REGISTRY)/$(PROJECT_NAME)

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.37.0

# Image URL to use all building/pushing image targets
IMG ?= $(REGISTRY)/$(PROJECT_NAME):$(VERSION)
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
# ref: https://github.com/kubernetes-sigs/kubebuilder/releases in v3.11.0-v3.14.1 ENVTEST_K8S_VERSION support 1.26.1 and 1.27.1
ENVTEST_K8S_VERSION ?= 1.26.1

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
# target descriptions by '##'. The awk commands is responsible for reading the
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

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.61.0
golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION) ;\
	}

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run --timeout 5m

.PHONY: test
test: manifests generate fmt vet envtest lint ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64
.PHONY: docker-buildx
docker-buildx: test ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	$(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	$(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	$(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

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
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	# add --server-side=true to fix "is invalid: metadata.annotations: Too long: must have at most 262144 bytes"
	# ref: https://stackoverflow.com/a/70083579
	$(KUSTOMIZE) build config/default | kubectl apply --server-side=true -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v5.5.0
CONTROLLER_TOOLS_VERSION ?= v0.16.5

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle --select-optional suite=operatorframework --optional-values=k8s-version=1.26

.PHONY: scorecard-test
scorecard-test: bundle ## Run the scorecard tests
	$(OPERATOR_SDK) scorecard bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	$(CONTAINER_TOOL) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: bundle-buildx
bundle-buildx: ## Build the bundle image.
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' bundle.Dockerfile > bundle.Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	$(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${BUNDLE_IMG} -f bundle.Dockerfile.cross .
	$(CONTAINER_TOOL) buildx rm project-v3-builder
	rm bundle.Dockerfile.cross

.PHONY: bundle-run
bundle-run: ## Run the bundle image.
	$(OPERATOR_SDK) run bundle $(BUNDLE_IMG)

.PHONY: bundle-cleanup
bundle-cleanup: ## Clean up the bundle image.
	$(OPERATOR_SDK) cleanup $(PROJECT_NAME)

OPM_VERSION ?= v1.43.0

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:latest

.PHONY: catalog
catalog: opm ## Build a catalog manifests.
	mkdir -p catalog
	@if ! test -f ./catalog.Dockerfile; then \
		$(OPM) generate dockerfile catalog; \
	fi
	sed -E "s|(image: ).*-bundle:v$(VERSION)|\1$(BUNDLE_IMG)|g" catalog-template.yaml | \
	$(OPM) alpha render-template basic -o yaml > catalog/catalog.yaml

.PHONY: catalog-build
catalog-docker-build: ## Build a catalog image.
	$(CONTAINER_TOOL) build -t ${CATALOG_IMG} -f catalog.Dockerfile .

# Push the catalog image.
.PHONY: catalog-push
catalog-docker-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

.PHONY: catalog-buildx
catalog-docker-buildx: ## Build and push a catalog image for cross-platform support
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) -f catalog.Dockerfile --tag ${CATALOG_IMG} .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder

##@ E2E

##@ helm

HELM_VERSION ?= v3.16.2
HELM = $(LOCALBIN)/helm

.PHONY: helm
helm: ## Download helm locally if necessary.
ifeq (,$(shell which $(HELM)))
ifeq (,$(shell which helm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(HELM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSL https://get.helm.sh/helm-$(HELM_VERSION)-$${OS}-$${ARCH}.tar.gz | tar -xz -C $(LOCALBIN) --strip-components=1 $${OS}-$${ARCH}/helm ;\
	chmod +x $(HELM) ;\
	}
else
HELM = $(shell which helm)
endif
endif

HELM_DEPENDS ?= commons-operator listener-operator secret-operator
TEST_NAMESPACE = kubedoop-operators

.PHONY: helm-install-depends
helm-install-depends: ## Install the helm chart depends.
	$(HELM) repo add kubedoop https://zncdatadev.github.io/kubedoop-helm-charts/
ifneq ($(strip $(HELM_DEPENDS)),)
	for dep in $(HELM_DEPENDS); do \
		$(HELM) upgrade --install --create-namespace --namespace $(TEST_NAMESPACE) --wait $$dep kubedoop/$$dep --version $(VERSION); \
	done
endif

.PHONY: helm-install
helm-install: helm-install-depends ## Install the helm chart.
	$(HELM) upgrade --install --create-namespace --namespace $(TEST_NAMESPACE) --wait $(PROJECT_NAME) kubedoop/$(PROJECT_NAME) --version $(VERSION)

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall the helm chart.
	$(HELM) uninstall --namespace $(TEST_NAMESPACE) $(PROJECT_NAME)

# kind
KIND_VERSION ?= v0.24.0

KINDTEST_K8S_VERSION ?= 1.26.15

KIND_IMAGE ?= kindest/node:v${KINDTEST_K8S_VERSION}

KIND_KUBECONFIG ?= ./kind-kubeconfig-$(KINDTEST_K8S_VERSION)
KIND_CLUSTER_NAME ?= ${PROJECT_NAME}-$(KINDTEST_K8S_VERSION)
KIND_CONFIG ?= test/e2e/kind-config.yaml

.PHONY: kind
KIND = $(LOCALBIN)/kind
kind: ## Download kind locally if necessary.
ifeq (,$(shell which $(KIND)))
ifeq (,$(shell which kind 2>/dev/null))
	@{ \
	set -e ;\
	go install sigs.k8s.io/kind@$(KIND_VERSION) ;\
	}
KIND = $(GOBIN)/bin/kind
else
KIND = $(shell which kind)
endif
endif

OLM_VERSION ?= v0.28.0

# Create a kind cluster
.PHONY: kind-create
kind-create: kind ## Create a kind cluster.
	$(KIND) create cluster --config $(KIND_CONFIG) --image $(KIND_IMAGE) --name $(KIND_CLUSTER_NAME) --kubeconfig $(KIND_KUBECONFIG) --wait 120s

.PHONY: kind-delete
kind-delete: kind ## Delete a kind cluster.
	$(KIND) delete cluster --name $(KIND_CLUSTER_NAME)

# chainsaw

CHAINSAW_VERSION ?= v0.2.11
CHAINSAW = $(LOCALBIN)/chainsaw

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
	KUBECONFIG=$(KIND_KUBECONFIG) make helm-install

.PHONY: chainsaw-test
chainsaw-test: chainsaw ## Run the chainsaw test
	KUBECONFIG=$(KIND_KUBECONFIG) $(CHAINSAW) test --cluster cluster-1=$(KIND_KUBECONFIG) --test-dir ./test/e2e/

.PHONY: chainsaw-cleanup
chainsaw-cleanup: ## Run the chainsaw cleanup
	KUBECONFIG=$(KIND_KUBECONFIG) make helm-uninstall
