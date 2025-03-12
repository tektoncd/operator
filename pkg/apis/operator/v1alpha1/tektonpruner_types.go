/*
Copyright 2024 The Tekton Authors

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
	_ TektonComponent     = (*TektonPruner)(nil)
	_ TektonComponentSpec = (*TektonPrunerSpec)(nil)
)

// TektonPruner is the Schema for the TektonPruner API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonPruner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonPrunerSpec   `json:"spec,omitempty"`
	Status TektonPrunerStatus `json:"status,omitempty"`
}

type TektonPrunerSpec struct {
	CommonSpec `json:",inline"`
	Pruner     `json:",inline"`
}

type Pruner struct {
	// options holds additions fields and these fields will be updated on the manifests
	Options AdditionalOptions `json:"options"`
}

// TektonPrunerList contains a list of TektonPruner
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonPrunerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonPruner `json:"items"`
}

// GetSpec implements TektonComponent
func (mag *TektonPruner) GetSpec() TektonComponentSpec {
	return &mag.Spec
}

// GetStatus implements TektonComponent
func (mag *TektonPruner) GetStatus() TektonComponentStatus {
	return &mag.Status
}

// TektonPrunerStatus defines the observed state of TektonPruner
type TektonPrunerStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonPruner
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`
}

func (t TektonPrunerStatus) MarkNotReady(s string) {
	//TODO implement me
	panic("implement me")
}

func (t TektonPrunerStatus) MarkInstallerSetReady() {
	//TODO implement me
	panic("implement me")
}

func (t TektonPrunerStatus) MarkInstallerSetNotReady(s string) {
	//TODO implement me
	panic("implement me")
}

func (t TektonPrunerStatus) MarkInstallerSetAvailable() {
	//TODO implement me
	panic("implement me")
}

func (t TektonPrunerStatus) MarkPreReconcilerFailed(s string) {
	//TODO implement me
	panic("implement me")
}

func (t TektonPrunerStatus) MarkPostReconcilerFailed(s string) {
	//TODO implement me
	panic("implement me")
}

func (t TektonPrunerStatus) GetVersion() string {
	//TODO implement me
	panic("implement me")
}

func (t TektonPrunerStatus) SetVersion(version string) {
	//TODO implement me
	panic("implement me")
}

func (t TektonPrunerStatus) IsReady() bool {
	//TODO implement me
	panic("implement me")
}
