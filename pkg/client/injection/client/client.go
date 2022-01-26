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

// Code generated by injection-gen. DO NOT EDIT.

package client

import (
	context "context"
	json "encoding/json"
	errors "errors"
	fmt "fmt"

	v1alpha1 "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	versioned "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	typedoperatorv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	discovery "k8s.io/client-go/discovery"
	dynamic "k8s.io/client-go/dynamic"
	rest "k8s.io/client-go/rest"
	injection "knative.dev/pkg/injection"
	dynamicclient "knative.dev/pkg/injection/clients/dynamicclient"
	logging "knative.dev/pkg/logging"
)

func init() {
	injection.Default.RegisterClient(withClientFromConfig)
	injection.Default.RegisterClientFetcher(func(ctx context.Context) interface{} {
		return Get(ctx)
	})
	injection.Dynamic.RegisterDynamicClient(withClientFromDynamic)
}

// Key is used as the key for associating information with a context.Context.
type Key struct{}

func withClientFromConfig(ctx context.Context, cfg *rest.Config) context.Context {
	return context.WithValue(ctx, Key{}, versioned.NewForConfigOrDie(cfg))
}

func withClientFromDynamic(ctx context.Context) context.Context {
	return context.WithValue(ctx, Key{}, &wrapClient{dyn: dynamicclient.Get(ctx)})
}

// Get extracts the versioned.Interface client from the context.
func Get(ctx context.Context) versioned.Interface {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		if injection.GetConfig(ctx) == nil {
			logging.FromContext(ctx).Panic(
				"Unable to fetch github.com/tektoncd/operator/pkg/client/clientset/versioned.Interface from context. This context is not the application context (which is typically given to constructors via sharedmain).")
		} else {
			logging.FromContext(ctx).Panic(
				"Unable to fetch github.com/tektoncd/operator/pkg/client/clientset/versioned.Interface from context.")
		}
	}
	return untyped.(versioned.Interface)
}

type wrapClient struct {
	dyn dynamic.Interface
}

var _ versioned.Interface = (*wrapClient)(nil)

func (w *wrapClient) Discovery() discovery.DiscoveryInterface {
	panic("Discovery called on dynamic client!")
}

func convert(from interface{}, to runtime.Object) error {
	bs, err := json.Marshal(from)
	if err != nil {
		return fmt.Errorf("Marshal() = %w", err)
	}
	if err := json.Unmarshal(bs, to); err != nil {
		return fmt.Errorf("Unmarshal() = %w", err)
	}
	return nil
}

// OperatorV1alpha1 retrieves the OperatorV1alpha1Client
func (w *wrapClient) OperatorV1alpha1() typedoperatorv1alpha1.OperatorV1alpha1Interface {
	return &wrapOperatorV1alpha1{
		dyn: w.dyn,
	}
}

type wrapOperatorV1alpha1 struct {
	dyn dynamic.Interface
}

func (w *wrapOperatorV1alpha1) RESTClient() rest.Interface {
	panic("RESTClient called on dynamic client!")
}

func (w *wrapOperatorV1alpha1) TektonAddons() typedoperatorv1alpha1.TektonAddonInterface {
	return &wrapOperatorV1alpha1TektonAddonImpl{
		dyn: w.dyn.Resource(schema.GroupVersionResource{
			Group:    "operator.tekton.dev",
			Version:  "v1alpha1",
			Resource: "tektonaddons",
		}),
	}
}

type wrapOperatorV1alpha1TektonAddonImpl struct {
	dyn dynamic.NamespaceableResourceInterface
}

var _ typedoperatorv1alpha1.TektonAddonInterface = (*wrapOperatorV1alpha1TektonAddonImpl)(nil)

func (w *wrapOperatorV1alpha1TektonAddonImpl) Create(ctx context.Context, in *v1alpha1.TektonAddon, opts v1.CreateOptions) (*v1alpha1.TektonAddon, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonAddon",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Create(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonAddon{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonAddonImpl) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return w.dyn.Delete(ctx, name, opts)
}

func (w *wrapOperatorV1alpha1TektonAddonImpl) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	return w.dyn.DeleteCollection(ctx, opts, listOpts)
}

func (w *wrapOperatorV1alpha1TektonAddonImpl) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TektonAddon, error) {
	uo, err := w.dyn.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonAddon{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonAddonImpl) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TektonAddonList, error) {
	uo, err := w.dyn.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonAddonList{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonAddonImpl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TektonAddon, err error) {
	uo, err := w.dyn.Patch(ctx, name, pt, data, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonAddon{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonAddonImpl) Update(ctx context.Context, in *v1alpha1.TektonAddon, opts v1.UpdateOptions) (*v1alpha1.TektonAddon, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonAddon",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Update(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonAddon{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonAddonImpl) UpdateStatus(ctx context.Context, in *v1alpha1.TektonAddon, opts v1.UpdateOptions) (*v1alpha1.TektonAddon, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonAddon",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.UpdateStatus(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonAddon{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonAddonImpl) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("NYI: Watch")
}

func (w *wrapOperatorV1alpha1) TektonChainses() typedoperatorv1alpha1.TektonChainsInterface {
	return &wrapOperatorV1alpha1TektonChainsImpl{
		dyn: w.dyn.Resource(schema.GroupVersionResource{
			Group:    "operator.tekton.dev",
			Version:  "v1alpha1",
			Resource: "tektonchainses",
		}),
	}
}

type wrapOperatorV1alpha1TektonChainsImpl struct {
	dyn dynamic.NamespaceableResourceInterface
}

var _ typedoperatorv1alpha1.TektonChainsInterface = (*wrapOperatorV1alpha1TektonChainsImpl)(nil)

func (w *wrapOperatorV1alpha1TektonChainsImpl) Create(ctx context.Context, in *v1alpha1.TektonChains, opts v1.CreateOptions) (*v1alpha1.TektonChains, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonChains",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Create(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonChains{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonChainsImpl) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return w.dyn.Delete(ctx, name, opts)
}

func (w *wrapOperatorV1alpha1TektonChainsImpl) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	return w.dyn.DeleteCollection(ctx, opts, listOpts)
}

func (w *wrapOperatorV1alpha1TektonChainsImpl) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TektonChains, error) {
	uo, err := w.dyn.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonChains{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonChainsImpl) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TektonChainsList, error) {
	uo, err := w.dyn.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonChainsList{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonChainsImpl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TektonChains, err error) {
	uo, err := w.dyn.Patch(ctx, name, pt, data, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonChains{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonChainsImpl) Update(ctx context.Context, in *v1alpha1.TektonChains, opts v1.UpdateOptions) (*v1alpha1.TektonChains, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonChains",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Update(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonChains{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonChainsImpl) UpdateStatus(ctx context.Context, in *v1alpha1.TektonChains, opts v1.UpdateOptions) (*v1alpha1.TektonChains, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonChains",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.UpdateStatus(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonChains{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonChainsImpl) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("NYI: Watch")
}

func (w *wrapOperatorV1alpha1) TektonConfigs() typedoperatorv1alpha1.TektonConfigInterface {
	return &wrapOperatorV1alpha1TektonConfigImpl{
		dyn: w.dyn.Resource(schema.GroupVersionResource{
			Group:    "operator.tekton.dev",
			Version:  "v1alpha1",
			Resource: "tektonconfigs",
		}),
	}
}

type wrapOperatorV1alpha1TektonConfigImpl struct {
	dyn dynamic.NamespaceableResourceInterface
}

var _ typedoperatorv1alpha1.TektonConfigInterface = (*wrapOperatorV1alpha1TektonConfigImpl)(nil)

func (w *wrapOperatorV1alpha1TektonConfigImpl) Create(ctx context.Context, in *v1alpha1.TektonConfig, opts v1.CreateOptions) (*v1alpha1.TektonConfig, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonConfig",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Create(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonConfig{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonConfigImpl) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return w.dyn.Delete(ctx, name, opts)
}

func (w *wrapOperatorV1alpha1TektonConfigImpl) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	return w.dyn.DeleteCollection(ctx, opts, listOpts)
}

func (w *wrapOperatorV1alpha1TektonConfigImpl) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TektonConfig, error) {
	uo, err := w.dyn.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonConfig{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonConfigImpl) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TektonConfigList, error) {
	uo, err := w.dyn.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonConfigList{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonConfigImpl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TektonConfig, err error) {
	uo, err := w.dyn.Patch(ctx, name, pt, data, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonConfig{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonConfigImpl) Update(ctx context.Context, in *v1alpha1.TektonConfig, opts v1.UpdateOptions) (*v1alpha1.TektonConfig, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonConfig",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Update(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonConfig{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonConfigImpl) UpdateStatus(ctx context.Context, in *v1alpha1.TektonConfig, opts v1.UpdateOptions) (*v1alpha1.TektonConfig, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonConfig",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.UpdateStatus(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonConfig{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonConfigImpl) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("NYI: Watch")
}

func (w *wrapOperatorV1alpha1) TektonDashboards() typedoperatorv1alpha1.TektonDashboardInterface {
	return &wrapOperatorV1alpha1TektonDashboardImpl{
		dyn: w.dyn.Resource(schema.GroupVersionResource{
			Group:    "operator.tekton.dev",
			Version:  "v1alpha1",
			Resource: "tektondashboards",
		}),
	}
}

type wrapOperatorV1alpha1TektonDashboardImpl struct {
	dyn dynamic.NamespaceableResourceInterface
}

var _ typedoperatorv1alpha1.TektonDashboardInterface = (*wrapOperatorV1alpha1TektonDashboardImpl)(nil)

func (w *wrapOperatorV1alpha1TektonDashboardImpl) Create(ctx context.Context, in *v1alpha1.TektonDashboard, opts v1.CreateOptions) (*v1alpha1.TektonDashboard, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonDashboard",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Create(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonDashboard{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonDashboardImpl) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return w.dyn.Delete(ctx, name, opts)
}

func (w *wrapOperatorV1alpha1TektonDashboardImpl) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	return w.dyn.DeleteCollection(ctx, opts, listOpts)
}

func (w *wrapOperatorV1alpha1TektonDashboardImpl) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TektonDashboard, error) {
	uo, err := w.dyn.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonDashboard{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonDashboardImpl) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TektonDashboardList, error) {
	uo, err := w.dyn.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonDashboardList{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonDashboardImpl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TektonDashboard, err error) {
	uo, err := w.dyn.Patch(ctx, name, pt, data, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonDashboard{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonDashboardImpl) Update(ctx context.Context, in *v1alpha1.TektonDashboard, opts v1.UpdateOptions) (*v1alpha1.TektonDashboard, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonDashboard",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Update(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonDashboard{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonDashboardImpl) UpdateStatus(ctx context.Context, in *v1alpha1.TektonDashboard, opts v1.UpdateOptions) (*v1alpha1.TektonDashboard, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonDashboard",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.UpdateStatus(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonDashboard{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonDashboardImpl) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("NYI: Watch")
}

func (w *wrapOperatorV1alpha1) TektonHubs() typedoperatorv1alpha1.TektonHubInterface {
	return &wrapOperatorV1alpha1TektonHubImpl{
		dyn: w.dyn.Resource(schema.GroupVersionResource{
			Group:    "operator.tekton.dev",
			Version:  "v1alpha1",
			Resource: "tektonhubs",
		}),
	}
}

type wrapOperatorV1alpha1TektonHubImpl struct {
	dyn dynamic.NamespaceableResourceInterface
}

var _ typedoperatorv1alpha1.TektonHubInterface = (*wrapOperatorV1alpha1TektonHubImpl)(nil)

func (w *wrapOperatorV1alpha1TektonHubImpl) Create(ctx context.Context, in *v1alpha1.TektonHub, opts v1.CreateOptions) (*v1alpha1.TektonHub, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonHub",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Create(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonHub{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonHubImpl) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return w.dyn.Delete(ctx, name, opts)
}

func (w *wrapOperatorV1alpha1TektonHubImpl) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	return w.dyn.DeleteCollection(ctx, opts, listOpts)
}

func (w *wrapOperatorV1alpha1TektonHubImpl) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TektonHub, error) {
	uo, err := w.dyn.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonHub{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonHubImpl) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TektonHubList, error) {
	uo, err := w.dyn.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonHubList{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonHubImpl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TektonHub, err error) {
	uo, err := w.dyn.Patch(ctx, name, pt, data, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonHub{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonHubImpl) Update(ctx context.Context, in *v1alpha1.TektonHub, opts v1.UpdateOptions) (*v1alpha1.TektonHub, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonHub",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Update(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonHub{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonHubImpl) UpdateStatus(ctx context.Context, in *v1alpha1.TektonHub, opts v1.UpdateOptions) (*v1alpha1.TektonHub, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonHub",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.UpdateStatus(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonHub{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonHubImpl) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("NYI: Watch")
}

func (w *wrapOperatorV1alpha1) TektonInstallerSets() typedoperatorv1alpha1.TektonInstallerSetInterface {
	return &wrapOperatorV1alpha1TektonInstallerSetImpl{
		dyn: w.dyn.Resource(schema.GroupVersionResource{
			Group:    "operator.tekton.dev",
			Version:  "v1alpha1",
			Resource: "tektoninstallersets",
		}),
	}
}

type wrapOperatorV1alpha1TektonInstallerSetImpl struct {
	dyn dynamic.NamespaceableResourceInterface
}

var _ typedoperatorv1alpha1.TektonInstallerSetInterface = (*wrapOperatorV1alpha1TektonInstallerSetImpl)(nil)

func (w *wrapOperatorV1alpha1TektonInstallerSetImpl) Create(ctx context.Context, in *v1alpha1.TektonInstallerSet, opts v1.CreateOptions) (*v1alpha1.TektonInstallerSet, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonInstallerSet",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Create(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonInstallerSet{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonInstallerSetImpl) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return w.dyn.Delete(ctx, name, opts)
}

func (w *wrapOperatorV1alpha1TektonInstallerSetImpl) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	return w.dyn.DeleteCollection(ctx, opts, listOpts)
}

func (w *wrapOperatorV1alpha1TektonInstallerSetImpl) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TektonInstallerSet, error) {
	uo, err := w.dyn.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonInstallerSet{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonInstallerSetImpl) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TektonInstallerSetList, error) {
	uo, err := w.dyn.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonInstallerSetList{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonInstallerSetImpl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TektonInstallerSet, err error) {
	uo, err := w.dyn.Patch(ctx, name, pt, data, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonInstallerSet{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonInstallerSetImpl) Update(ctx context.Context, in *v1alpha1.TektonInstallerSet, opts v1.UpdateOptions) (*v1alpha1.TektonInstallerSet, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonInstallerSet",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Update(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonInstallerSet{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonInstallerSetImpl) UpdateStatus(ctx context.Context, in *v1alpha1.TektonInstallerSet, opts v1.UpdateOptions) (*v1alpha1.TektonInstallerSet, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonInstallerSet",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.UpdateStatus(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonInstallerSet{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonInstallerSetImpl) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("NYI: Watch")
}

func (w *wrapOperatorV1alpha1) TektonPipelines() typedoperatorv1alpha1.TektonPipelineInterface {
	return &wrapOperatorV1alpha1TektonPipelineImpl{
		dyn: w.dyn.Resource(schema.GroupVersionResource{
			Group:    "operator.tekton.dev",
			Version:  "v1alpha1",
			Resource: "tektonpipelines",
		}),
	}
}

type wrapOperatorV1alpha1TektonPipelineImpl struct {
	dyn dynamic.NamespaceableResourceInterface
}

var _ typedoperatorv1alpha1.TektonPipelineInterface = (*wrapOperatorV1alpha1TektonPipelineImpl)(nil)

func (w *wrapOperatorV1alpha1TektonPipelineImpl) Create(ctx context.Context, in *v1alpha1.TektonPipeline, opts v1.CreateOptions) (*v1alpha1.TektonPipeline, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonPipeline",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Create(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonPipeline{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonPipelineImpl) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return w.dyn.Delete(ctx, name, opts)
}

func (w *wrapOperatorV1alpha1TektonPipelineImpl) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	return w.dyn.DeleteCollection(ctx, opts, listOpts)
}

func (w *wrapOperatorV1alpha1TektonPipelineImpl) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TektonPipeline, error) {
	uo, err := w.dyn.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonPipeline{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonPipelineImpl) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TektonPipelineList, error) {
	uo, err := w.dyn.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonPipelineList{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonPipelineImpl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TektonPipeline, err error) {
	uo, err := w.dyn.Patch(ctx, name, pt, data, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonPipeline{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonPipelineImpl) Update(ctx context.Context, in *v1alpha1.TektonPipeline, opts v1.UpdateOptions) (*v1alpha1.TektonPipeline, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonPipeline",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Update(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonPipeline{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonPipelineImpl) UpdateStatus(ctx context.Context, in *v1alpha1.TektonPipeline, opts v1.UpdateOptions) (*v1alpha1.TektonPipeline, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonPipeline",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.UpdateStatus(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonPipeline{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonPipelineImpl) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("NYI: Watch")
}

func (w *wrapOperatorV1alpha1) TektonResults() typedoperatorv1alpha1.TektonResultInterface {
	return &wrapOperatorV1alpha1TektonResultImpl{
		dyn: w.dyn.Resource(schema.GroupVersionResource{
			Group:    "operator.tekton.dev",
			Version:  "v1alpha1",
			Resource: "tektonresults",
		}),
	}
}

type wrapOperatorV1alpha1TektonResultImpl struct {
	dyn dynamic.NamespaceableResourceInterface
}

var _ typedoperatorv1alpha1.TektonResultInterface = (*wrapOperatorV1alpha1TektonResultImpl)(nil)

func (w *wrapOperatorV1alpha1TektonResultImpl) Create(ctx context.Context, in *v1alpha1.TektonResult, opts v1.CreateOptions) (*v1alpha1.TektonResult, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonResult",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Create(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonResult{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonResultImpl) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return w.dyn.Delete(ctx, name, opts)
}

func (w *wrapOperatorV1alpha1TektonResultImpl) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	return w.dyn.DeleteCollection(ctx, opts, listOpts)
}

func (w *wrapOperatorV1alpha1TektonResultImpl) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TektonResult, error) {
	uo, err := w.dyn.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonResult{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonResultImpl) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TektonResultList, error) {
	uo, err := w.dyn.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonResultList{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonResultImpl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TektonResult, err error) {
	uo, err := w.dyn.Patch(ctx, name, pt, data, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonResult{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonResultImpl) Update(ctx context.Context, in *v1alpha1.TektonResult, opts v1.UpdateOptions) (*v1alpha1.TektonResult, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonResult",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Update(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonResult{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonResultImpl) UpdateStatus(ctx context.Context, in *v1alpha1.TektonResult, opts v1.UpdateOptions) (*v1alpha1.TektonResult, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonResult",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.UpdateStatus(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonResult{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonResultImpl) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("NYI: Watch")
}

func (w *wrapOperatorV1alpha1) TektonTriggers() typedoperatorv1alpha1.TektonTriggerInterface {
	return &wrapOperatorV1alpha1TektonTriggerImpl{
		dyn: w.dyn.Resource(schema.GroupVersionResource{
			Group:    "operator.tekton.dev",
			Version:  "v1alpha1",
			Resource: "tektontriggers",
		}),
	}
}

type wrapOperatorV1alpha1TektonTriggerImpl struct {
	dyn dynamic.NamespaceableResourceInterface
}

var _ typedoperatorv1alpha1.TektonTriggerInterface = (*wrapOperatorV1alpha1TektonTriggerImpl)(nil)

func (w *wrapOperatorV1alpha1TektonTriggerImpl) Create(ctx context.Context, in *v1alpha1.TektonTrigger, opts v1.CreateOptions) (*v1alpha1.TektonTrigger, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonTrigger",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Create(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonTrigger{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonTriggerImpl) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return w.dyn.Delete(ctx, name, opts)
}

func (w *wrapOperatorV1alpha1TektonTriggerImpl) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	return w.dyn.DeleteCollection(ctx, opts, listOpts)
}

func (w *wrapOperatorV1alpha1TektonTriggerImpl) Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TektonTrigger, error) {
	uo, err := w.dyn.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonTrigger{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonTriggerImpl) List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TektonTriggerList, error) {
	uo, err := w.dyn.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonTriggerList{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonTriggerImpl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TektonTrigger, err error) {
	uo, err := w.dyn.Patch(ctx, name, pt, data, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonTrigger{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonTriggerImpl) Update(ctx context.Context, in *v1alpha1.TektonTrigger, opts v1.UpdateOptions) (*v1alpha1.TektonTrigger, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonTrigger",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.Update(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonTrigger{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonTriggerImpl) UpdateStatus(ctx context.Context, in *v1alpha1.TektonTrigger, opts v1.UpdateOptions) (*v1alpha1.TektonTrigger, error) {
	in.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.tekton.dev",
		Version: "v1alpha1",
		Kind:    "TektonTrigger",
	})
	uo := &unstructured.Unstructured{}
	if err := convert(in, uo); err != nil {
		return nil, err
	}
	uo, err := w.dyn.UpdateStatus(ctx, uo, opts)
	if err != nil {
		return nil, err
	}
	out := &v1alpha1.TektonTrigger{}
	if err := convert(uo, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (w *wrapOperatorV1alpha1TektonTriggerImpl) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("NYI: Watch")
}
