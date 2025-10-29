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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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
	// options holds additions fields and these fields will be updated on the manifests
	Options AdditionalOptions `json:"options"`
}

// PipelineProperties defines customizable flags for Pipeline Component.
type PipelineProperties struct {
	DisableAffinityAssistant                 *bool  `json:"disable-affinity-assistant,omitempty"`
	DisableCredsInit                         *bool  `json:"disable-creds-init,omitempty"`
	AwaitSidecarReadiness                    *bool  `json:"await-sidecar-readiness,omitempty"`
	RunningInEnvironmentWithInjectedSidecars *bool  `json:"running-in-environment-with-injected-sidecars,omitempty"`
	RequireGitSshSecretKnownHosts            *bool  `json:"require-git-ssh-secret-known-hosts,omitempty"`
	EnableCustomTasks                        *bool  `json:"enable-custom-tasks,omitempty"`
	EnableApiFields                          string `json:"enable-api-fields,omitempty"`
	EmbeddedStatus                           string `json:"embedded-status,omitempty"`
	SendCloudEventsForRuns                   *bool  `json:"send-cloudevents-for-runs,omitempty"`
	// "verification-mode" is deprecated and never used.
	// This field will be removed, see https://github.com/tektoncd/operator/issues/1497
	// originally this field was removed in https://github.com/tektoncd/operator/pull/1481
	// there is no use with this field, just adding back to unblock the upgrade

	// not in use, see: https://github.com/tektoncd/pipeline/pull/7789
	// this field is removed from pipeline component
	// keeping here to maintain the API compatibility
	EnableTektonOciBundles *bool `json:"enable-tekton-oci-bundles,omitempty"`

	VerificationMode          string `json:"verification-mode,omitempty"`
	VerificationNoMatchPolicy string `json:"trusted-resources-verification-no-match-policy,omitempty"`
	EnableProvenanceInStatus  *bool  `json:"enable-provenance-in-status,omitempty"`

	// ScopeWhenExpressionsToTask is deprecated and never used.
	ScopeWhenExpressionsToTask *bool `json:"scope-when-expressions-to-task,omitempty"`

	EnforceNonfalsifiability  string `json:"enforce-nonfalsifiability,omitempty"`
	EnableKeepPodOnCancel     *bool  `json:"keep-pod-on-cancel,omitempty"`
	ResultExtractionMethod    string `json:"results-from,omitempty"`
	MaxResultSize             *int32 `json:"max-result-size,omitempty"`
	SetSecurityContext        *bool  `json:"set-security-context,omitempty"`
	Coschedule                string `json:"coschedule,omitempty"`
	EnableCELInWhenExpression *bool  `json:"enable-cel-in-whenexpression,omitempty"`
	EnableStepActions         *bool  `json:"enable-step-actions,omitempty"`
	EnableParamEnum           *bool  `json:"enable-param-enum,omitempty"`
	DisableInlineSpec         string `json:"disable-inline-spec,omitempty"`

	PipelineMetricsProperties `json:",inline"`
	// +optional
	TracingProperties `json:",inline"`
	// +optional
	OptionalPipelineProperties `json:",inline"`
	// +optional
	Resolvers `json:",inline"`
	// +optional
	Performance PerformanceProperties `json:"performance,omitempty"`
}

// OptionalPipelineProperties defines the fields which are to be
// defined for pipelines only if user pass them
type OptionalPipelineProperties struct {
	DefaultTimeoutMinutes               *uint  `json:"default-timeout-minutes,omitempty"`
	DefaultServiceAccount               string `json:"default-service-account,omitempty"`
	DefaultManagedByLabelValue          string `json:"default-managed-by-label-value,omitempty"`
	DefaultPodTemplate                  string `json:"default-pod-template,omitempty"`
	DefaultCloudEventsSink              string `json:"default-cloud-events-sink,omitempty"`
	DefaultAffinityAssistantPodTemplate string `json:"default-affinity-assistant-pod-template,omitempty"`
	DefaultTaskRunWorkspaceBinding      string `json:"default-task-run-workspace-binding,omitempty"`
	DefaultMaxMatrixCombinationsCount   string `json:"default-max-matrix-combinations-count,omitempty"`
	DefaultForbiddenEnv                 string `json:"default-forbidden-env,omitempty"`
	DefaultResolverType                 string `json:"default-resolver-type,omitempty"`
}

// WebhookOptions defines options for webhooks
type WebhookConfigurationOptions struct {
	FailurePolicy  *admissionregistrationv1.FailurePolicyType `json:"failurePolicy,omitempty"`
	TimeoutSeconds *int32                                     `json:"timeoutSeconds,omitempty"`
	SideEffects    *admissionregistrationv1.SideEffectClass   `json:"sideEffects,omitempty"`
}

// PipelineMetricsProperties defines the fields which are configurable for
// metrics
type PipelineMetricsProperties struct {
	MetricsTaskrunLevel            string `json:"metrics.taskrun.level,omitempty"`
	MetricsTaskrunDurationType     string `json:"metrics.taskrun.duration-type,omitempty"`
	MetricsPipelinerunLevel        string `json:"metrics.pipelinerun.level,omitempty"`
	MetricsPipelinerunDurationType string `json:"metrics.pipelinerun.duration-type,omitempty"`
	CountWithReason                *bool  `json:"metrics.count.enable-reason,omitempty"`
}

// TracingProperties defines the fields which are configurable for tracing
type TracingProperties struct {
	// Enabled controls whether tracing is enabled or not
	// +optional
	Enabled *bool `json:"traces.enabled,omitempty"`
	// Endpoint is the URL for the OpenTelemetry trace collector
	// +optional
	Endpoint string `json:"traces.endpoint,omitempty"`
	// CredentialsSecret is the name of the secret containing credentials for the tracing endpoint
	// +optional
	CredentialsSecret string `json:"traces.credentialsSecret,omitempty"`
}

// Resolvers defines the fields to configure resolvers
type Resolvers struct {
	EnableBundlesResolver *bool `json:"enable-bundles-resolver,omitempty"`
	EnableHubResolver     *bool `json:"enable-hub-resolver,omitempty"`
	EnableGitResolver     *bool `json:"enable-git-resolver,omitempty"`
	EnableClusterResolver *bool `json:"enable-cluster-resolver,omitempty"`
	ResolversConfig       `json:",inline"`
}

// ResolversConfig defines the fields to configure each of the resolver
type ResolversConfig struct {
	BundlesResolverConfig map[string]string `json:"bundles-resolver-config,omitempty"`
	HubResolverConfig     map[string]string `json:"hub-resolver-config,omitempty"`
	GitResolverConfig     map[string]string `json:"git-resolver-config,omitempty"`
	ClusterResolverConfig map[string]string `json:"cluster-resolver-config,omitempty"`
}
