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

type OpenShift struct {
	// PipelinesAsCode allows configuring PipelinesAsCode configurations
	// +optional
	PipelinesAsCode *PipelinesAsCode `json:"pipelinesAsCode,omitempty"`
	// SCC allows configuring security context constraints used by workloads
	// +optional
	SCC *SCC `json:"scc,omitempty"`
	// EnableCentralTLSConfig enables TLS configuration inheritance from
	// the cluster's APIServer TLS security profile. When enabled, TLS settings
	// (minimum version, cipher suites, curve preferences) are automatically
	// derived from the cluster-wide security policy and injected into Tekton
	// component containers that support TLS configuration.
	// If the APIServer does not have a TLS profile configured, user-specified
	// TLS settings in component configurations will be used as fallback.
	// Default: false (opt-in)
	// +optional
	EnableCentralTLSConfig bool `json:"enableCentralTLSConfig,omitempty"`
}

type PipelinesAsCode struct {
	// Enable or disable pipelines as code by changing this bool
	// +optional
	Enable *bool `json:"enable,omitempty"`
	// PACSettings allows user to configure PAC configurations
	// +optional
	PACSettings `json:",inline"`
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
