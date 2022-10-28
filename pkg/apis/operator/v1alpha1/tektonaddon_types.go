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

// TektonAddon is the Schema for the tektonaddons API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonAddon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonAddonSpec   `json:"spec,omitempty"`
	Status TektonAddonStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (tp *TektonAddon) GetSpec() TektonComponentSpec {
	return &tp.Spec
}

// GetStatus implements TektonComponent
func (tp *TektonAddon) GetStatus() TektonComponentStatus {
	return &tp.Status
}

// TektonAddonSpec defines the desired state of TektonAddon
type TektonAddonSpec struct {
	CommonSpec `json:",inline"`
	Addon      `json:",inline"`
	// Config holds the configuration for resources created by Addon
	// +optional
	Config Config `json:"config,omitempty"`
}

// TektonAddonStatus defines the observed state of TektonAddon
type TektonAddonStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// TektonInstallerSet created to install addons
	// +optional
	AddonsInstallerSet map[string]string `json:"installerSets,omitempty"`
}

func (in *TektonAddonStatus) MarkInstallerSetAvailable() {
	//TODO implement me
	panic("implement me")
}

// Addon defines the field to customize Addon component
type Addon struct {
	// Params is the list of params passed for Addon customization
	// +optional
	Params []Param `json:"params,omitempty"`
	// Deprecated, will be removed in further release
	// EnablePAC field defines whether to install PAC
	// +optional
	EnablePAC *bool `json:"enablePipelinesAsCode,omitempty"`
}

func (a Addon) IsEmpty() bool {
	return len(a.Params) == 0
}

// TektonAddonsList contains a list of TektonAddon
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonAddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonAddon `json:"items"`
}
