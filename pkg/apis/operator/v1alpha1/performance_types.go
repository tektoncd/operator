/*
Copyright 2025 The Tekton Authors

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

// PerformanceProperties defines the fields which are configurable
// to tune the performance of component controller
type PerformanceProperties struct {
	// +optional
	PerformanceLeaderElectionConfig `json:",inline"`
	// +optional
	PerformanceStatefulsetOrdinalsConfig `json:",inline"`
	// +optional
	DeploymentPerformanceArgs `json:",inline"`
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// performance configurations to tune the performance of a component controller
// these properties will be added/updated in the ConfigMap(config-leader-election)
// https://tekton.dev/docs/pipelines/enabling-ha/
type PerformanceLeaderElectionConfig struct {
	Buckets *uint `json:"buckets,omitempty"`
}

// allow to configure component controller ha mode to statefulset ordinals
type PerformanceStatefulsetOrdinalsConfig struct {
	//if is true, enable StatefulsetOrdinals mode
	StatefulsetOrdinals *bool `json:"statefulset-ordinals,omitempty"`
}

// performance configurations to tune the performance of a component controller
// these properties will be added/updated as arguments in component controller deployment
// https://tekton.dev/docs/pipelines/tekton-controller-performance-configuration/
type DeploymentPerformanceArgs struct {
	// if it is true, disables the HA feature
	DisableHA bool `json:"disable-ha"`

	// The number of workers to use when processing the component controller's work queue
	ThreadsPerController *int `json:"threads-per-controller,omitempty"`

	// queries per second (QPS) and burst to the master from rest API client
	// actually the number multiplied by 2
	// https://github.com/pierretasci/pipeline/blob/05d67e427c722a2a57e58328d7097e21429b7524/cmd/controller/main.go#L85-L87
	// defaults: https://github.com/tektoncd/pipeline/blob/34618964300620dca44d10a595e4af84e9903a55/vendor/k8s.io/client-go/rest/config.go#L45-L46
	KubeApiQPS   *float32 `json:"kube-api-qps,omitempty"`
	KubeApiBurst *int     `json:"kube-api-burst,omitempty"`
}
