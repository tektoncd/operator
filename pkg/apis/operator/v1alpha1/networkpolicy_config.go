/*
Copyright 2024 The Tekton Authors

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
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"

	networkingv1 "k8s.io/api/networking/v1"
)

// NetworkPolicyConfig configures NetworkPolicy creation for a Tekton component.
type NetworkPolicyConfig struct {
	// Disabled disables all NetworkPolicy creation for this component.
	// Existing policies are removed on the next reconcile.
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	// Policies merges with the operator's default NetworkPolicies by name.
	// A key matching a default policy name replaces that default entirely.
	// A key not matching any default is added alongside the defaults.
	// If nil or empty, all operator defaults are applied unchanged.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Policies map[string]networkingv1.NetworkPolicySpec `json:"policies,omitempty"`
}

func (c NetworkPolicyConfig) validate(path string) (errs *apis.FieldError) {
	for name := range c.Policies {
		if msgs := validation.IsDNS1123Subdomain(name); len(msgs) > 0 {
			errs = errs.Also(apis.ErrInvalidKeyName(name, path+".policies", msgs...))
		}
	}
	return errs
}
