include operatorhub/Makefile

MODULE   = $(shell env GO111MODULE=on $(GO) list -m)
DATE         ?= $(shell date +%FT%T%z)
KO_DATA_PATH  = $(shell pwd)/cmd/$(TARGET)/operator/kodata
TARGET        = kubernetes
COMPONENT ?= components.yaml
FORCE_FETCH_RELEASE = false
CR            = config/basic
PLATFORM := $(if $(PLATFORM),--platform $(PLATFORM))

GOLANGCI_VERSION  = $(shell yq '.jobs.linting.steps[] | select(.name == "golangci-lint") | .with.version' .github/workflows/ci.yaml)

BIN      = $(CURDIR)/.bin

GO           = go
TIMEOUT_UNIT = 5m
TIMEOUT_E2E  = 20m
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m🐱\033[0m")

export GO111MODULE=on
export GOTOOLCHAIN=auto

$(BIN):
	@mkdir -p $@
$(BIN)/%: | $(BIN) ; $(info $(M) building $(PACKAGE)…)
	$Q tmp=$$(mktemp -d); cd $$tmp; \
		env GO111MODULE=on GOPATH=$$tmp GOBIN=$(BIN) $(GO) install $(PACKAGE) \
		|| ret=$$?; \
		env GO111MODULE=on GOPATH=$$tmp GOBIN=$(BIN) $(GO) clean -modcache \
        || ret=$$?; \
		cd - ; \
	  	rm -rf $$tmp ; exit $$ret

KO = $(or ${KO_BIN},${KO_BIN},$(BIN)/ko)
$(BIN)/ko: PACKAGE=github.com/google/ko@latest

KUSTOMIZE = $(or ${KUSTOMIZE_BIN},${KUSTOMIZE_BIN},$(BIN)/kustomize)
$(BIN)/kustomize: | $(BIN) ; $(info $(M) getting kustomize)
	@curl -sSfL https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh | bash
	@mv ./kustomize $(BIN)/kustomize

GOLANGCILINT = $(or ${GOLANGCILINT_BIN},${GOLANGCILINT_BIN},$(BIN)/golangci-lint)
$(BIN)/golangci-lint: | $(BIN) ; $(info $(M) getting golangci-lint $(GOLANGCI_VERSION))
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN) $(GOLANGCI_VERSION)

##@ Clean 
.PHONY: clean-cluster
clean-cluster: | $(KO) $(KUSTOMIZE) clean-cr; $(info $(M) clean $(TARGET)…) @ ## Cleanup cluster
	@ ## --load-restrictor LoadRestrictionsNone is needed in kustomize build as files which not in child tree of kustomize base are pulled
	@ ## https://github.com/kubernetes-sigs/kustomize/issues/766
	-$(KUSTOMIZE) build --load-restrictor LoadRestrictionsNone config/$(TARGET)/overlays/default | $(KO) delete -f -
	-kubectl delete ns tekton-pipelines --ignore-not-found
	-kubectl delete \
		-f $(KO_DATA_PATH)/ \
		--ignore-not-found \
		--recursive

.PHONY: clean-manifest
clean-manifest: ## Cleanup manifest
ifeq ($(TARGET), openshift)
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-pipeline
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-trigger
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-chains
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-hub
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-results
	rm -rf ./cmd/$(TARGET)/operator/kodata/manual-approval-gate
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-pruner
	rm -rf ./cmd/$(TARGET)/operator/kodata/pruner
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-addon/pipelines-as-code
	find ./cmd/$(TARGET)/operator/kodata/tekton-addon/addons/06-ecosystem/tasks -type f ! -name "role.yaml" ! -name "rolebinding.yaml" -delete 
	find ./cmd/$(TARGET)/operator/kodata/tekton-addon/addons/06-ecosystem/stepactions -type f ! -name "role.yaml" ! -name "rolebinding.yaml" -delete
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-addon/pipelines-as-code-templates/go.yaml
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-addon/pipelines-as-code-templates/java.yaml
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-addon/pipelines-as-code-templates/nodejs.yaml
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-addon/pipelines-as-code-templates/python.yaml
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-addon/pipelines-as-code-templates/generic.yaml
else
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton*
	rm -rf ./cmd/$(TARGET)/operator/kodata/pruner
	rm -rf ./cmd/$(TARGET)/operator/kodata/manual-approval-gate
endif

.PHONY: clean-bin # Clean binary
clean-bin: 
	-rm -rf $(BIN)
	-rm -rf bin
	-rm -rf test/tests.* test/coverage.*

.PHONY: clean
clean: clean-cluster clean-bin clean-manifest; $(info $(M) clean all) @ ## Cleanup everything


.PHONY: clean-cr
clean-cr: | ; $(info $(M) clean CRs on $(TARGET)) @ ## Clean the CRs to the current cluster
	-$Q kubectl delete -f config/crs/$(TARGET)/$(CR)

##@ General
.PHONY: help
help: ## print this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

FORCE:

bin/%: cmd/% FORCE
	$Q $(GO) build -mod=vendor $(LDFLAGS) -v -o $@ ./$<

.PHONY: resolve
resolve: | $(KO) $(KUSTOMIZE) get-releases ; $(info $(M) ko resolve on $(TARGET)) @ ## Resolve config to the current cluster
	@ ## --load-restrictor LoadRestrictionsNone is needed in kustomize build as files which not in child tree of kustomize base are pulled
	@ ## https://github.com/kubernetes-sigs/kustomize/issues/766
	$Q $(KUSTOMIZE) build --load-restrictor LoadRestrictionsNone config/$(TARGET)/overlays/default | $(KO) resolve --push=false --platform=all --oci-layout-path=$(BIN)/oci -f -

.PHONY: generated
generated: | vendor ; $(info $(M) update generated files) ## Update generated files
	$Q ./hack/update-codegen.sh

.PHONY: vendor
vendor: ; $(info $(M) update vendor folder)  ## Update vendor folder
	$Q ./hack/update-deps.sh

##@ Bump Components Version
.PHONY: components/bump  
components/bump: $(OPERATORTOOL) ## Bump the version of a component 
	@go run ./cmd/tool bump ${COMPONENT}

.PHONY: components/bump-bugfix
components/bump-bugfix: $(OPERATORTOOL)  ## Bump bump-bugfix
	@go run ./cmd/tool bump --bugfix ${COMPONENT}

.PHONY: get-releases
get-releases: | ## Get releases
	$Q ./hack/fetch-releases.sh $(TARGET) ${COMPONENT} $(FORCE_FETCH_RELEASE) || exit ;

##@ Apply
.PHONY: apply
apply: | $(KO) $(KUSTOMIZE) get-releases ; $(info $(M) ko apply on $(TARGET)) @ ## Apply config to the current cluster
	@ ## --load-restrictor LoadRestrictionsNone is needed in kustomize build as files which not in child tree of kustomize base are pulled
	@ ## https://github.com/kubernetes-sigs/kustomize/issues/766
	$Q $(KUSTOMIZE) build --load-restrictor LoadRestrictionsNone config/$(TARGET)/overlays/default | $(KO) apply $(KO_FLAGS) $(PLATFORM) -f -

.PHONY: apply-cr
apply-cr: | ; $(info $(M) apply CRs on $(TARGET)) @ ## Apply the CRs to the current cluster
	$Q kubectl apply -f config/crs/$(TARGET)/$(CR)

##@ Bundle

.PHONY: operator-bundle
operator-bundle: bundle-generate ## Generate the operator bundle manifests
	@echo "Operator bundle created successfully."

.PHONY: operator-bundle-build
operator-bundle-build: bundle-build ## Build the operator bundle image
	@echo "Building the bundle image: $(BUNDLE_IMG)"

.PHONY: operator-bundle-push
operator-bundle-push: bundle-push  ## Push the operator bundle to the registry
	@echo "Operator bundle pushed successfully."

.PHONY: operator-catalog-build
operator-catalog-build: catalog-build ## Build a file-based OLM catalog image containing a released operator bundle.
	@echo "Operator catalog built successfully."

.PHONY: operator-catalog-push
operator-catalog-push: catalog-push ## Build and push an OLM catalog image with a released operator bundle.
	@echo "Operator catalog pushed successfully."

.PHONY: operator-catalog-run
operator-catalog-run: catalog-run ## Run the operator from a catalog image, using an OLM subscription
	@echo "Operator catalog run successfully."

##@ Tests
GO           = go
TEST_UNIT_TARGETS := test-unit-verbose test-unit-race test-unit-failfast test-unit-verbose-and-race
test-unit-verbose: ARGS=-v
test-unit-failfast: ARGS=-failfast
test-unit-race:    ARGS=-race
test-unit-verbose-and-race: ARGS=-v -race
$(TEST_UNIT_TARGETS): test-unit
test-clean:  ## Clean testcache
	@echo "Cleaning test cache"
	@go clean -testcache
.PHONY: $(TEST_UNIT_TARGETS) test test-unit
test: test-clean test-unit ## Run test-unit
test-unit: ## Run unit tests
	@echo "Running unit tests..."
	$Q $(GO) test -timeout $(TIMEOUT_UNIT) $(ARGS) ./...


.PHONY: lint
lint: lint-go lint-yaml ## run all linters

.PHONY: lint-go
lint-go: | $(GOLANGCILINT) ## runs go linter on all go files
	@echo "Linting go files..."
	@$(GOLANGCILINT) run ./... --modules-download-mode=vendor \
							--max-issues-per-linter=0 \
							--max-same-issues=0 \
							--timeout 5m

YAML_FILES := $(shell find . -type f -regex ".*y[a]ml" -print)
.PHONY: lint-yaml
lint-yaml: ${YAML_FILES} ## runs yamllint on all yaml files
	@echo "Linting yaml files..."
	@yamllint -c .yamllint $(YAML_FILES)


# Prerequisite: docker [or] podman and kind
# this will deploy a local registry using docker and create a kind cluster
# configuring with the registry
# then does make apply to deploy the operator
# and show the location of kubeconfig at last
.PHONY: dev-setup
dev-setup: # setup kind with local registry for local development
	@cd ./hack/dev/kind/;./install.sh
