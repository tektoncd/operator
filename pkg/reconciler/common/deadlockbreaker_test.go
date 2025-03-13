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

package common

import (
	"context"
	"fmt"
	"testing"

	mf "github.com/manifestival/manifestival"
	mff "github.com/manifestival/manifestival/fake"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestPreemptDeadlock(t *testing.T) {
	tests := []struct {
		name        string
		resources   []unstructured.Unstructured
		component   string
		expectError bool
		endpoints   *v1.Endpoints
	}{
		{
			name:        "Webhook service not found",
			resources:   []unstructured.Unstructured{},
			component:   v1alpha1.PipelineResourceName,
			expectError: true,
		},
		{
			name: "Webhook endpoints are active",
			resources: []unstructured.Unstructured{
				namespacedResource("v1", "Service", "test-namespace", "tekton-pipelines-webhook"),
				createWebhookConfig(),
			},
			component:   v1alpha1.PipelineResourceName,
			expectError: false,
			endpoints: &v1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tekton-pipelines-webhook",
					Namespace: "test-namespace",
				},
				Subsets: []v1.EndpointSubset{
					{
						Addresses: []v1.EndpointAddress{
							{IP: "1.2.3.4"},
						},
					},
				},
			},
		},
		{
			name: "Webhook endpoints are not active",
			resources: []unstructured.Unstructured{
				namespacedResource("v1", "Service", "test-namespace", "tekton-pipelines-webhook"),
				createWebhookConfig(),
			},
			component:   v1alpha1.PipelineResourceName,
			expectError: false,
			endpoints: &v1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tekton-pipelines-webhook",
					Namespace: "test-namespace",
				},
				Subsets: []v1.EndpointSubset{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mff.New([]runtime.Object{}...)
			k8sClient := k8sfake.NewSimpleClientset()

			manifest, err := mf.ManifestFrom(mf.Slice(tt.resources), mf.UseClient(client))
			if err != nil {
				t.Fatalf("Failed to generate manifest: %v", err)
			}

			if tt.endpoints != nil {
				_, err := k8sClient.CoreV1().Endpoints(tt.endpoints.Namespace).Create(context.TODO(), tt.endpoints, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create endpoint: %v", err)
				}
			}

			err = PreemptDeadlock(context.TODO(), &manifest, k8sClient, tt.component)

			if tt.expectError && err == nil {
				t.Errorf("Expected an error but got nil")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func createWebhookConfig() unstructured.Unstructured {
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "config.webhook.pipeline.tekton.dev",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "config.webhook.triggers.tekton.dev",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name:      "tekton-triggers-webhook",
						Namespace: "tekton-pipelines",
					},
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps/*"},
						},
					},
				},
			},
		},
	}

	webhookConfigObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(webhookConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to convert webhook config to unstructured: %v", err))
	}

	return unstructured.Unstructured{Object: webhookConfigObj}
}
