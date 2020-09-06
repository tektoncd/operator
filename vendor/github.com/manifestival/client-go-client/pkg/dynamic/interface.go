package dynamic

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// ResourceGetter is the interface to returning a Resource
type ResourceGetter interface {
	// ResourceInterface translates an Unstructured object to a ResourceInterface
	// The ResourceInterface can then be used to execute k8s api calls.
	ResourceInterface(obj *unstructured.Unstructured) (dynamic.ResourceInterface, error)
}
