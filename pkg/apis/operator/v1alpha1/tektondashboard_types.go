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
	_ TektonComponent     = (*TektonDashboard)(nil)
	_ TektonComponentSpec = (*TektonDashboardSpec)(nil)
)

// TektonDashboard is the Schema for the tektondashboards API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonDashboard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonDashboardSpec   `json:"spec,omitempty"`
	Status TektonDashboardStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (tp *TektonDashboard) GetSpec() TektonComponentSpec {
	return &tp.Spec
}

// GetStatus implements TektonComponent
func (tp *TektonDashboard) GetStatus() TektonComponentStatus {
	return &tp.Status
}

// TektonDashboardSpec defines the desired state of TektonDashboard
type TektonDashboardSpec struct {
	CommonSpec `json:",inline"`
	// Config holds the configuration for resources created by TektonDashboard
	// +optional
	Config Config `json:"config,omitempty"`
}

// TektonDashboardStatus defines the observed state of TektonDashboard
type TektonDashboardStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The url links of the manifests, separated by comma
	// +optional
	Manifests []string `json:"manifests,omitempty"`
}

// TektonDashboardsList contains a list of TektonDashboard
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonDashboardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonDashboard `json:"items"`
}
