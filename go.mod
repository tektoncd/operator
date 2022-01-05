module github.com/tektoncd/operator

require (
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0
	github.com/google/go-cmp v0.5.6
	github.com/manifestival/client-go-client v0.5.0
	github.com/manifestival/controller-runtime-client v0.4.0
	github.com/manifestival/manifestival v0.7.0
	github.com/markbates/inflect v1.0.4
	github.com/openshift/api v0.0.0-20210910062324-a41d3573a3ba
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142
	github.com/tektoncd/pipeline v0.31.0
	github.com/tektoncd/plumbing v0.0.0-20211012143332-c7cc43d9bc0c
	github.com/tektoncd/triggers v0.18.0
	go.opencensus.io v0.23.0
	go.uber.org/zap v1.19.1
	golang.org/x/mod v0.5.1
	gomodules.xyz/jsonpatch/v2 v2.2.0
	gotest.tools/v3 v3.0.3
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/code-generator v0.21.4
	knative.dev/pkg v0.0.0-20211206113427-18589ac7627e
	sigs.k8s.io/controller-runtime v0.7.2
)

go 1.14
