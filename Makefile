.PHONY: local-dev
local-dev:
	GO111MODULE=on operator-sdk up local --namespace "" --operator-flags '--zap-encoder=console'

update-deps:
	GO111MODULE=on go mod tidy

local-test-e2e:
	GO111MODULE=on \
	operator-sdk test local ./test/e2e  \
	--up-local \
	--namespace operators \
	--debug  \
	--verbose
