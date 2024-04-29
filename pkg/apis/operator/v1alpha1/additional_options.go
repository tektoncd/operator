/*
Copyright 2023 The Tekton Authors

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
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
)

// additional options will be updated on the manifests
// these values will be final
type AdditionalOptions struct {
	Disabled                    *bool                                            `json:"disabled,omitempty"`
	ConfigMaps                  map[string]corev1.ConfigMap                      `json:"configMaps,omitempty"`
	Deployments                 map[string]appsv1.Deployment                     `json:"deployments,omitempty"`
	HorizontalPodAutoscalers    map[string]autoscalingv2.HorizontalPodAutoscaler `json:"horizontalPodAutoscalers,omitempty"`
	StatefulSets                map[string]appsv1.StatefulSet                    `json:"statefulSets,omitempty"`
	WebhookConfigurationOptions map[string]WebhookConfigurationOptions           `json:"webhookConfigurationOptions,omitempty"`
}
