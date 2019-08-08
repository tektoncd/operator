.PHONY: local-dev
local-dev:
	operator-sdk up local --namespace ""

update-deps:
	GO111MODULE=on go mod tidy
