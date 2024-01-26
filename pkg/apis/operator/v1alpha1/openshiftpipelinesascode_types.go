/*
Copyright 2022 The Tekton Authors

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

// OpenShiftPipelinesAsCode is the Schema for the OpenShiftPipelinesAsCode API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type OpenShiftPipelinesAsCode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenShiftPipelinesAsCodeSpec   `json:"spec,omitempty"`
	Status OpenShiftPipelinesAsCodeStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (pac *OpenShiftPipelinesAsCode) GetSpec() TektonComponentSpec {
	return &pac.Spec
}

// GetStatus implements TektonComponent
func (pac *OpenShiftPipelinesAsCode) GetStatus() TektonComponentStatus {
	return &pac.Status
}

// OpenShiftPipelinesAsCodeSpec defines the desired state of OpenShiftPipelinesAsCode
type OpenShiftPipelinesAsCodeSpec struct {
	CommonSpec  `json:",inline"`
	Config      Config `json:"config,omitempty"`
	PACSettings `json:",inline"`
}

// OpenShiftPipelinesAsCodeStatus defines the observed state of OpenShiftPipelinesAsCode
type OpenShiftPipelinesAsCodeStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`
}

// OpenShiftPipelinesAsCodeList contains a list of OpenShiftPipelinesAsCode
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type OpenShiftPipelinesAsCodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenShiftPipelinesAsCode `json:"items"`
}

type PACSettings struct {
	Settings map[string]string `json:"settings,omitempty"`
	// AdditionalPACControllers allows to deploy additional PAC controller
	// +optional
	AdditionalPACControllers map[string]AdditionalPACControllerConfig `json:"additionalPACControllers,omitempty"`
	// options holds additions fields and these fields will be updated on the manifests
	Options AdditionalOptions `json:"options"`
}

// AdditionalPACControllerConfig contains config for additionalPACControllers
type AdditionalPACControllerConfig struct {
	// Enable or disable this additional pipelines as code instance by changing this bool
	// +optional
	Enable *bool `json:"enable,omitempty"`
	// Name of the additional controller configMap
	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`
	// Name of the additional controller Secret
	// +optional
	SecretName string `json:"secretName,omitempty"`
	// Setting will contains the configMap data
	// +optional
	Settings map[string]string `json:"settings,omitempty"`
}
