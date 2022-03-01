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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TektonPipeline is the Schema for the tektonpipelines API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonPipelineSpec   `json:"spec,omitempty"`
	Status TektonPipelineStatus `json:"status,omitempty"`
}

func (tp *TektonPipeline) GetSpec() TektonComponentSpec {
	return &tp.Spec
}

func (tp *TektonPipeline) GetStatus() TektonComponentStatus {
	return &tp.Status
}

// TektonPipelineSpec defines the desired state of TektonPipeline
type TektonPipelineSpec struct {
	CommonSpec `json:",inline"`
	Pipeline   `json:",inline"`
	// Config holds the configuration for resources created by TektonPipeline
	// +optional
	Config Config `json:"config,omitempty"`
}

// TektonPipelineStatus defines the observed state of TektonPipeline
type TektonPipelineStatus struct {
	duckv1.Status `json:",inline"`
	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonPipeline
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`

	// The installer sets created for extension components
	// +optional
	ExtentionInstallerSets map[string]string `json:"extTektonInstallerSets,omitempty"`
}

// TektonPipelineList contains a list of TektonPipeline
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonPipeline `json:"items"`
}

// Pipeline defines the field to customize Pipeline component
type Pipeline struct {
	PipelineProperties `json:",inline"`
	// The params to customize different components of Pipelines
	// +optional
	Params []Param `json:"params,omitempty"`
}

// PipelineProperties defines customizable flags for Pipeline Component.
type PipelineProperties struct {
	DisableAffinityAssistant *bool `json:"disable-affinity-assistant,omitempty"`

	// DEPRECATED:  (Removed in Pipelines v0.33.0): to be removed in next release
	DisableHomeEnvOverwrite *bool `json:"disable-home-env-overwrite,omitempty"`
	// DEPRECATED: (Removed in Pipelines v0.33.0): to be removed in next release
	DisableWorkingDirectoryOverwrite *bool `json:"disable-working-directory-overwrite,omitempty"`

	DisableCredsInit                         *bool  `json:"disable-creds-init,omitempty"`
	RunningInEnvironmentWithInjectedSidecars *bool  `json:"running-in-environment-with-injected-sidecars,omitempty"`
	RequireGitSshSecretKnownHosts            *bool  `json:"require-git-ssh-secret-known-hosts,omitempty"`
	EnableTektonOciBundles                   *bool  `json:"enable-tekton-oci-bundles,omitempty"`
	EnableCustomTasks                        *bool  `json:"enable-custom-tasks,omitempty"`
	EnableApiFields                          string `json:"enable-api-fields,omitempty"`
	ScopeWhenExpressionsToTask               *bool  `json:"scope-when-expressions-to-task,omitempty"`
	PipelineMetricsProperties                `json:",inline"`
	// +optional
	OptionalPipelineProperties `json:",inline"`
}

// OptionalPipelineProperties defines the fields which are to be
// defined for pipelines only if user pass them
type OptionalPipelineProperties struct {
	DefaultTimeoutMinutes          *uint  `json:"default-timeout-minutes,omitempty"`
	DefaultServiceAccount          string `json:"default-service-account,omitempty"`
	DefaultManagedByLabelValue     string `json:"default-managed-by-label-value,omitempty"`
	DefaultPodTemplate             string `json:"default-pod-template,omitempty"`
	DefaultCloudEventsSink         string `json:"default-cloud-events-sink,omitempty"`
	DefaultTaskRunWorkspaceBinding string `json:"default-task-run-workspace-binding,omitempty"`
}

// PipelineMetricsProperties defines the fields which are configurable for
// metrics
type PipelineMetricsProperties struct {
	MetricsTaskrunLevel            string `json:"metrics.taskrun.level,omitempty"`
	MetricsTaskrunDurationType     string `json:"metrics.taskrun.duration-type,omitempty"`
	MetricsPipelinerunLevel        string `json:"metrics.pipelinerun.level,omitempty"`
	MetricsPipelinerunDurationType string `json:"metrics.pipelinerun.duration-type,omitempty"`
}
