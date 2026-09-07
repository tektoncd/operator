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

// The ShipwrightBuild desired-state fields (Shipwright and SharedResource) are
// copied from github.com/redhat-openshift-builds/operator (api/v1alpha1) so that
// this operator owns the definition instead of importing the external module.
// The surrounding object/spec/status follow this operator's component
// conventions (CommonSpec + duckv1.Status), so ShipwrightBuild is reconciled by a
// standard genreconciler controller like every other Tekton component.

var (
	_ TektonComponent     = (*ShipwrightBuild)(nil)
	_ TektonComponentSpec = (*ShipwrightBuildSpec)(nil)
)

// ShipwrightBuild is the Schema for the ShipwrightBuild API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.version`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`
type ShipwrightBuild struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ShipwrightBuildSpec   `json:"spec,omitempty"`
	Status ShipwrightBuildStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (sb *ShipwrightBuild) GetSpec() TektonComponentSpec {
	return &sb.Spec
}

// GetStatus implements TektonComponent
func (sb *ShipwrightBuild) GetStatus() TektonComponentStatus {
	return &sb.Status
}

// ShipwrightBuildSpec defines the desired state of Shipwright ShipwrightBuildConfig component
type ShipwrightBuildSpec struct {
	// ShipwrightBuildConfig defines the configurations for Shipwright Build component
	ShipwrightBuildConfig `json:",inline"`

	// CommonSpec holds common fields and functions on the Spec
	CommonSpec `json:",inline"`

	// Config holds the configuration for resources created by ShipwrightBuild
	// +optional
	Config Config `json:"config,omitempty"`

	// NetworkPolicy configures NetworkPolicy creation for the controller,
	// watcher and webhook workloads deployed by ShipwrightBuild.
	// +optional
	NetworkPolicy NetworkPolicyConfig `json:"networkPolicy,omitempty"`

	// options holds additions fields and these fields will be updated on the manifests
	// +optional
	Options AdditionalOptions `json:"options"`
}

// ShipwrightBuildConfig defines the configurations for Shared Resource component
type ShipwrightBuildConfig struct {
	// enable or disable ShipwrightBuild Component
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// ShipwrightBuildStatus defines the observed state of ShipwrightBuild
type ShipwrightBuildStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonResult
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`
}

// ShipwrightBuildList contains a list of ShipwrightBuild
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ShipwrightBuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShipwrightBuild `json:"items"`
}
