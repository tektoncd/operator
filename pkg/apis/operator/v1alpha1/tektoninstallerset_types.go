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

package v1alpha1

import (
	mf "github.com/manifestival/manifestival"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TektonInstallerSet is the Schema for the TektonInstallerSet API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonInstallerSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonInstallerSetSpec   `json:"spec,omitempty"`
	Status TektonInstallerSetStatus `json:"status,omitempty"`
}

// TektonInstallerSetSpec defines the desired state of TektonInstallerSet
type TektonInstallerSetSpec struct {
	Manifests mf.Slice `json:"manifests,omitempty"`
}

// TektonInstallerSetStatus defines the observed state of TektonInstallerSet
type TektonInstallerSetStatus struct {
	duckv1.Status `json:",inline"`
}

// TektonInstallerSetList contains a list of TektonInstallerSet
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonInstallerSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonInstallerSet `json:"items"`
}
