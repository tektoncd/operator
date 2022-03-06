/*
Copyright 2022 The Tekton Authors

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
	_ TektonComponent     = (*TektonChain)(nil)
	_ TektonComponentSpec = (*TektonChainSpec)(nil)
)

// TektonChain is the Schema for the tektonchain API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonChain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonChainSpec   `json:"spec,omitempty"`
	Status TektonChainStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (tc *TektonChain) GetSpec() TektonComponentSpec {
	return &tc.Spec
}

// GetStatus implements TektonComponent
func (tc *TektonChain) GetStatus() TektonComponentStatus {
	return &tc.Status
}

// TektonChainSpec defines the desired state of TektonChain
type TektonChainSpec struct {
	CommonSpec `json:",inline"`
	// Config holds the configuration for resources created by TektonChain
	// +optional
	Config Config `json:"config,omitempty"`
}

// TektonChainStatus defines the observed state of TektonChain
type TektonChainStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonChain
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`
}

// TektonChainList contains a list of TektonChain
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonChainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonChain `json:"items"`
}
