module github.com/tektoncd/operator

require (
	github.com/go-logr/zapr v0.1.1
	github.com/google/go-cmp v0.5.2
	github.com/manifestival/client-go-client v0.4.0
	github.com/manifestival/manifestival v0.6.1
	github.com/tektoncd/plumbing v0.0.0-20200430135134-e53521e1d887
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a // indirect
	golang.org/x/mod v0.3.0
	honnef.co/go/tools v0.0.1-2020.1.5 // indirect
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/code-generator v0.18.8
	knative.dev/pkg v0.0.0-20201026165741-2f75016c1368
)

go 1.14

// Pin k8s deps to 0.18.8
replace (
	k8s.io/api => k8s.io/api v0.18.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.8
	k8s.io/code-generator => k8s.io/code-generator v0.18.8
)
