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

package filtered

import (
	context "context"

	apisoperatorv1alpha1 "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	versioned "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	v1alpha1 "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	client "github.com/tektoncd/operator/pkg/client/injection/client"
	filtered "github.com/tektoncd/operator/pkg/client/injection/informers/factory/filtered"
	operatorv1alpha1 "github.com/tektoncd/operator/pkg/client/listers/operator/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	cache "k8s.io/client-go/tools/cache"
	controller "knative.dev/pkg/controller"
	injection "knative.dev/pkg/injection"
	logging "knative.dev/pkg/logging"
)

func init() {
	injection.Default.RegisterFilteredInformers(withInformer)
	injection.Dynamic.RegisterDynamicInformer(withDynamicInformer)
}

// Key is used for associating the Informer inside the context.Context.
type Key struct {
	Selector string
}

func withInformer(ctx context.Context) (context.Context, []controller.Informer) {
	untyped := ctx.Value(filtered.LabelKey{})
	if untyped == nil {
		logging.FromContext(ctx).Panic(
			"Unable to fetch labelkey from context.")
	}
	labelSelectors := untyped.([]string)
	infs := []controller.Informer{}
	for _, selector := range labelSelectors {
		f := filtered.Get(ctx, selector)
		inf := f.Operator().V1alpha1().TektonChainses()
		ctx = context.WithValue(ctx, Key{Selector: selector}, inf)
		infs = append(infs, inf.Informer())
	}
	return ctx, infs
}

func withDynamicInformer(ctx context.Context) context.Context {
	untyped := ctx.Value(filtered.LabelKey{})
	if untyped == nil {
		logging.FromContext(ctx).Panic(
			"Unable to fetch labelkey from context.")
	}
	labelSelectors := untyped.([]string)
	for _, selector := range labelSelectors {
		inf := &wrapper{client: client.Get(ctx), selector: selector}
		ctx = context.WithValue(ctx, Key{Selector: selector}, inf)
	}
	return ctx
}

// Get extracts the typed informer from the context.
func Get(ctx context.Context, selector string) v1alpha1.TektonChainsInformer {
	untyped := ctx.Value(Key{Selector: selector})
	if untyped == nil {
		logging.FromContext(ctx).Panicf(
			"Unable to fetch github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1.TektonChainsInformer with selector %s from context.", selector)
	}
	return untyped.(v1alpha1.TektonChainsInformer)
}

type wrapper struct {
	client versioned.Interface

	selector string
}

var _ v1alpha1.TektonChainsInformer = (*wrapper)(nil)
var _ operatorv1alpha1.TektonChainsLister = (*wrapper)(nil)

func (w *wrapper) Informer() cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(nil, &apisoperatorv1alpha1.TektonChains{}, 0, nil)
}

func (w *wrapper) Lister() operatorv1alpha1.TektonChainsLister {
	return w
}

func (w *wrapper) List(selector labels.Selector) (ret []*apisoperatorv1alpha1.TektonChains, err error) {
	reqs, err := labels.ParseToRequirements(w.selector)
	if err != nil {
		return nil, err
	}
	selector = selector.Add(reqs...)
	lo, err := w.client.OperatorV1alpha1().TektonChainses().List(context.TODO(), v1.ListOptions{
		LabelSelector: selector.String(),
		// TODO(mattmoor): Incorporate resourceVersion bounds based on staleness criteria.
	})
	if err != nil {
		return nil, err
	}
	for idx := range lo.Items {
		ret = append(ret, &lo.Items[idx])
	}
	return ret, nil
}

func (w *wrapper) Get(name string) (*apisoperatorv1alpha1.TektonChains, error) {
	// TODO(mattmoor): Check that the fetched object matches the selector.
	return w.client.OperatorV1alpha1().TektonChainses().Get(context.TODO(), name, v1.GetOptions{
		// TODO(mattmoor): Incorporate resourceVersion bounds based on staleness criteria.
	})
}
