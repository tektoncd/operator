/*
Copyright 2025 The Tekton Authors

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

package common

import (
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

// ComponentListerAdapter is a generic adapter that converts a typed lister
// to a ResourceLister. T must be a pointer type that implements ResourceWithName
// (e.g., *v1alpha1.TektonResult which has GetName() from embedded ObjectMeta).
type ComponentListerAdapter[T ResourceWithName] struct {
	Lister interface {
		List(selector labels.Selector) ([]T, error)
	}
}

func (a ComponentListerAdapter[T]) List(selector labels.Selector) ([]ResourceWithName, error) {
	items, err := a.Lister.List(selector)
	if err != nil {
		return nil, err
	}
	result := make([]ResourceWithName, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result, nil
}

// Type aliases for each Tekton component lister adapter.
// These provide convenient, typed constructors for each component.

type TektonResultListerAdapter = ComponentListerAdapter[*v1alpha1.TektonResult]
type TektonPipelineListerAdapter = ComponentListerAdapter[*v1alpha1.TektonPipeline]
type TektonTriggerListerAdapter = ComponentListerAdapter[*v1alpha1.TektonTrigger]
type TektonDashboardListerAdapter = ComponentListerAdapter[*v1alpha1.TektonDashboard]
type TektonHubListerAdapter = ComponentListerAdapter[*v1alpha1.TektonHub]
type TektonChainListerAdapter = ComponentListerAdapter[*v1alpha1.TektonChain]
