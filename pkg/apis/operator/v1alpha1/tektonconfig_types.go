/*
Copyright 2020 The Tekton Authors

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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	_ TektonComponent     = (*TektonConfig)(nil)
	_ TektonComponentSpec = (*TektonConfigSpec)(nil)
)

// TektonConfig is the Schema for the TektonConfigs API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonConfigSpec   `json:"spec,omitempty"`
	Status TektonConfigStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (tp *TektonConfig) GetSpec() TektonComponentSpec {
	return &tp.Spec
}

// GetStatus implements TektonComponent
func (tp *TektonConfig) GetStatus() TektonComponentStatus {
	return &tp.Status
}

// Prune defines the pruner
type Prune struct {
	// The resources which need to be pruned
	Resources []string `json:"resources,omitempty"`
	// The number of resource to keep
	// You dont want to delete all the pipelinerun/taskrun's by a cron
	Keep *uint `json:"keep,omitempty"`
	// How frequent pruning should happen
	Schedule string `json:"schedule,omitempty"`
}

func (p Prune) IsEmpty() bool {
	return reflect.DeepEqual(p, Prune{})
}

// TektonConfigSpec defines the desired state of TektonConfig
type TektonConfigSpec struct {
	Profile string `json:"profile,omitempty"`
	// Config holds the configuration for resources created by TektonConfig
	// +optional
	Config Config `json:"config,omitempty"`
	// Pruner holds the prune config
	// +optional
	Pruner     Prune `json:"pruner,omitempty"`
	CommonSpec `json:",inline"`
	// Addon holds the addons config
	// +optional
	Addon Addon `json:"addon,omitempty"`
	// Pipeline holds the customizable option for pipeline component
	// +optional
	Pipeline Pipeline `json:"pipeline,omitempty"`
}

// TektonConfigStatus defines the observed state of TektonConfig
type TektonConfigStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The url links of the manifests, separated by comma
	// +optional
	Manifests []string `json:"manifests,omitempty"`
}

// TektonConfigList contains a list of TektonConfig
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonConfig `json:"items"`
}

// Addon defines the field to customize Addon component
type Addon struct {
	// Params is the list of params passed for Addon customization
	// +optional
	Params []Param `json:"params,omitempty"`
}

func (a Addon) IsEmpty() bool {
	return reflect.DeepEqual(a, Addon{})
}

// Pipeline defines the field to customize Pipeline component
type Pipeline struct {
	PipelineProperties `json:",inline"`
}

type Config struct {
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
}
