/*
Copyright 2021 The Tekton Authors

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

package v1beta1

import (
	"context"

	"github.com/tektoncd/triggers/pkg/apis/config"
	"github.com/tektoncd/triggers/pkg/apis/triggers/contexts"
)

// SetDefaults sets the defaults on the object.
func (el *EventListener) SetDefaults(ctx context.Context) {
	cfg := config.FromContextOrDefaults(ctx)
	if contexts.IsUpgradeViaDefaulting(ctx) {
		defaultSA := cfg.Defaults.DefaultServiceAccount
		// set defaults
		if el.Spec.ServiceAccountName == "" && defaultSA != "" {
			el.Spec.ServiceAccountName = defaultSA
		}

		if el.Spec.Resources.KubernetesResource != nil {
			if el.Spec.Resources.KubernetesResource.Replicas != nil && *el.Spec.Resources.KubernetesResource.Replicas == 0 {
				*el.Spec.Resources.KubernetesResource.Replicas = 1
			}
		}

		for i, t := range el.Spec.Triggers {
			triggerSpecBindingArray(el.Spec.Triggers[i].Bindings).defaultBindings()
			for _, ti := range t.Interceptors {
				if ti != nil {
					ti.defaultInterceptorKind()
				}
			}
		}

		for _, tg := range el.Spec.TriggerGroups {
			for _, ti := range tg.Interceptors {
				if ti != nil {
					ti.defaultInterceptorKind()
				}
			}
		}
	}
}
