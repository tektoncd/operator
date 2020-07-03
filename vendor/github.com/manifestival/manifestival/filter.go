package manifestival

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Predicate returns true if u should be included in result
type Predicate func(u *unstructured.Unstructured) bool

// Filter returns a Manifest containing only the resources for which
// *all* Predicates return true. Any changes callers make to the
// resources passed to their Predicate[s] will only be reflected in
// the returned Manifest.
func (m Manifest) Filter(preds ...Predicate) Manifest {
	result := m
	result.resources = []unstructured.Unstructured{}
	pred := All(preds...)
	for _, spec := range m.Resources() {
		if !pred(&spec) {
			continue
		}
		result.resources = append(result.resources, spec)
	}
	return result
}

func All(preds ...Predicate) Predicate {
	return func(u *unstructured.Unstructured) bool {
		for _, p := range preds {
			if !p(u) {
				return false
			}
		}
		return true
	}
}

func Any(preds ...Predicate) Predicate {
	return func(u *unstructured.Unstructured) bool {
		for _, p := range preds {
			if p(u) {
				return true
			}
		}
		return false
	}
}

// None returns true iff none of the preds are true
func None(preds ...Predicate) Predicate {
	p := Any(preds...)
	return func(u *unstructured.Unstructured) bool {
		return !p(u)
	}
}

// CRDs returns only CustomResourceDefinitions
var CRDs = ByKind("CustomResourceDefinition")

// NoCRDs returns no CustomResourceDefinitions
var NoCRDs = None(CRDs)

// ByName returns resources with a specifc name
func ByName(name string) Predicate {
	return func(u *unstructured.Unstructured) bool {
		return u.GetName() == name
	}
}

// ByKind returns resources matching a particular kind
func ByKind(kind string) Predicate {
	return func(u *unstructured.Unstructured) bool {
		return u.GetKind() == kind
	}
}

// ByLabel returns resources that contain a particular label and
// value. A value of "" denotes *ANY* value
func ByLabel(label, value string) Predicate {
	return func(u *unstructured.Unstructured) bool {
		v, ok := u.GetLabels()[label]
		if value == "" {
			return ok
		}
		return v == value
	}
}

// ByLabels returns true when the resource contains any of the labels.
func ByLabels(labels map[string]string) Predicate {
	return func(u *unstructured.Unstructured) bool {
		for key, value := range labels {
			if v := u.GetLabels()[key]; v == value {
				return true
			}
		}
		return false
	}
}

// ByGVK returns resources of a particular GroupVersionKind
func ByGVK(gvk schema.GroupVersionKind) Predicate {
	return func(u *unstructured.Unstructured) bool {
		return u.GroupVersionKind() == gvk
	}
}
