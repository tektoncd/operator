.PHONY: local-dev
local-dev:
	operator-sdk up local --namespace "" --operator-flags '--zap-encoder=console'

update-deps:
	GO111MODULE=on go mod tidy
