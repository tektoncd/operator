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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	_ TektonComponent     = (*ManualApprovalGate)(nil)
	_ TektonComponentSpec = (*ManualApprovalGateSpec)(nil)
)

// ManualApprovalGate is the Schema for the ManualApprovalGate API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type ManualApprovalGate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManualApprovalGateSpec   `json:"spec,omitempty"`
	Status ManualApprovalGateStatus `json:"status,omitempty"`
}

type ManualApprovalGateSpec struct {
	CommonSpec     `json:",inline"`
	ManualApproval `json:",inline"`
}

type ManualApproval struct {
	// options holds additions fields and these fields will be updated on the manifests
	Options AdditionalOptions `json:"options"`
}

// ManualApprovalGateList contains a list of ManualApprovalGate
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ManualApprovalGateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManualApprovalGate `json:"items"`
}

// GetSpec implements TektonComponent
func (mag *ManualApprovalGate) GetSpec() TektonComponentSpec {
	return &mag.Spec
}

// GetStatus implements TektonComponent
func (mag *ManualApprovalGate) GetStatus() TektonComponentStatus {
	return &mag.Status
}

// ManualApprovalGateStatus defines the observed state of ManualApprovalGate
type ManualApprovalGateStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for ManualApprovalGate
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`
}
