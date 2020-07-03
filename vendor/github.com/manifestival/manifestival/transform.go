package manifestival

import (
	"os"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Transformer transforms a resource from the manifest in place.
type Transformer func(u *unstructured.Unstructured) error

// Owner is a partial Kubernetes metadata schema.
type Owner interface {
	v1.Object
	schema.ObjectKind
}

// Transform applies an ordered set of Transformer functions to the
// `Resources` in this Manifest.  If an error occurs, no resources are
// transformed.
func (m Manifest) Transform(fns ...Transformer) (Manifest, error) {
	result := m
	result.resources = m.Resources() // deep copies
	for i := range result.resources {
		spec := &result.resources[i]
		for _, transform := range fns {
			if transform != nil {
				err := transform(spec)
				if err != nil {
					return Manifest{}, err
				}
			}
		}
	}
	return result, nil
}

// InjectNamespace creates a Transformer which adds a namespace to existing
// resources if appropriate. We assume all resources in the manifest live in
// the same namespace.
func InjectNamespace(ns string) Transformer {
	namespace := resolveEnv(ns)
	return func(u *unstructured.Unstructured) error {
		switch strings.ToLower(u.GetKind()) {
		case "namespace":
			u.SetName(namespace)
		case "clusterrolebinding", "rolebinding":
			subjects, _, _ := unstructured.NestedFieldNoCopy(u.Object, "subjects")
			for _, subject := range subjects.([]interface{}) {
				m := subject.(map[string]interface{})
				if _, ok := m["namespace"]; ok {
					m["namespace"] = namespace
				}
			}
		case "validatingwebhookconfiguration", "mutatingwebhookconfiguration":
			hooks, _, _ := unstructured.NestedFieldNoCopy(u.Object, "webhooks")
			for _, hook := range hooks.([]interface{}) {
				m := hook.(map[string]interface{})
				if c, ok := m["clientConfig"]; ok {
					cfg := c.(map[string]interface{})
					if s, ok := cfg["service"]; ok {
						srv := s.(map[string]interface{})
						srv["namespace"] = namespace
					}
				}
			}
		case "apiservice":
			spec, _, _ := unstructured.NestedFieldNoCopy(u.Object, "spec")
			m := spec.(map[string]interface{})
			if c, ok := m["service"]; ok {
				srv := c.(map[string]interface{})
				srv["namespace"] = namespace
			}
		}
		if !isClusterScoped(u.GetKind()) {
			u.SetNamespace(namespace)
		}
		return nil
	}
}

// InjectOwner creates a Tranformer which adds an OwnerReference pointing to
// `owner` to namespace-scoped objects.
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
