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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	_ TektonComponent     = (*TektonKueue)(nil)
	_ TektonComponentSpec = (*TektonKueueSpec)(nil)
)

// TektonKueue is the Schema for the TektonKueue API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster

type TektonKueue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TektonKueueSpec   `json:"spec,omitempty"`
	Status            TektonKueueStatus `json:"status,omitempty"`
}

type Kueue struct {
	// enable or disable TektonKueue Component
	Disabled *bool `json:"disabled"`

	// options holds additions fields and these fields will be updated on the manifests
	Options AdditionalOptions `json:"options"`
}

// TektonKueueList contains a list of TektonKueue
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonKueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonKueue `json:"items"`
}

type TektonKueueSpec struct {
	CommonSpec `json:",inline"`
	Kueue      `json:",inline"`
	// Config holds the configuration for resources created by TektonKueue
	// +optional
	Config Config `json:"config,omitempty"`
}

// TektonKueueStatus defines the observed state of TektonKueue
type TektonKueueStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonKueue
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`
}

// GetSpec implements TektonComponent
func (tp *TektonKueue) GetSpec() TektonComponentSpec {
	return &tp.Spec
}

func (tp *TektonKueue) GetStatus() TektonComponentStatus {
	return &tp.Status
}

// IsDisabled returns true if the TektonKueue is disabled
func (p *Kueue) IsDisabled() bool {
	if p == nil || p.Disabled == nil {
		// When the Kueue is nil or Disabled is nil, we assume it is the default state.
		return DefaultKueueDisabled
	}
	return *p.Disabled
}
