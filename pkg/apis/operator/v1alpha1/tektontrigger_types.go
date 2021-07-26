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
	_ TektonComponent     = (*TektonTrigger)(nil)
	_ TektonComponentSpec = (*TektonTriggerSpec)(nil)
)

// TektonTrigger is the Schema for the tektontriggers API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonTrigger struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonTriggerSpec   `json:"spec,omitempty"`
	Status TektonTriggerStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (tp *TektonTrigger) GetSpec() TektonComponentSpec {
	return &tp.Spec
}

// GetStatus implements TektonComponent
func (tp *TektonTrigger) GetStatus() TektonComponentStatus {
	return &tp.Status
}

// TektonTriggerSpec defines the desired state of TektonTrigger
type TektonTriggerSpec struct {
	CommonSpec `json:",inline"`
	// Config holds the configuration for resources created by TektonTrigger
	// +optional
	Config Config `json:"config,omitempty"`
}

// TektonTriggerStatus defines the observed state of TektonTrigger
type TektonTriggerStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The url links of the manifests, separated by comma
	// +optional
	Manifests []string `json:"manifests,omitempty"`
}

// TektonTriggersList contains a list of TektonTrigger
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonTriggerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonTrigger `json:"items"`
}
