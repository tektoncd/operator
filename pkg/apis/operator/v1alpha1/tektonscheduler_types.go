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
	"github.com/konflux-ci/tekton-kueue/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	_ TektonComponent     = (*TektonScheduler)(nil)
	_ TektonComponentSpec = (*TektonSchedulerSpec)(nil)
)

// TektonScheduler is the deprecated predecessor of TektonKueue. It is retained
// temporarily so existing resources remain readable while pre-upgrade migration
// copies their configuration to TektonKueue.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.version`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`
type TektonScheduler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TektonSchedulerSpec   `json:"spec,omitempty"`
	Status            TektonSchedulerStatus `json:"status,omitempty"`
}

// Scheduler defines the deprecated TektonScheduler configuration.
type Scheduler struct {
	// enable or disable TektonScheduler Component
	Disabled           *bool `json:"disabled"`
	SchedulerConfig    `json:",inline"`
	MultiClusterConfig `json:",inline"`
	// options holds additions fields and these fields will be updated on the manifests
	// +optional
	Options AdditionalOptions `json:"options"`
}

// SchedulerConfig contains the deprecated TektonScheduler operand configuration.
type SchedulerConfig struct {
	// This holds the config data loaded by tekton-kueue as config.yaml.
	// +kubebuilder:pruning:PreserveUnknownFields
	config.Config `json:"config.yaml"`
}

// ToKueue converts deprecated scheduler configuration to its replacement.
func (s Scheduler) ToKueue() Kueue {
	return Kueue{
		Disabled: s.Disabled,
		KueueConfig: KueueConfig{
			Config: s.Config,
		},
		MultiClusterConfig: s.MultiClusterConfig,
		Options:            s.Options,
	}
}

// TektonSchedulerList contains a list of deprecated TektonScheduler resources.
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonSchedulerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonScheduler `json:"items"`
}

// TektonSchedulerSpec defines the desired state of the deprecated TektonScheduler API.
type TektonSchedulerSpec struct {
	CommonSpec `json:",inline"`
	Scheduler  `json:",inline"`
	// +optional
	NetworkPolicy NetworkPolicyConfig `json:"networkPolicy,omitempty"`
}

// TektonSchedulerStatus defines the observed state of the deprecated TektonScheduler API.
type TektonSchedulerStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonScheduler
	// +optional
	TektonScheduler string `json:"tekton-scheduler,omitempty"`
}

// GetSpec implements TektonComponent.
func (ts *TektonScheduler) GetSpec() TektonComponentSpec {
	return &ts.Spec
}

// GetStatus implements TektonComponent.
func (ts *TektonScheduler) GetStatus() TektonComponentStatus {
	return &ts.Status
}
