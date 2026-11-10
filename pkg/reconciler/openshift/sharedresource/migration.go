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

package sharedresource

import (
	"context"
	"encoding/json"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/common/legacybuilds"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	legacyNodeDaemonSet = NodeDaemonSetName
	migrationAnnotation = "operator.tekton.dev/openshift-builds-migration"
	migrationDone       = "csi-completed"
)

// DaemonSets are in the static InstallerSet, which does not check their rollout.
// Do not report Ready or retire the old webhook until every desired node has the
// replacement plugin available at the observed generation.
func waitForNodePluginReady(ctx context.Context, client kubernetes.Interface, namespace string) error {
	d, err := client.AppsV1().DaemonSets(namespace).Get(ctx, NodeDaemonSetName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	if err != nil {
		return err
	}
	desired := d.Status.DesiredNumberScheduled
	if d.Status.ObservedGeneration < d.Generation || desired == 0 || d.Status.UpdatedNumberScheduled != desired ||
		d.Status.NumberReady != desired || d.Status.NumberAvailable != desired || d.Status.NumberUnavailable != 0 {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	return nil
}

// prepareLegacySharedResource runs before InstallerSets: two node plugins must
// never serve the same driver/socket concurrently. Stopping the manager alone is
// insufficient because the DaemonSet controller keeps its old node pods running.
func prepareLegacySharedResource(ctx context.Context, kubeClient kubernetes.Interface, dynClient dynamic.Interface, namespace string) error {
	if err := legacybuilds.Prepare(ctx, kubeClient, dynClient); err != nil {
		return err
	}
	if namespace == legacybuilds.Namespace {
		return nil // InstallerSets update the existing node plugin in place.
	}
	return legacybuilds.StopDaemonSet(ctx, kubeClient, legacyNodeDaemonSet)
}

// migrateLegacySharedResource runs only after the replacement InstallerSets are
// ready. Keep all legacy parents, finalizers, RBAC and shared data intact. They
// are not a prerequisite for handing management to Pipelines.
func migrateLegacySharedResource(ctx context.Context, kubeClient kubernetes.Interface, dynClient dynamic.Interface, operatorClient versioned.Interface, sr *v1alpha1.SharedResource) error {
	if err := prepareLegacySharedResource(ctx, kubeClient, dynClient, sr.Spec.TargetNamespace); err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1alpha1", Resource: "openshiftbuilds"}
	_, err := dynClient.Resource(gvr).Get(ctx, "cluster", metav1.GetOptions{})
	if apierrors.IsNotFound(err) || apimeta.IsNoMatchError(err) {
		return nil // Fresh installation: do not mark a migration that never ran.
	}
	if err != nil {
		return err
	}
	if sr.Spec.TargetNamespace != legacybuilds.Namespace {
		_, err := kubeClient.AppsV1().Deployments(legacybuilds.Namespace).Get(ctx, WebhookDeploymentName, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if err == nil {
			if err := legacybuilds.WaitForWebhookReady(ctx, kubeClient, sr.Spec.TargetNamespace, WebhookDeploymentName); err != nil {
				return err
			}
			configuration, err := kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, "validation.webhook.csidriversharedresource", metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return v1alpha1.REQUEUE_EVENT_AFTER
			}
			if err != nil {
				return err
			}
			if len(configuration.Webhooks) == 0 {
				return v1alpha1.REQUEUE_EVENT_AFTER
			}
			for _, webhook := range configuration.Webhooks {
				service := webhook.ClientConfig.Service
				if service == nil || service.Namespace != sr.Spec.TargetNamespace || service.Name != WebhookDeploymentName || len(webhook.ClientConfig.CABundle) == 0 {
					return v1alpha1.REQUEUE_EVENT_AFTER
				}
			}
		}
		if err := legacybuilds.StopDeployment(ctx, kubeClient, WebhookDeploymentName); err != nil {
			return err
		}
	}
	if sr.Annotations[migrationAnnotation] == migrationDone {
		return nil
	}
	patch, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"annotations": map[string]string{migrationAnnotation: migrationDone}},
	})
	if err != nil {
		return err
	}
	_, err = operatorClient.OperatorV1alpha1().SharedResources().Patch(ctx, sr.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}
