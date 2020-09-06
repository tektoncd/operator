package dynamic

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

type unsafeResourceInterface struct {
	client dynamic.Interface
}

// NewUnsafeResourceGetter returns a ResourceGetter based on a dynamic client.
// It is considered unsafe because it makes an assumption on the resource name
// based on the Kind.
func NewUnsafeResourceGetter(client dynamic.Interface) ResourceGetter {
	return &unsafeResourceInterface{
		client: client,
	}
}

func (unsaferi *unsafeResourceInterface) ResourceInterface(obj *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	plural, _ := meta.UnsafeGuessKindToResource(obj.GroupVersionKind())
	return unsaferi.client.Resource(plural).Namespace(obj.GetNamespace()), nil
}

var _ ResourceGetter = (*unsafeResourceInterface)(nil)
