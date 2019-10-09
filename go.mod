module github.com/tektoncd/operator

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/coreos/prometheus-operator v0.31.1 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/jcrossley3/manifestival v0.0.0-20190621184852-78b6b04ae6ff
	github.com/operator-framework/operator-sdk v0.9.1-0.20190715204459-936584d47ff9
	github.com/prometheus/common v0.2.0
	github.com/spf13/pflag v1.0.3
	github.com/tektoncd/plumbing v0.0.0-20191008065817-933f0722e02c
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7
	k8s.io/api v0.0.0-20190612125737-db0771252981
	k8s.io/apiextensions-apiserver v0.0.0-20190820104113-47893d27d7f7 // indirect
	k8s.io/apimachinery v0.0.0-20190612125636-6a5db36e93ad
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208 // indirect
	k8s.io/kube-state-metrics v1.7.2 // indirect
	sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools v0.1.10
)

// Pinned to kubernetes-1.13.4
replace (
	k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190228174230-b40b2a5939e4
)

replace (
	git.apache.org/thrift.git => github.com/apache/thrift v0.0.0-20180902110319-2566ecd5d999
	github.com/coreos/prometheus-operator => github.com/coreos/prometheus-operator v0.29.0
	k8s.io/kube-state-metrics => k8s.io/kube-state-metrics v1.6.0
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools => sigs.k8s.io/controller-tools v0.1.11-0.20190411181648-9d55346c2bde
)

replace github.com/operator-framework/operator-sdk => github.com/operator-framework/operator-sdk v0.9.0
