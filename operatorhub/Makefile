BUNDLE_ARGS := $(or ${BUNDLE_ARGS},--help)

OPERATOR_SDK = $(or ${OPERATOR_SDK_BIN},$(BIN)/operator-sdk)

$(BIN)/operator-sdk: | $(BIN) ; $(info $(M) getting operator-sdk)
	ls $(BIN)
	@./operatorhub/tools/install_operator-sdk.sh $(BIN)

.PHONY: operator-bundle
operator-bundle: | $(OPERATOR_SDK)
	@$(info BUNDLE_ARGS: $(BUNDLE_ARGS))
	@OPERATOR_SDK=$(OPERATOR_SDK) operatorhub/tools/bundle.py ${BUNDLE_ARGS}
