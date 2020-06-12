.PHONY: all
all: local-dev

.PHONY: clean
clean:
	-kubectl delete -f deploy/
	-kubectl delete -f deploy/crds/
	-kubectl delete namespace tekton-pipelines

.PHONY: local-dev
local-dev: dev-setup
	GO111MODULE=on operator-sdk run local --watch-namespace "" --operator-flags '--zap-encoder=console'

.PHONY: update-deps dev-setup
dev-setup:
	kubectl create namespace tekton-pipelines
	kubectl apply -f deploy/crds/operator_v1alpha1_addon_crd.yaml
	kubectl apply -f deploy/crds/operator_v1alpha1_config_crd.yaml
	kubectl apply -f deploy/service_account.yaml
	kubectl apply -f deploy/role.yaml
	kubectl apply -f deploy/role_binding.yaml

.PHONY: update-deps
update-deps:
	GO111MODULE=on go mod tidy

.PHONY: local-test-e2e
local-test-e2e:
	GO111MODULE=on \
	operator-sdk test local ./test/e2e  \
	--up-local \
	--watch-namespace "" \
	--operator-namespace operators \
	--debug  \
	--verbose
