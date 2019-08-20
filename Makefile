.PHONY: local-dev
local-dev:
	GO111MODULE=on operator-sdk up local --namespace "" --operator-flags '--zap-encoder=console'
