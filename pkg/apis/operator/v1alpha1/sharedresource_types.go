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

// The SharedResource desired-state fields (Shipwright and SharedResource) are
// copied from github.com/redhat-openshift-builds/operator (api/v1alpha1) so that
// this operator owns the definition instead of importing the external module.
// The surrounding object/spec/status follow this operator's component
// conventions (CommonSpec + duckv1.Status), so SharedResource is reconciled by a
// standard genreconciler controller like every other Tekton component.

var (
	_ TektonComponent     = (*SharedResource)(nil)
	_ TektonComponentSpec = (*SharedResourceSpec)(nil)
)

// SharedResource is the Schema for the SharedResource API
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
type SharedResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SharedResourceSpec   `json:"spec,omitempty"`
	Status SharedResourceStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (sr *SharedResource) GetSpec() TektonComponentSpec {
	return &sr.Spec
}

// GetStatus implements TektonComponent
func (sr *SharedResource) GetStatus() TektonComponentStatus {
	return &sr.Status
}

// SharedResourceSpec defines the desired state of Shared Resource component
type SharedResourceSpec struct {
	// SharedResourceConfig defines the configurations for Shared Resource component
	SharedResourceConfig `json:",inline"`

	// CommonSpec holds common fields and functions on the Spec
	CommonSpec `json:",inline"`

	// Config holds the configuration for resources created by SharedResource
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

// SharedResourceConfig defines the configurations for Shared Resource component
type SharedResourceConfig struct {
	// enable or disable SharedResource Component
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// SharedResourceStatus defines the observed state of SharedResource.
type SharedResourceStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`


	// The current installer set name for TektonResult
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`
}

// SharedResourceList contains a list of SharedResource
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SharedResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SharedResource `json:"items"`
}
