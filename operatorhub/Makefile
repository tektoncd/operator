BUNDLE_ARGS := $(or ${BUNDLE_ARGS},--help)

BIN = $(CURDIR)/.bin

OPERATOR_SDK = $(or ${OPERATOR_SDK_BIN},$(BIN)/operator-sdk)

$(BIN):
	@mkdir -p $@

$(BIN)/operator-sdk: | $(BIN) ; $(info $(M) getting operator-sdk)
	@./tools/install_operator-sdk.sh $(BIN)

.PHONY: operator-bundle
operator-bundle: | $(OPERATOR_SDK) openshift/release-artifacts/bundle/manifests
	@$(info BUNDLE_ARGS: $(BUNDLE_ARGS))
	@OPERATOR_SDK=$(OPERATOR_SDK) ./tools/bundle.py ${BUNDLE_ARGS}

openshift/release-artifacts/bundle/manifests:
	mkdir -p openshift/release-artifacts/bundle/manifests
