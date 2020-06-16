module github.com/tektoncd/operator

require (
	github.com/go-logr/logr v0.1.0
	github.com/jcrossley3/manifestival v0.0.0-20191210210340-41c7c3a22365
	github.com/operator-framework/operator-sdk v0.17.0
	github.com/prometheus/common v0.9.1
	github.com/spf13/pflag v1.0.5
	github.com/tektoncd/plumbing v0.0.0-20200430135134-e53521e1d887
	golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543
	k8s.io/api v0.17.6
	k8s.io/apimachinery v0.17.6
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.5.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6 // Required by prometheus-operator
)

go 1.13
