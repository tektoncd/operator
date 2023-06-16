# include operatorhub/Makefile

MODULE   = $(shell env GO111MODULE=on $(GO) list -m)
DATE         ?= $(shell date +%FT%T%z)
KO_DATA_PATH  = $(shell pwd)/cmd/$(TARGET)/operator/kodata
TARGET        = kubernetes
FORCE_FETCH_RELEASE = false
CR            = config/basic
PLATFORM := $(if $(PLATFORM),--platform $(PLATFORM))

GOLANGCI_VERSION  = v1.47.2

BIN      = $(CURDIR)/.bin

GO           = go
TIMEOUT_UNIT = 5m
TIMEOUT_E2E  = 20m
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m🐱\033[0m")

export GO111MODULE=on

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
clean-manifest:
ifeq ($(TARGET), openshift)
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-pipeline
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-trigger
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-chains
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-hub
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-results
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-addon/pipelines-as-code
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton-addon/addons/02-clustertasks/source_external/
	rm -rf ./cmd/openshift/operator/kodata/tekton-addon/pipelines-as-code-templates/go.yaml
	rm -rf ./cmd/openshift/operator/kodata/tekton-addon/pipelines-as-code-templates/java.yaml
	rm -rf ./cmd/openshift/operator/kodata/tekton-addon/pipelines-as-code-templates/nodejs.yaml
	rm -rf ./cmd/openshift/operator/kodata/tekton-addon/pipelines-as-code-templates/python.yaml
	rm -rf ./cmd/openshift/operator/kodata/tekton-addon/pipelines-as-code-templates/generic.yaml
else
	rm -rf ./cmd/$(TARGET)/operator/kodata/tekton*
endif

.PHONY: clean-bin
clean-bin:
	-rm -rf $(BIN)
	-rm -rf bin
	-rm -rf test/tests.* test/coverage.*

.PHONY: clean
clean: clean-cluster clean-bin clean-manifest; $(info $(M) clean all) @ ## Cleanup everything

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

FORCE:

bin/%: cmd/% FORCE
	$Q $(GO) build -mod=vendor $(LDFLAGS) -v -o $@ ./$<

.PHONY: compoments/bump
components/bump: $(OPERATORTOOL)
	@go run ./cmd/tool bump components.yaml

.PHONY: components/bump-bugfix
components/bump-bugfix: $(OPERATORTOOL)
	@go run ./cmd/tool --bugfix components.yaml

.PHONY: get-releases
get-releases: |
	$Q ./hack/fetch-releases.sh $(TARGET) components.yaml $(FORCE_FETCH_RELEASE) || exit ;

.PHONY: apply
apply: | $(KO) $(KUSTOMIZE) get-releases ; $(info $(M) ko apply on $(TARGET)) @ ## Apply config to the current cluster
	@ ## --load-restrictor LoadRestrictionsNone is needed in kustomize build as files which not in child tree of kustomize base are pulled
	@ ## https://github.com/kubernetes-sigs/kustomize/issues/766
	$Q $(KUSTOMIZE) build --load-restrictor LoadRestrictionsNone config/$(TARGET)/overlays/default | $(KO) apply $(KO_FLAGS) $(PLATFORM) -f -

.PHONY: apply-cr
apply-cr: | ; $(info $(M) apply CRs on $(TARGET)) @ ## Apply the CRs to the current cluster
	$Q kubectl apply -f config/crs/$(TARGET)/$(CR)

.PHONY: operator-bundle
operator-bundle:
	make -C operatorhub operator-bundle

.PHONY: clean-cr
clean-cr: | ; $(info $(M) clean CRs on $(TARGET)) @ ## Clean the CRs to the current cluster
	-$Q kubectl delete -f config/crs/$(TARGET)/$(CR)

.PHONY: resolve
resolve: | $(KO) $(KUSTOMIZE) get-releases ; $(info $(M) ko resolve on $(TARGET)) @ ## Resolve config to the current cluster
	@ ## --load-restrictor LoadRestrictionsNone is needed in kustomize build as files which not in child tree of kustomize base are pulled
	@ ## https://github.com/kubernetes-sigs/kustomize/issues/766
	$Q $(KUSTOMIZE) build --load-restrictor LoadRestrictionsNone config/$(TARGET)/overlays/default | $(KO) resolve --push=false --oci-layout-path=$(BIN)/oci -f -

.PHONY: generated
generated: | vendor ; $(info $(M) update generated files) ## Update generated files
	$Q ./hack/update-codegen.sh

.PHONY: vendor
vendor: ; $(info $(M) update vendor folder)  ## Update vendor folder
	$Q ./hack/update-deps.sh


## Tests
GO           = go
TEST_UNIT_TARGETS := test-unit-verbose test-unit-race test-unit-failfast
test-unit-verbose: ARGS=-v
test-unit-failfast: ARGS=-failfast
test-unit-race:    ARGS=-race
$(TEST_UNIT_TARGETS): test-unit
test-clean:  ## Clean testcache
	@echo "Cleaning test cache"
	@go clean -testcache ./...
.PHONY: $(TEST_UNIT_TARGETS) test test-unit
test: test-clean test-unit ## Run test-unit
test-unit: ## Run unit tests
	@echo "Running unit tests..."
	@set -o pipefail ; \
		$(GO) test -timeout $(TIMEOUT_UNIT) $(ARGS) ./... | { grep -v 'no test files'; true; }


.PHONY: lint
lint: lint-go lint-yaml ## run all linters

.PHONY: lint-go
lint-go: | $(GOLANGCILINT) ## runs go linter on all go files
	@echo "Linting go files..."
	@$(GOLANGCILINT) run ./... --modules-download-mode=vendor \
							--max-issues-per-linter=0 \
							--max-same-issues=0 \
							--deadline 5m

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
