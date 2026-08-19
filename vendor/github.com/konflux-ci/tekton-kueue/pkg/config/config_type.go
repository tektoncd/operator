// +k8s:deepcopy-gen=package
package config

/*
Copyright 2025.

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

// Config defines the webhook behavior, loaded from the tekton-kueue-config
// ConfigMap under the "config.yaml" key.
type Config struct {
	// QueueName is the Kueue LocalQueue that PipelineRuns are assigned to.
	// This is set as the "kueue.x-k8s.io/queue-name" label on each PipelineRun.
	QueueName string `json:"queueName,omitempty"`

	// MultiKueueOverride, when true, sets the PipelineRun's managedBy field
	// to "kueue.x-k8s.io/multikueue", enabling Kueue to dispatch the
	// PipelineRun to a remote worker cluster.
	MultiKueueOverride bool `json:"multiKueueOverride,omitempty"`

	// CEL contains optional CEL expressions for dynamic PipelineRun mutation.
	CEL CEL `json:"cel,omitempty"`
}

// CEL holds a list of CEL expressions that are evaluated against each
// PipelineRun during webhook admission. Expressions can set annotations,
// labels, or resource requests based on PipelineRun properties.
// See the internal/cel package for available functions and variables.
type CEL struct {
	Expressions []string `json:"expressions,omitempty"`
}
