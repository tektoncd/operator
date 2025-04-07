BUNDLE_ARGS := $(or ${BUNDLE_ARGS},--help)

OPERATOR_SDK = $(or ${OPERATOR_SDK_BIN},$(BIN)/operator-sdk)

$(BIN):
	@mkdir -p $@

CONTAINER_ENGINE ?= docker
IMAGE_HOST ?= quay.io
IMAGE_NAMESPACE ?= jkhelil/tektoncd/operator
IMAGE_REPO ?= $(IMAGE_HOST)/$(IMAGE_NAMESPACE)
IMAGE_TAG_BASE ?= $(IMAGE_REPO)/operator
VERSION ?= 0.75.0

# Define the bundle image with versioning
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)
BUNDLE_DIR ?= operatorhub/kubernetes/release-artifacts
BUNDLE_DOCKERFILE ?= operatorhub/kubernetes/release-artifacts/bundle.Dockerfile

CATALOG_DIR ?= operatorhub/kubernetes/catalog
CATALOG_SOURCE_DIR ?= operatorhub/kubernetes/catalog_source
SUBSCRIPTION_DIR ?= operatorhub/kubernetes/subscription
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:$(VERSION)

KUBECTL_BIN ?= kubectl
SED_BIN ?= sed

# operating-system type and architecture based on golang
OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.21.0/$(OS)-$(ARCH)-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# Use HTTP for opm registry operations
OPM_USE_HTTP ?= false


OPERATOR_SDK = $(shell pwd)/.bin/operator-sdk
.PHONY: operator-sdk
operator-sdk:
	OS=${OS} ARCH=${ARCH} ./operatorhub/tools/install_operator-sdk.sh $(OPERATOR_SDK)

.PHONY: install-olm
install-olm: operator-sdk
	$(OPERATOR_SDK) olm install

.PHONY: bundle-generate
bundle-generate: | $(OPERATOR_SDK) openshift/release-artifacts/bundle/manifests
	@$(info BUNDLE_ARGS: $(BUNDLE_ARGS))
	@OPERATOR_SDK=$(OPERATOR_SDK) ./operatorhub/tools/bundle.py ${BUNDLE_ARGS}

openshift/release-artifacts/bundle/manifests:
	mkdir -p openshift/release-artifacts/bundle/manifests

.PHONY: bundle-build
bundle-build:
	@echo "Building the bundle image: $(BUNDLE_IMG)"
	$(CONTAINER_ENGINE) build -f $(BUNDLE_DOCKERFILE) -t $(BUNDLE_IMG) $(BUNDLE_DIR)


.PHONY: bundle-push
bundle-push: IMAGE_PUSH=true
bundle-push: bundle-build
	@echo "Pushing the bundle image: $(BUNDLE_IMG)"
	$(CONTAINER_ENGINE) push $(BUNDLE_IMG)

.PHONY: catalog-build
catalog-build: opm
	BUNDLE_IMG=$(BUNDLE_IMG) OPM_BIN=$(OPM) SED_BIN=$(SED_BIN) CSV_VERSION=$(VERSION) USE_HTTP=$(OPM_USE_HTTP) CATALOG_DIR=$(CATALOG_DIR) ./operatorhub/tools/render-candidate-catalog.sh
	$(CONTAINER_ENGINE) build -f $(CATALOG_DIR)/released.Dockerfile -t $(CATALOG_IMG) $(CATALOG_DIR) 

.PHONY: catalog-push
catalog-push: catalog-build
	$(CONTAINER_ENGINE) push $(CATALOG_IMG)

CATALOG_NAMESPACE ?= tektoncd-operator

.PHONY: catalog-run
catalog-run:
	CATALOG_IMG=$(CATALOG_IMG) CSV_VERSION=$(VERSION) KUBECTL_BIN=$(KUBECTL_BIN) NAMESPACE=$(CATALOG_NAMESPACE) SED_BIN=$(SED_BIN) CATALOG_SOURCE_DIR=$(CATALOG_SOURCE_DIR) SUBSCRIPTION_DIR=$(SUBSCRIPTION_DIR) ./operatorhub/tools/run-operator-catalog.sh


