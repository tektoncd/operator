package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddonSpec defines the desired state of Addon
// +k8s:openapi-gen=true
type AddonSpec struct {
	Version string `json:"version"`
}

// AddonStatus defines the observed state of Addon
// +k8s:openapi-gen=true
type AddonStatus struct {
	// installation status sorted in reverse chronological order
	Conditions []AddonCondition `json:"conditions,omitempty"`
}

// AddonCondition defines the observed state of installation at a point in time
// +k8s:openapi-gen=true
type AddonCondition struct {
	// Code indicates the status of installation of addon resources
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

// Addon is the Schema for the addons API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Addon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonSpec   `json:"spec,omitempty"`
	Status AddonStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AddonList contains a list of Addon
type AddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Addon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Addon{}, &AddonList{})
}
