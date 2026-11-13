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

package legacybuilds

import (
	"context"
	"errors"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestWaitForWebhookReady(t *testing.T) {
	for _, scenario := range []string{"ready", "absent", "generation", "updating", "crashing", "unavailable", "disabled"} {
		t.Run(scenario, func(t *testing.T) {
			one := int32(1)
			d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "webhook", Namespace: "new", Generation: 2},
				Spec:   appsv1.DeploymentSpec{Replicas: &one},
				Status: appsv1.DeploymentStatus{ObservedGeneration: 2, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}}
			switch scenario {
			case "generation":
				d.Status.ObservedGeneration = 1
			case "updating":
				d.Status.UpdatedReplicas = 0
			case "crashing":
				d.Status.ReadyReplicas = 0
			case "unavailable":
				d.Status.UnavailableReplicas = 1
			case "disabled":
				*d.Spec.Replicas = 0
			}
			client := kubefake.NewSimpleClientset()
			if scenario != "absent" {
				_, err := client.AppsV1().Deployments(d.Namespace).Create(context.Background(), d, metav1.CreateOptions{})
				assert.NilError(t, err)
			}
			err := WaitForWebhookReady(context.Background(), client, d.Namespace, d.Name)
			if scenario == "ready" {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER))
			}
		})
	}
}

func TestWebhookReadiness(t *testing.T) {
	r := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]interface{}{"name": "webhook"},
		"spec": map[string]interface{}{"template": map[string]interface{}{"spec": map[string]interface{}{
			"containers": []interface{}{map[string]interface{}{"name": "webhook"}},
		}}},
	}}
	transform := WebhookReadiness("webhook", "webhook", 8443)
	assert.NilError(t, transform(r))
	containers, _, err := unstructured.NestedSlice(r.Object, "spec", "template", "spec", "containers")
	assert.NilError(t, err)
	probe, _, err := unstructured.NestedMap(containers[0].(map[string]interface{}), "readinessProbe")
	assert.NilError(t, err)
	port, _, err := unstructured.NestedInt64(probe, "tcpSocket", "port")
	assert.NilError(t, err)
	assert.Equal(t, port, int64(8443))
	before := r.DeepCopy()
	assert.NilError(t, transform(r))
	assert.DeepEqual(t, r.Object, before.Object)
}

func TestWaitForConversionRouting(t *testing.T) {
	for _, scenario := range []string{"ready", "legacy", "missing-ca", "absent"} {
		t.Run(scenario, func(t *testing.T) {
			r := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
				"metadata": map[string]interface{}{"name": "builds.shipwright.io"},
			}}
			config := map[string]interface{}{"service": map[string]interface{}{"namespace": "new", "name": "webhook"}, "caBundle": "injected"}
			if scenario == "legacy" {
				config["service"].(map[string]interface{})["namespace"] = Namespace
			}
			if scenario == "missing-ca" {
				delete(config, "caBundle")
			}
			assert.NilError(t, unstructured.SetNestedMap(r.Object, config, "spec", "conversion", "webhook", "clientConfig"))
			objects := []runtime.Object{r}
			if scenario == "absent" {
				objects = nil
			}
			err := WaitForConversionRouting(context.Background(), dynfake.NewSimpleDynamicClient(runtime.NewScheme(), objects...), "new", "webhook", r.GetName())
			if scenario == "ready" {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER))
			}
		})
	}
}
