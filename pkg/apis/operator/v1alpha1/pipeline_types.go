package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TektonPipelineSpec defines the desired state of TektonPipeline
// +k8s:openapi-gen=true
type TektonPipelineSpec struct {
	// namespace where pipelines will be installed
	TargetNamespace string `json:"targetNamespace"`
}

// TektonPipelineStatus defines the observed state of TektonPipeline
// +k8s:openapi-gen=true
type TektonPipelineStatus struct {

	// installation status sorted in reverse chronological order
	Conditions []TektonPipelineCondition `json:"conditions,omitempty"`
}

// TektonPipelineCondition defines the observed state of installation at a point in time
// +k8s:openapi-gen=true
type TektonPipelineCondition struct {
	// Code indicates the status of installation of pipeline resources
	// Valid values are:
	//   - "error"
	//   - "installing"
	//   - "installed"
	Code InstallStatus `json:"code"`

	// Additional details about the Code
	Details string `json:"details,omitempty"`

	// The version of pipelines
	Version string `json:"version"`
}

// InstallStatus describes the state of installation of pipelines
// +kubebuilder:validation:Enum=Allow;Forbid;Replace
type InstallStatus string

const (
	// InstalledStatus indicates that the pipeline resources are installed
	InstalledStatus InstallStatus = "installed"

	// InstallingStatus indicates that the pipeline resources are being installed
	InstallingStatus InstallStatus = "installing"

	// ErrorStatus indicates that there was an error installing pipeline resources
	// Check details field for additional details
	ErrorStatus InstallStatus = "error"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TektonPipeline is the Schema for the tektonpipelines API
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=tektonpipeline
// +kubebuilder:subresource:status
type TektonPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonPipelineSpec   `json:"spec,omitempty"`
	Status TektonPipelineStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TektonPipelineList contains a list of TektonPipeline
type TektonPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonPipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TektonPipeline{}, &TektonPipelineList{})
}
