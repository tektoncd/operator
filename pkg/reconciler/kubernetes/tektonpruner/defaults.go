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

package tektonpruner

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Default resource limits for pruner deployments based on empirical benchmark data.
//
// Override Mechanism:
// These are operator-internal defaults. End users can override via:
//
//	TektonPruner.Spec.Options.Deployments (TektonPruner CR)
//	TektonConfig.Spec.Pruner.Options.Deployments (TektonConfig CR)
//
// The options transformer runs after this default transformer, so user values
// always take precedence.
const (
	// Controller resources - handles PipelineRun/TaskRun informer cache
	defaultControllerCPURequest    = "100m"
	defaultControllerMemoryRequest = "256Mi" // Baseline for typical workloads (5k-10k runs)
	defaultControllerMemoryLimit   = "2Gi"   // Supports ~50k runs with 10x safety margin (846-1126 MB measured at 26k-38k runs)

	// Webhook resources - stateless validation webhook
	defaultWebhookCPURequest    = "50m"
	defaultWebhookMemoryRequest = "64Mi"
	defaultWebhookMemoryLimit   = "512Mi" // Stateless, minimal footprint (62-95 MB across all scales)
)

var (
	// DefaultControllerResources defines resource requests and limits for tekton-pruner-controller.
	// Based on benchmark data showing linear memory growth with resident PipelineRuns.
	DefaultControllerResources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(defaultControllerCPURequest),
			corev1.ResourceMemory: resource.MustParse(defaultControllerMemoryRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(defaultControllerMemoryLimit),
		},
	}

	// DefaultWebhookResources defines resource requests and limits for tekton-pruner-webhook.
	// Webhook is stateless with minimal memory footprint regardless of cluster scale.
	DefaultWebhookResources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(defaultWebhookCPURequest),
			corev1.ResourceMemory: resource.MustParse(defaultWebhookMemoryRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(defaultWebhookMemoryLimit),
		},
	}
)
