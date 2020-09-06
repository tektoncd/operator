module github.com/tektoncd/operator

require (
	github.com/emicklei/go-restful v2.11.1+incompatible // indirect
	github.com/go-logr/zapr v0.1.1
	github.com/manifestival/client-go-client v0.3.0
	github.com/manifestival/manifestival v0.6.1
	github.com/tektoncd/plumbing v0.0.0-20200430135134-e53521e1d887
	go.uber.org/zap v1.15.0
	golang.org/x/mod v0.3.0
	gonum.org/v1/gonum v0.0.0-20190710053202-4340aa3071a0 // indirect
	k8s.io/api v0.18.7-rc.0
	k8s.io/apimachinery v0.18.7-rc.0
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/code-generator v0.18.6
	knative.dev/pkg v0.0.0-20200902221531-b0307fc6d285
	knative.dev/test-infra v0.0.0-20200828211307-9d4372c9b1c7
)

go 1.14

replace k8s.io/api => k8s.io/api v0.17.6

replace k8s.io/apimachinery => k8s.io/apimachinery v0.17.6

replace k8s.io/client-go => k8s.io/client-go v0.17.6

replace k8s.io/code-generator => k8s.io/code-generator v0.17.6
