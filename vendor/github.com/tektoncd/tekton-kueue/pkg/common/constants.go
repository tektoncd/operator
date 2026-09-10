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

// Package common provides shared constants and utilities used across the
// controller, webhook, and mutate packages.
package common

const (
	// ManagedByMultiKueueLabel is set on PipelineRuns when multiKueue is enabled,
	// telling Kueue to dispatch the workload to a remote worker cluster.
	ManagedByMultiKueueLabel = "kueue.x-k8s.io/multikueue"

	// QueueLabel is the standard Kueue label used to assign workloads to a LocalQueue.
	QueueLabel = "kueue.x-k8s.io/queue-name"

	// ConfigKey is the key within the tekton-kueue-config ConfigMap that holds
	// the YAML configuration.
	ConfigKey = "config.yaml"

	// ConfigMapName is the name of the ConfigMap that configures the webhook.
	ConfigMapName = "tekton-kueue-config"
)
