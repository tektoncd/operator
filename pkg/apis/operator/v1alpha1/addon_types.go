package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TektonAddonSpec defines the desired state of TektonAddon
// +k8s:openapi-gen=true
type TektonAddonSpec struct {
	Version string `json:"version"`
}

// TektonAddonStatus defines the observed state of TektonAddon
// +k8s:openapi-gen=true
type TektonAddonStatus struct {
	// installation status sorted in reverse chronological order
	Conditions []TektonAddonCondition `json:"conditions,omitempty"`
}

// TektonAddonCondition defines the observed state of installation at a point in time
// +k8s:openapi-gen=true
type TektonAddonCondition struct {
	// Code indicates the status of installation of tektonaddon resources
	// Valid values are:
	//   - "error"
	//   - "installing"
	//   - "installed"
	Code InstallStatus `json:"code"`

	// Additional details about the Code
	Details string `json:"details,omitempty"`

	// The version of installed addon
	Version string `json:"version"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Addon is the Schema for the tektonaddons API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type TektonAddon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonAddonSpec   `json:"spec,omitempty"`
	Status TektonAddonStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TektonAddonList contains a list of Addon
type TektonAddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonAddon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TektonAddon{}, &TektonAddonList{})
}
