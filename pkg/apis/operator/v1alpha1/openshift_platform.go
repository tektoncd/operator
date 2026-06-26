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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type OpenShift struct {
	// PipelinesAsCode allows configuring PipelinesAsCode configurations
	// +optional
	PipelinesAsCode *PipelinesAsCode `json:"pipelinesAsCode,omitempty"`
	// SCC allows configuring security context constraints used by workloads
	// +optional
	SCC *SCC `json:"scc,omitempty"`
	// EnableCentralTLSConfig controls TLS configuration inheritance from the
	// cluster's APIServer TLS security profile. When enabled (the default),
	// TLS settings (minimum version, cipher suites, curve preferences) are
	// automatically derived from the cluster-wide security policy and injected
	// into Tekton component containers that support TLS configuration.
	// Set to false to opt out and manage TLS settings manually.
	// Default: true (opt-out)
	// +optional
	EnableCentralTLSConfig *bool `json:"enableCentralTLSConfig,omitempty"`
	// NamespaceSync controls what the Tekton Operator synchronises into each
	// user namespace on OpenShift (pipeline SA, CA bundles, edit RoleBinding,
	// and registry secret bindings).
	// +optional
	NamespaceSync *NamespaceSyncConfig `json:"namespaceSync,omitempty"`
}

// NamespaceSyncConfig configures the NamespaceSyncController which watches
// user namespaces and ensures Tekton-required resources are present and up to date.
// All boolean fields default to true when the NamespaceSync block is present.
type NamespaceSyncConfig struct {
	// CreatePipelineSA controls whether the pipeline ServiceAccount is created
	// in each namespace. Disable only if you manage the pipeline SA externally.
	// Replaces the legacy spec.params entry createRbacResource.
	// Default: true
	// +optional
	CreatePipelineSA *bool `json:"createPipelineSA,omitempty"`

	// CreateCABundles controls whether the CA bundle ConfigMaps
	// (config-trusted-cabundle, config-service-cabundle) are injected into
	// each namespace for TLS trust.
	// Replaces the legacy spec.params entry createCABundleConfigMaps.
	// Default: true
	// +optional
	CreateCABundles *bool `json:"createCABundles,omitempty"`

	// CreateEditRoleBinding controls whether a RoleBinding named
	// openshift-pipelines-edit is created in each namespace, binding the
	// pipeline SA to the built-in edit ClusterRole. Set to false for
	// least-privilege environments where PipelineRuns should not have
	// broad write permissions in their namespace.
	// Replaces the legacy spec.params entry legacyPipelineRbac.
	// Default: true
	// +optional
	CreateEditRoleBinding *bool `json:"createEditRoleBinding,omitempty"`

	// CreateSCCRoleBinding controls whether the pipelines-scc-rolebinding
	// RoleBinding (and, when a namespace-level SCC is requested via the
	// operator.tekton.dev/scc annotation, the pipelines-scc-role Role) is
	// managed in each namespace. When enabled, the pipeline SA is granted
	// permission to use the cluster-wide default SCC (pipelines-scc by
	// default) or a namespace-specific SCC when the annotation is present.
	// Default: true
	// +optional
	CreateSCCRoleBinding *bool `json:"createSCCRoleBinding,omitempty"`

	// SecretBindings declares secrets that should be automatically bound to
	// the pipeline SA in every namespace. When a secret matching a binding
	// appears in a namespace it is added to both imagePullSecrets and secrets
	// on the pipeline SA. When the secret is deleted the reference is removed.
	// Each entry must set exactly one of labelSelector or secretName.
	// +optional
	SecretBindings []SecretBinding `json:"secretBindings,omitempty"`
}

// SecretBinding describes a secret (or class of secrets by label) that the
// NamespaceSyncController should bind to the pipeline SA in every namespace.
// Exactly one of LabelSelector or SecretName must be set.
type SecretBinding struct {
	// LabelSelector selects secrets by label. All secrets matching this
	// selector in a given namespace are bound to the pipeline SA.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// SecretName binds a specific named secret in each namespace to the
	// pipeline SA. The secret is bound when it exists and unbound when deleted.
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

type SCC struct {
	// Default contains the default SCC that will be attached to the service
	// account used for workloads (`pipeline` SA by default) and defined in
	// PipelineProperties.OptionalPipelineProperties.DefaultServiceAccount
	// +optional
	Default string `json:"default,omitempty"`
	// MaxAllowed specifies the highest SCC that can be requested for in a
	// namespace or in the Default field.
	// +optional
	MaxAllowed string `json:"maxAllowed,omitempty"`
}
