/*
Copyright 2019 The Tekton Authors

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

package v1alpha1

import (
	"context"
)

// SetDefaults sets the defaults on the object.
func (el *EventListener) SetDefaults(ctx context.Context) {
	if IsUpgradeViaDefaulting(ctx) {
		// set defaults
		if el.Spec.Replicas != nil && *el.Spec.Replicas == 0 {
			*el.Spec.Replicas = 1
		}
		for i := range el.Spec.Triggers {
			triggerSpecBindingArray(el.Spec.Triggers[i].Bindings).
				defaultBindings()
			if el.Spec.Triggers[i].Template != nil {
				templateNameToRef(el.Spec.Triggers[i].Template)
			}
		}
		// Remove Deprecated Resource Fields
		// To be removed in a later release #904
		el.Spec.updatePodTemplate()
		el.Spec.updateServiceType()
	}
}

// To be Removed in a later release #904
func (spec *EventListenerSpec) updatePodTemplate() {
	if spec.DeprecatedPodTemplate.NodeSelector != nil {
		if spec.Resources.KubernetesResource == nil {
			spec.Resources.KubernetesResource = &KubernetesResource{}
		}
		spec.Resources.KubernetesResource.Template.Spec.NodeSelector = spec.DeprecatedPodTemplate.NodeSelector
		spec.DeprecatedPodTemplate.NodeSelector = nil
	}
	if spec.DeprecatedPodTemplate.Tolerations != nil {
		if spec.Resources.KubernetesResource == nil {
			spec.Resources.KubernetesResource = &KubernetesResource{}
		}
		spec.Resources.KubernetesResource.Template.Spec.Tolerations = spec.DeprecatedPodTemplate.Tolerations
		spec.DeprecatedPodTemplate.Tolerations = nil
	}
}

// To be Removed in a later release #904
func (spec *EventListenerSpec) updateServiceType() {
	if spec.DeprecatedServiceType != "" {
		if spec.Resources.KubernetesResource == nil {
			spec.Resources.KubernetesResource = &KubernetesResource{}
		}
		spec.Resources.KubernetesResource.ServiceType = spec.DeprecatedServiceType
		spec.DeprecatedServiceType = ""
	}
}
