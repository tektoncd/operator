export KO_DATA_PATH=$(shell pwd)/cmd/manager/kodata

.PHONY: all
all: local-dev

.PHONY: clean
clean:
	-ko delete -f config/
	-kubectl delete ns tekton-pipelines --ignore-not-found
	-kubectl delete \
		-f $$KO_DATA_PATH \
		--ignore-not-found \
		--recursive

.PHONY: update-deps dev-setup
dev-setup:
	kubectl apply -f config/100-namespace.yaml
	kubectl apply -f config/300-operator_v1alpha1_addon_crd.yaml
	kubectl apply -f config/300-operator_v1alpha1_pipeline_crd.yaml

.PHONY: local-dev
local-dev: clean dev-setup
	GO111MODULE=on \
	operator-sdk run --local \
		--watch-namespace "" \
		--operator-flags '--zap-encoder=console'

.PHONY: update-deps
update-deps:
	GO111MODULE=on go mod tidy

.PHONY: ko-apply
ko-apply: clean
	ko apply -f config/

.PHONY: local-test-e2e
local-test-e2e: clean dev-setup
	GO111MODULE=on \
	operator-sdk test local ./test/e2e  \
	--up-local \
	--watch-namespace "" \
	--operator-namespace tekton-operators \
	--no-setup \
	--verbose \
	--go-test-flags "-v -timeout 20m"
	--debug
