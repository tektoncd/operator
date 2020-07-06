package manifestival

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Predicate returns true if u should be included in result
type Predicate func(u *unstructured.Unstructured) bool

var (
	Everything = func(u *unstructured.Unstructured) bool { return true }
	Nothing    = Not(Everything)
)

// Filter returns a Manifest containing only the resources for which
// *all* Predicates return true. Any changes callers make to the
// resources passed to their Predicate[s] will only be reflected in
// the returned Manifest.
func (m Manifest) Filter(preds ...Predicate) Manifest {
	result := m
	result.resources = []unstructured.Unstructured{}
	pred := Everything
	if len(preds) > 0 {
		pred = All(preds[0], preds[1:]...)
	}
	for _, spec := range m.Resources() {
		if !pred(&spec) {
			continue
		}
		result.resources = append(result.resources, spec)
	}
	return result
}

// All returns true iff all of the predicates are true
func All(pred Predicate, preds ...Predicate) Predicate {
	return func(u *unstructured.Unstructured) bool {
		for _, p := range append([]Predicate{pred}, preds...) {
			if !p(u) {
				return false
			}
		}
		return true
	}
}

// Any returns true iff any of the predicates are true
func Any(pred Predicate, preds ...Predicate) Predicate {
	return func(u *unstructured.Unstructured) bool {
		for _, p := range append([]Predicate{pred}, preds...) {
			if p(u) {
				return true
			}
		}
		return false
	}
}

// Not returns the complement of a given predicate.
func Not(pred Predicate) Predicate {
	return func(u *unstructured.Unstructured) bool {
		return !pred(u)
	}
}

// CRDs returns only CustomResourceDefinitions
var CRDs = ByKind("CustomResourceDefinition")

// NoCRDs returns no CustomResourceDefinitions
var NoCRDs = Not(CRDs)

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

// ByAnnotation returns resources that contain a particular annotation
// and value. A value of "" denotes *ANY* value
func ByAnnotation(annotation, value string) Predicate {
	return func(u *unstructured.Unstructured) bool {
		v, ok := u.GetAnnotations()[annotation]
		if value == "" {
			return ok
		}
		return v == value
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

// In(m) returns a Predicate that tests for membership in m, using
// "gvk|namespace/name" as a unique identifier
func In(manifest Manifest) Predicate {
	key := func(u *unstructured.Unstructured) string {
		return fmt.Sprintf("%s|%s/%s", u.GroupVersionKind(), u.GetNamespace(), u.GetName())
	}
	index := sets.NewString()
	for _, u := range manifest.resources {
		index.Insert(key(&u))
	}
	return func(u *unstructured.Unstructured) bool {
		return index.Has(key(u))
	}
}
