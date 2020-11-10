package client

import (
	"context"
	mfDynamic "github.com/manifestival/client-go-client/pkg/dynamic"
	mf "github.com/manifestival/manifestival"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// NewManifest returns a manifestival Manifest based on a path and config
func NewManifest(pathname string, config *rest.Config, opts ...mf.Option) (mf.Manifest, error) {
	client, err := NewClient(config)
	if err != nil {
		return mf.Manifest{}, err
	}
	return mf.NewManifest(pathname, append(opts, mf.UseClient(client))...)
}

// NewClient returns a manifestival client based on a rest config
func NewClient(config *rest.Config) (mf.Client, error) {
	resourceGetter, err := mfDynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &clientGoClient{resourceGetter: resourceGetter}, nil
}

// NewUnsafeDynamicClient returns a manifestival client based on dynamic kubernetes client
func NewUnsafeDynamicClient(client dynamic.Interface) (mf.Client, error) {
	resourceGetter := mfDynamic.NewUnsafeResourceGetter(client)
	return &clientGoClient{resourceGetter: resourceGetter}, nil
}

type clientGoClient struct {
	resourceGetter mfDynamic.ResourceGetter
}

// verify implementation
var _ mf.Client = (*clientGoClient)(nil)

func (c *clientGoClient) Create(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	resource, err := c.resourceGetter.ResourceInterface(obj)
	if err != nil {
		return err
	}
	opts := mf.ApplyWith(options)
	_, err = resource.Create(context.TODO(), obj, *opts.ForCreate)
	return err
}

func (c *clientGoClient) Update(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	resource, err := c.resourceGetter.ResourceInterface(obj)
	if err != nil {
		return err
	}
	opts := mf.ApplyWith(options)
	_, err = resource.Update(context.TODO(), obj, *opts.ForUpdate)
	return err
}

func (c *clientGoClient) Delete(obj *unstructured.Unstructured, options ...mf.DeleteOption) error {
	resource, err := c.resourceGetter.ResourceInterface(obj)
	if err != nil {
		return err
	}
	opts := mf.DeleteWith(options)
	err = resource.Delete(context.TODO(), obj.GetName(), *opts.ForDelete)
	if apierrors.IsNotFound(err) && opts.IgnoreNotFound {
		return nil
	}
	return err
}

func (c *clientGoClient) Get(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	resource, err := c.resourceGetter.ResourceInterface(obj)
	if err != nil {
		return nil, err
	}
	return resource.Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
}
