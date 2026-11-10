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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// WebhookReadiness requires a listening server, not merely a running container.
// Both Builds webhooks can otherwise briefly appear ready before crashing on
// invalid TLS options. Preserve any stronger probe provided by the payload.
func WebhookReadiness(deployment, container string, port int64) mf.Transformer {
	return func(resource *unstructured.Unstructured) error {
		if resource.GetKind() != "Deployment" || resource.GetName() != deployment {
			return nil
		}
		containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec", "containers")
		if err != nil || !found {
			return err
		}
		for _, value := range containers {
			c, ok := value.(map[string]interface{})
			if !ok || c["name"] != container || c["readinessProbe"] != nil {
				continue
			}
			c["readinessProbe"] = map[string]interface{}{
				"tcpSocket":     map[string]interface{}{"port": port},
				"periodSeconds": int64(5), "timeoutSeconds": int64(2),
				"successThreshold": int64(2), "failureThreshold": int64(2),
			}
		}
		return unstructured.SetNestedSlice(resource.Object, containers, "spec", "template", "spec", "containers")
	}
}

// WaitForWebhookReady fences retirement and migration completion on the current
// rollout. Deployment Available alone can describe an old replica during rollout.
func WaitForWebhookReady(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	d, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	if err != nil {
		return err
	}
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	if d.DeletionTimestamp != nil || desired == 0 || d.Status.ObservedGeneration < d.Generation ||
		d.Status.UpdatedReplicas != desired || d.Status.ReadyReplicas != desired ||
		d.Status.AvailableReplicas != desired || d.Status.UnavailableReplicas != 0 {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	return nil
}

// WaitForConversionRouting verifies every migrated CRD before the old conversion
// webhook is stopped. A stale or uninjected route must leave that webhook alive.
func WaitForConversionRouting(ctx context.Context, client dynamic.Interface, namespace, service string, crds ...string) error {
	gvr := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	for _, name := range crds {
		crd, err := client.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		if err != nil {
			return err
		}
		config, _, err := unstructured.NestedMap(crd.Object, "spec", "conversion", "webhook", "clientConfig")
		if err != nil {
			return err
		}
		route, _, err := unstructured.NestedMap(config, "service")
		if err != nil {
			return err
		}
		if route["namespace"] != namespace || route["name"] != service || config["caBundle"] == nil || config["caBundle"] == "" {
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}
	return nil
}
