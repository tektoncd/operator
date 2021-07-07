MODULE   = $(shell env GO111MODULE=on $(GO) list -m)
DATE         ?= $(shell date +%FT%T%z)
KO_DATA_PATH  = $(shell pwd)/cmd/$(TARGET)/operator/kodata
TARGET        = kubernetes
CR            = config/basic
PLATFORM := $(if $(PLATFORM),--platform $(PLATFORM))

GOLANGCI_VERSION  = v1.30.0

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
	$Q tmp=$$(mktemp -d); \
	   env GO111MODULE=off GOPATH=$$tmp GOBIN=$(BIN) $(GO) get $(PACKAGE) \
		|| ret=$$?; \
	   rm -rf $$tmp ; exit $$ret

KO = $(or ${KO_BIN},${KO_BIN},$(BIN)/ko)
$(BIN)/ko: PACKAGE=github.com/google/ko/cmd/ko

KUSTOMIZE = $(or ${KUSTOMIZE_BIN},${KUSTOMIZE_BIN},$(BIN)/kustomize)
$(BIN)/kustomize: | $(BIN) ; $(info $(M) getting kustomize)
	@curl -sSfL https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh | bash
	@mv ./kustomize $(BIN)/kustomize

GOLANGCILINT = $(or ${GOLANGCILINT_BIN},${GOLANGCILINT_BIN},$(BIN)/golangci-lint)
$(BIN)/golangci-lint: | $(BIN) ; $(info $(M) getting golangci-lint $(GOLANGCI_VERSION))
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN) $(GOLANGCI_VERSION)

.PHONY: clean-cluster
clean-cluster: | $(KO) $(KUSTOMIZE) clean-cr; $(info $(M) clean $(TARGET)…) @ ## Cleanup cluster
	-$(KUSTOMIZE) build --load-restrictor LoadRestrictionsNone config/$(TARGET) | $(KO) delete -f -
	-kubectl delete ns tekton-pipelines --ignore-not-found
	-kubectl delete \
		-f $(KO_DATA_PATH)/ \
		--ignore-not-found \
		--recursive

.PHONY: clean-bin
clean-bin:
	-rm -rf $(BIN)
	-rm -rf bin
	-rm -rf test/tests.* test/coverage.*

.PHONY: clean
clean: clean-cluster clean-bin; $(info $(M) clean all) @ ## Cleanup everything

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

FORCE:

bin/%: cmd/% FORCE
	$Q $(GO) build -mod=vendor $(LDFLAGS) -v -o $@ ./$<

.PHONY: apply
apply: | $(KO) $(KUSTOMIZE) ; $(info $(M) ko apply on $(TARGET)) @ ## Apply config to the current cluster
	$Q $(KUSTOMIZE) build --load-restrictor LoadRestrictionsNone config/$(TARGET) | $(KO) apply $(PLATFORM) -f -

.PHONY: apply-cr
apply-cr: | ; $(info $(M) apply CRs on $(TARGET)) @ ## Apply the CRs to the current cluster
	$Q kubectl apply -f config/crs/$(TARGET)/$(CR)

.PHONY: clean-cr
clean-cr: | ; $(info $(M) clean CRs on $(TARGET)) @ ## Clean the CRs to the current cluster
	-$Q kubectl delete -f config/crs/$(TARGET)/$(CR)

.PHONY: resolve
resolve: | $(KO) $(KUSTOMIZE) ; $(info $(M) ko resolve on $(TARGET)) @ ## Resolve config to the current cluster
	$Q $(KUSTOMIZE) build --load-restrictor LoadRestrictionsNone config/$(TARGET) | $(KO) resolve --push=false --oci-layout-path=$(BIN)/oci -f -

.PHONY: generated
generated: | vendor ; $(info $(M) update generated files) ## Update generated files
	$Q ./hack/update-codegen.sh

.PHONY: vendor
vendor: ; $(info $(M) update vendor folder)  ## Update vendor folder
	$Q ./hack/update-deps.sh

.PHONY: bundlegen
bundlegen:
	mkdir csv-stub
	kustomize build ../../config/olm-csv-stubs/overlays/openshift > csv-stub/csv-stub.yaml
	pwd
	kustomize build ../../config/olm-bundle-config/openshift | operator-sdk generate bundle \
		--channels stable,preview \
		--default-channel stable \
		--kustomize-dir csv-stub \
		--overwrite \
		--package openshift-pipelines-operator-rh \
		--version 0.22.0-1
	rm -rf csv-stub


bundle:
	$(MAKE) -C operatorhub/openshift --file ../../Makefile bundlegen
