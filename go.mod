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
	github.com/tektoncd/pipeline v0.27.3
	github.com/tektoncd/plumbing v0.0.0-20210514044347-f8a9689d5bd5
	github.com/tektoncd/triggers v0.16.0
	go.opencensus.io v0.23.0
	go.uber.org/zap v1.19.0
	golang.org/x/mod v0.5.0
	gomodules.xyz/jsonpatch/v2 v2.2.0
	gotest.tools/v3 v3.0.3
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/code-generator v0.21.4
	knative.dev/pkg v0.0.0-20210827184538-2bd91f75571c
	sigs.k8s.io/controller-runtime v0.7.2
)

go 1.14
