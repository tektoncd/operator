/*
Copyright 2026 The Tekton Authors

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
	_ TektonComponent     = (*TektonMulticlusterProxyAAE)(nil)
	_ TektonComponentSpec = (*TektonMulticlusterProxyAAESpec)(nil)
)

// TektonMulticlusterProxyAAE is the Schema for the TektonMulticlusterProxyAAE API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

type TektonMulticlusterProxyAAE struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonMulticlusterProxyAAESpec   `json:"spec,omitempty"`
	Status TektonMulticlusterProxyAAEStatus `json:"status,omitempty"`
}

type TektonMulticlusterProxyAAESpec struct {
	CommonSpec                  `json:",inline"`
	MulticlusterProxyAAEOptions `json:",inline"`
}

type MulticlusterProxyAAEOptions struct {
	// options holds additional fields and these fields will be updated on the manifests
	Options AdditionalOptions `json:"options"`
}

// TektonMulticlusterProxyAAEList contains a list of TektonMulticlusterProxyAAE
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonMulticlusterProxyAAEList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonMulticlusterProxyAAE `json:"items"`
}

// TektonMulticlusterProxyAAEStatus defines the observed state of TektonMulticlusterProxyAAE
type TektonMulticlusterProxyAAEStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonMulticlusterProxyAAE
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`
}

// GetSpec implements TektonComponent
func (t *TektonMulticlusterProxyAAE) GetSpec() TektonComponentSpec {
	return &t.Spec
}

// GetStatus implements TektonComponent
func (t *TektonMulticlusterProxyAAE) GetStatus() TektonComponentStatus {
	return &t.Status
}
