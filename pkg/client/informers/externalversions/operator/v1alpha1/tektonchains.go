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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	operatorv1alpha1 "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	versioned "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	internalinterfaces "github.com/tektoncd/operator/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/tektoncd/operator/pkg/client/listers/operator/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// TektonChainsInformer provides access to a shared informer and lister for
// TektonChainses.
type TektonChainsInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.TektonChainsLister
}

type tektonChainsInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewTektonChainsInformer constructs a new informer for TektonChains type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewTektonChainsInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredTektonChainsInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredTektonChainsInformer constructs a new informer for TektonChains type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredTektonChainsInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.OperatorV1alpha1().TektonChainses().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.OperatorV1alpha1().TektonChainses().Watch(context.TODO(), options)
			},
		},
		&operatorv1alpha1.TektonChains{},
		resyncPeriod,
		indexers,
	)
}

func (f *tektonChainsInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredTektonChainsInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *tektonChainsInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&operatorv1alpha1.TektonChains{}, f.defaultInformer)
}

func (f *tektonChainsInformer) Lister() v1alpha1.TektonChainsLister {
	return v1alpha1.NewTektonChainsLister(f.Informer().GetIndexer())
}
