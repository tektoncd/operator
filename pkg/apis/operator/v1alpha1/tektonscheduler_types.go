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

// TektonScheduler is the Schema for the TektonScheduler API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster

type TektonScheduler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TektonSchedulerSpec   `json:"spec,omitempty"`
	Status            TektonSchedulerStatus `json:"status,omitempty"`
}

// Scheduler Configuration  to manage Scheduler Configuration.
type Scheduler struct {
	// enable or disable TektonScheduler Component
	Disabled           *bool `json:"disabled"`
	SchedulerConfig    `json:",inline"`
	MultiClusterConfig `json:",inline"`
	// options holds additions fields and these fields will be updated on the manifests
	Options AdditionalOptions `json:"options"`
}

type SchedulerConfig struct {
	// This hold the config data from tekton-kueue. ConfigMap in tekton kueue is loaded as config.yaml so we need to
	//match the key here
	config.Config `json:"config.yaml"`
}

// MultiClusterConfig Configuration to enable/disable MultiCluster Configuration
type MultiClusterConfig struct {
	MultiClusterDisabled bool             `json:"multi-cluster-disabled"`
	MultiClusterRole     MultiClusterRole `json:"multi-cluster-role"`
}

// MultiClusterRole Define the role of current cluster in multi-cluster environment. The MultiClusterRole
// can be one of Hub or Spoke
type MultiClusterRole string

// TektonSchedulerList contains a list of TektonScheduler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonSchedulerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonScheduler `json:"items"`
}

type TektonSchedulerSpec struct {
	CommonSpec `json:",inline"`
	Scheduler  `json:",inline"`
}

// TektonSchedulerStatus defines the observed state of TektonScheduler
type TektonSchedulerStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonScheduler
	// +optional
	TektonScheduler string `json:"tekton-scheduler,omitempty"`
}

// GetSpec implements TektonComponent
func (tp *TektonScheduler) GetSpec() TektonComponentSpec {
	return &tp.Spec
}

func (tp *TektonScheduler) GetStatus() TektonComponentStatus {
	return &tp.Status
}

// IsDisabled returns true if the TektonScheduler is disabled
func (p *Scheduler) IsDisabled() bool {
	if p == nil || p.Disabled == nil {
		// When the Scheduler is nil or Disabled is nil, we assume it is the default state.
		return DefaultSchedulerDisabled
	}
	return *p.Disabled
}
