/*
Copyright 2021 The Tekton Authors

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
)

// Check that TriggerTemplate may be validated and defaulted.
var _ apis.Validatable = (*TriggerTemplate)(nil)
var _ apis.Defaultable = (*TriggerTemplate)(nil)

// TriggerTemplateSpec holds the desired state of TriggerTemplate
type TriggerTemplateSpec struct {
	// +listType=atomic
	Params []ParamSpec `json:"params,omitempty"`
	// +listType=atomic
	ResourceTemplates []TriggerResourceTemplate `json:"resourcetemplates,omitempty"`
}

// TriggerResourceTemplate describes a resource to create
type TriggerResourceTemplate struct {
	runtime.RawExtension `json:",inline"`
}

// TriggerTemplateStatus describes the desired state of TriggerTemplate
type TriggerTemplateStatus struct{}

// TriggerTemplate takes parameters and uses them to create CRDs
//
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
type TriggerTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec holds the desired state of the TriggerTemplate from the client
	// +optional
	Spec TriggerTemplateSpec `json:"spec"`
	// +optional
	Status TriggerTemplateStatus `json:"status,omitempty"`
}

// TriggerTemplateList contains a list of TriggerTemplate
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TriggerTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TriggerTemplate `json:"items"`
}
