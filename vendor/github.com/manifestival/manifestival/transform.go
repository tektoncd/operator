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
	updateService := func(obj map[string]interface{}, fields ...string) error {
		srv, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
		if err != nil {
			return err
		}
		if found {
			m := srv.(map[string]interface{})
			if _, ok := m["namespace"]; ok {
				m["namespace"] = namespace
			}
		}
		return nil
	}
	return func(u *unstructured.Unstructured) error {
		if !isClusterScoped(u.GetKind()) {
			u.SetNamespace(namespace)
		}
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
				if err := updateService(hook.(map[string]interface{}), "clientConfig", "service"); err != nil {
					return err
				}
			}
		case "apiservice":
			return updateService(u.Object, "spec", "service")
		case "customresourcedefinition":
			if u.GroupVersionKind().Version == "v1" {
				return updateService(u.Object, "spec", "conversion", "webhook", "clientConfig", "service")
			}
			return updateService(u.Object, "spec", "conversion", "webhookClientConfig", "service")
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
