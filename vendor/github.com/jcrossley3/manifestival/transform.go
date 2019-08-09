package manifestival

import (
	"os"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Transform a resource from the manifest
type Transformer func(u *unstructured.Unstructured) error

type Owner interface {
	v1.Object
	schema.ObjectKind
}

// If an error occurs, no resources are transformed
func (f *Manifest) Transform(fns ...Transformer) error {
	var results []unstructured.Unstructured
	for i := 0; i < len(f.Resources); i++ {
		spec := f.Resources[i].DeepCopy()
		for _, transform := range fns {
			err := transform(spec)
			if err != nil {
				return err
			}
		}
		results = append(results, *spec)
	}
	f.Resources = results
	return nil
}

// We assume all resources in the manifest live in the same namespace
func InjectNamespace(ns string) Transformer {
	namespace := resolveEnv(ns)
	return func(u *unstructured.Unstructured) error {
		switch strings.ToLower(u.GetKind()) {
		case "namespace":
			u.SetName(namespace)
		case "clusterrolebinding":
			subjects, _, _ := unstructured.NestedFieldNoCopy(u.Object, "subjects")
			for _, subject := range subjects.([]interface{}) {
				m := subject.(map[string]interface{})
				if _, ok := m["namespace"]; ok {
					m["namespace"] = namespace
				}
			}
		}
		if !isClusterScoped(u.GetKind()) {
			u.SetNamespace(namespace)
		}
		return nil
	}
}

func InjectOwner(owner Owner) Transformer {
	return func(u *unstructured.Unstructured) error {
		if !isClusterScoped(u.GetKind()) {
			// apparently reference counting for cluster-scoped
			// resources is broken, so trust the GC only for ns-scoped
			// dependents
			u.SetOwnerReferences([]v1.OwnerReference{*v1.NewControllerRef(owner, owner.GroupVersionKind())})
		}
		return nil
	}
}

func isClusterScoped(kind string) bool {
	// TODO: something more clever using !APIResource.Namespaced maybe?
	switch strings.ToLower(kind) {
	case "componentstatus",
		"namespace",
		"node",
		"persistentvolume",
		"mutatingwebhookconfiguration",
		"validatingwebhookconfiguration",
		"customresourcedefinition",
		"apiservice",
		"meshpolicy",
		"tokenreview",
		"selfsubjectaccessreview",
		"selfsubjectrulesreview",
		"subjectaccessreview",
		"certificatesigningrequest",
		"podsecuritypolicy",
		"clusterrolebinding",
		"clusterrole",
		"priorityclass",
		"storageclass",
		"volumeattachment":
		return true
	}
	return false
}

func resolveEnv(x string) string {
	if len(x) > 1 && x[:1] == "$" {
		return os.Getenv(x[1:])
	}
	return x
}
