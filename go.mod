module github.com/tektoncd/operator

require (
	github.com/go-logr/zapr v0.4.0
	github.com/google/go-cmp v0.5.5
	github.com/manifestival/client-go-client v0.5.0
	github.com/manifestival/controller-runtime-client v0.4.0
	github.com/manifestival/manifestival v0.7.0
	github.com/markbates/inflect v1.0.4
	github.com/tektoncd/plumbing v0.0.0-20210514044347-f8a9689d5bd5
	github.com/tektoncd/triggers v0.14.1
	go.opencensus.io v0.23.0
	go.uber.org/zap v1.16.0
	golang.org/x/mod v0.4.1
	gomodules.xyz/jsonpatch/v2 v2.1.0
	gotest.tools v2.2.0+incompatible
	gotest.tools/v3 v3.0.3
	k8s.io/api v0.19.7
	k8s.io/apiextensions-apiserver v0.19.7
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.19.7
	k8s.io/code-generator v0.19.7
	knative.dev/pkg v0.0.0-20210331065221-952fdd90dbb0
	sigs.k8s.io/controller-runtime v0.7.2
)

go 1.14
