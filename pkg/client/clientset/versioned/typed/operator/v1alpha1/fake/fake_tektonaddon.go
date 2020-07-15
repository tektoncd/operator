/*
Copyright 2020 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeTektonAddons implements TektonAddonInterface
type FakeTektonAddons struct {
	Fake *FakeOperatorV1alpha1
}

var tektonaddonsResource = schema.GroupVersionResource{Group: "operator.tekton.dev", Version: "v1alpha1", Resource: "tektonaddons"}

var tektonaddonsKind = schema.GroupVersionKind{Group: "operator.tekton.dev", Version: "v1alpha1", Kind: "TektonAddon"}

// Get takes name of the tektonAddon, and returns the corresponding tektonAddon object, and an error if there is any.
func (c *FakeTektonAddons) Get(name string, options v1.GetOptions) (result *v1alpha1.TektonAddon, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(tektonaddonsResource, name), &v1alpha1.TektonAddon{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TektonAddon), err
}

// List takes label and field selectors, and returns the list of TektonAddons that match those selectors.
func (c *FakeTektonAddons) List(opts v1.ListOptions) (result *v1alpha1.TektonAddonList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(tektonaddonsResource, tektonaddonsKind, opts), &v1alpha1.TektonAddonList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.TektonAddonList{ListMeta: obj.(*v1alpha1.TektonAddonList).ListMeta}
	for _, item := range obj.(*v1alpha1.TektonAddonList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested tektonAddons.
func (c *FakeTektonAddons) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(tektonaddonsResource, opts))
}

// Create takes the representation of a tektonAddon and creates it.  Returns the server's representation of the tektonAddon, and an error, if there is any.
func (c *FakeTektonAddons) Create(tektonAddon *v1alpha1.TektonAddon) (result *v1alpha1.TektonAddon, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(tektonaddonsResource, tektonAddon), &v1alpha1.TektonAddon{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TektonAddon), err
}

// Update takes the representation of a tektonAddon and updates it. Returns the server's representation of the tektonAddon, and an error, if there is any.
func (c *FakeTektonAddons) Update(tektonAddon *v1alpha1.TektonAddon) (result *v1alpha1.TektonAddon, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(tektonaddonsResource, tektonAddon), &v1alpha1.TektonAddon{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TektonAddon), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeTektonAddons) UpdateStatus(tektonAddon *v1alpha1.TektonAddon) (*v1alpha1.TektonAddon, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(tektonaddonsResource, "status", tektonAddon), &v1alpha1.TektonAddon{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TektonAddon), err
}

// Delete takes name of the tektonAddon and deletes it. Returns an error if one occurs.
func (c *FakeTektonAddons) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(tektonaddonsResource, name), &v1alpha1.TektonAddon{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeTektonAddons) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(tektonaddonsResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.TektonAddonList{})
	return err
}

// Patch applies the patch and returns the patched tektonAddon.
func (c *FakeTektonAddons) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.TektonAddon, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(tektonaddonsResource, name, pt, data, subresources...), &v1alpha1.TektonAddon{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.TektonAddon), err
}
