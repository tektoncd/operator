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

var (
	_ TektonComponent     = (*SyncerService)(nil)
	_ TektonComponentSpec = (*SyncerServiceSpec)(nil)
)

// SyncerService is the Schema for the syncerservices API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type SyncerService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncerServiceSpec   `json:"spec,omitempty"`
	Status SyncerServiceStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (ss *SyncerService) GetSpec() TektonComponentSpec {
	return &ss.Spec
}

// GetStatus implements TektonComponent
func (ss *SyncerService) GetStatus() TektonComponentStatus {
	return &ss.Status
}

// SyncerServiceSpec defines the desired state of SyncerService
type SyncerServiceSpec struct {
	CommonSpec           `json:",inline"`
	SyncerServiceOptions `json:",inline"`
	// Config holds the configuration for resources created by SyncerService
	// +optional
	Config Config `json:"config,omitempty"`
}

// SyncerServiceOptions defines the fields to customize SyncerService component
type SyncerServiceOptions struct {
	// Options holds additions fields and these fields will be updated on the manifests
	Options AdditionalOptions `json:"options,omitempty"`
}

// SyncerServiceStatus defines the observed state of SyncerService
type SyncerServiceStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for SyncerService
	// +optional
	SyncerServiceInstallerSet string `json:"syncerServiceInstallerSet,omitempty"`
}

func (sss *SyncerServiceStatus) MarkPreReconcilerFailed(msg string) {
	sss.MarkNotReady("PreReconciliation failed")
	syncerServiceCondSet.Manage(sss).MarkFalse(
		PreReconciler,
		"Error",
		msg)
}

func (sss *SyncerServiceStatus) MarkPostReconcilerFailed(msg string) {
	sss.MarkNotReady("PostReconciliation failed")
	syncerServiceCondSet.Manage(sss).MarkFalse(
		PostReconciler,
		"Error",
		msg)
}

// SyncerServiceList contains a list of SyncerService
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SyncerServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncerService `json:"items"`
}
