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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	_ TektonComponent     = (*TektonChains)(nil)
	_ TektonComponentSpec = (*TektonChainsSpec)(nil)
)

// TektonChains is the Schema for the tektonchains API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonChains struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonChainsSpec   `json:"spec,omitempty"`
	Status TektonChainsStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (tc *TektonChains) GetSpec() TektonComponentSpec {
	return &tc.Spec
}

// GetStatus implements TektonComponent
func (tc *TektonChains) GetStatus() TektonComponentStatus {
	return &tc.Status
}

// TektonChainsSpec defines the desired state of TektonChains
type TektonChainsSpec struct {
	CommonSpec `json:",inline"`
	// Config holds the configuration for resources created by TektonChains
	// +optional
	Config Config `json:"config,omitempty"`
}

// TektonChainsStatus defines the observed state of TektonChains
type TektonChainsStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonChains
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`
}

// TektonChainsList contains a list of TektonChains
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonChainsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonChains `json:"items"`
}
