package dynamic

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type clientRestMapper struct {
	client dynamic.Interface
	mapper meta.RESTMapper
}

// NewForConfig returns a ResourceGetter for a rest config.
func NewForConfig(config *rest.Config) (ResourceGetter, error) {
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, err
	}
	mapper, err := apiutil.NewDynamicRESTMapper(config, httpClient)

	if err != nil {
		return nil, err
	}

	resourceGetter := &clientRestMapper{
		client: client,
		mapper: mapper,
	}
	return resourceGetter, nil
}

func (cm *clientRestMapper) ResourceInterface(obj *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	gvk := obj.GroupVersionKind()
	mapping, err := cm.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	if mapping.Scope.Name() == meta.RESTScopeNameRoot {
		return cm.client.Resource(mapping.Resource), nil
	}
	return cm.client.Resource(mapping.Resource).Namespace(obj.GetNamespace()), nil
}

var _ ResourceGetter = (*clientRestMapper)(nil)
