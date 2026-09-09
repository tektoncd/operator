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
const (
	defaultControllerCPURequest    = "100m"
	defaultControllerMemoryRequest = "256Mi"
	defaultControllerMemoryLimit   = "2Gi"

	defaultWebhookCPURequest    = "50m"
	defaultWebhookMemoryRequest = "64Mi"
	defaultWebhookMemoryLimit   = "512Mi"
)

var (
	DefaultControllerResources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(defaultControllerCPURequest),
			corev1.ResourceMemory: resource.MustParse(defaultControllerMemoryRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(defaultControllerMemoryLimit),
		},
	}

	DefaultWebhookResources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(defaultWebhookCPURequest),
			corev1.ResourceMemory: resource.MustParse(defaultWebhookMemoryRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(defaultWebhookMemoryLimit),
		},
	}

	DefaultResourcesMap = map[string]corev1.ResourceRequirements{
		"tekton-pruner-controller": DefaultControllerResources,
		"tekton-pruner-webhook":    DefaultWebhookResources,
	}
)
