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
	"errors"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/common/legacybuilds"
	"gotest.tools/v3/assert"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func legacyCSI() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: legacyNodeDaemonSet, Namespace: legacybuilds.Namespace,
			UID: types.UID("legacy-csi"), Finalizers: []string{"operator.openshift.io/openshiftbuilds"}},
		Spec: appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "old-csi"}}},
	}
}

func migratingSharedResource() *v1alpha1.SharedResource {
	return &v1alpha1.SharedResource{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.SharedResourceResourceName},
		Spec:       v1alpha1.SharedResourceSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "openshift-pipelines"}},
	}
}

func legacyBuildsParent() *unstructured.Unstructured {
	parent := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.openshift.io/v1alpha1", "kind": "OpenShiftBuild",
		"metadata": map[string]interface{}{"name": "cluster"},
	}}
	parent.SetFinalizers([]string{"operator.openshift.io/openshiftbuilds"})
	return parent
}

func TestCSIMigration_DrainsBeforeInstallingReplacement(t *testing.T) {
	ctx := context.Background()
	d := legacyCSI()
	client := kubefake.NewSimpleClientset(d)
	dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), legacyBuildsParent())
	oe := &openshiftExtension{kubernetesClient: client, dynClient: dynamicClient}
	// The production PreReconcile hook must requeue before TLS resolution or
	// InstallerSet creation while the legacy node plugin is being retired.
	assert.Assert(t, errors.Is(oe.PreReconcile(ctx, migratingSharedResource()), v1alpha1.REQUEUE_EVENT_AFTER))
	assert.NilError(t, prepareLegacySharedResource(ctx, client, dynamicClient, "openshift-pipelines"))
	updated, err := client.AppsV1().DaemonSets(legacybuilds.Namespace).Get(ctx, d.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, updated.UID, d.UID)
	assert.DeepEqual(t, updated.Finalizers, d.Finalizers)
	assert.Assert(t, updated.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil)
}

func TestCSIMigration_RetiresWebhookAfterReplacementAndPreservesData(t *testing.T) {
	ctx := context.Background()
	sr := migratingSharedResource()
	one := int32(1)
	webhook := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: WebhookDeploymentName, Namespace: legacybuilds.Namespace},
		Spec:       appsv1.DeploymentSpec{Replicas: &one, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "old-webhook"}}},
	}
	source := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "source", Namespace: "user"}, Data: map[string]string{"value": "preserved"}}
	replacement := webhook.DeepCopy()
	replacement.Namespace = sr.Spec.TargetNamespace
	replacement.Generation = 2
	replacement.Status = appsv1.DeploymentStatus{ObservedGeneration: 2, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	configuration := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "validation.webhook.csidriversharedresource"},
		Webhooks: []admissionv1.ValidatingWebhook{{ClientConfig: admissionv1.WebhookClientConfig{
			Service: &admissionv1.ServiceReference{Namespace: sr.Spec.TargetNamespace, Name: WebhookDeploymentName}, CABundle: []byte("injected"),
		}}},
	}
	client := kubefake.NewSimpleClientset(legacyCSI(), webhook, source, replacement, configuration)
	parent := legacyBuildsParent()
	dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), parent)
	operatorClient := operatorfake.NewSimpleClientset(sr)
	assert.Assert(t, errors.Is(prepareLegacySharedResource(ctx, client, dynamicClient, sr.Spec.TargetNamespace), v1alpha1.REQUEUE_EVENT_AFTER))
	assert.Assert(t, errors.Is(migrateLegacySharedResource(ctx, client, dynamicClient, operatorClient, sr), v1alpha1.REQUEUE_EVENT_AFTER))
	unchanged, err := operatorClient.OperatorV1alpha1().SharedResources().Get(ctx, sr.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, unchanged.Annotations[migrationAnnotation] == "")
	// A crashing replacement must not retire the old webhook or mark migration done.
	replacement.Status.ReadyReplicas = 0
	_, err = client.AppsV1().Deployments(replacement.Namespace).UpdateStatus(ctx, replacement, metav1.UpdateOptions{})
	assert.NilError(t, err)
	assert.Assert(t, errors.Is(migrateLegacySharedResource(ctx, client, dynamicClient, operatorClient, sr), v1alpha1.REQUEUE_EVENT_AFTER))
	replacement.Status.ReadyReplicas = 1
	_, err = client.AppsV1().Deployments(replacement.Namespace).UpdateStatus(ctx, replacement, metav1.UpdateOptions{})
	assert.NilError(t, err)
	assert.NilError(t, migrateLegacySharedResource(ctx, client, dynamicClient, operatorClient, sr))
	updated, err := operatorClient.OperatorV1alpha1().SharedResources().Get(ctx, sr.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, updated.Annotations[migrationAnnotation], migrationDone)
	assert.NilError(t, migrateLegacySharedResource(ctx, client, dynamicClient, operatorClient, updated))
	data, err := client.CoreV1().ConfigMaps(source.Namespace).Get(ctx, source.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, data.Data, source.Data)
	gvr := schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1alpha1", Resource: "openshiftbuilds"}
	remainingParent, err := dynamicClient.Resource(gvr).Get(ctx, "cluster", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, remainingParent.Object, parent.Object)
	for _, action := range client.Actions() {
		assert.Assert(t, action.GetVerb() != "delete" && action.GetVerb() != "delete-collection")
	}
	for _, action := range dynamicClient.Actions() {
		assert.Equal(t, action.GetVerb(), "get")
	}
}

func TestCSIMigration_CompletionMarkerDoesNotSkipRestart(t *testing.T) {
	ctx := context.Background()
	sr := migratingSharedResource()
	sr.Annotations = map[string]string{migrationAnnotation: migrationDone}
	client := kubefake.NewSimpleClientset(legacyCSI())
	dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), legacyBuildsParent())
	assert.Assert(t, errors.Is(migrateLegacySharedResource(ctx, client, dynamicClient, operatorfake.NewSimpleClientset(sr), sr), v1alpha1.REQUEUE_EVENT_AFTER))
}

func TestCSIMigration_SameNamespaceUpdatesInPlace(t *testing.T) {
	ctx := context.Background()
	sr := migratingSharedResource()
	sr.Spec.TargetNamespace = legacybuilds.Namespace
	d := legacyCSI()
	client := kubefake.NewSimpleClientset(d)
	dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), legacyBuildsParent())
	assert.NilError(t, prepareLegacySharedResource(ctx, client, dynamicClient, sr.Spec.TargetNamespace))
	assert.NilError(t, migrateLegacySharedResource(ctx, client, dynamicClient, operatorfake.NewSimpleClientset(sr), sr))
	updated, err := client.AppsV1().DaemonSets(legacybuilds.Namespace).Get(ctx, d.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, updated, d)
}

func TestCSIMigration_FreshInstallHasNoMigrationMarker(t *testing.T) {
	sr := migratingSharedResource()
	operatorClient := operatorfake.NewSimpleClientset(sr)
	assert.NilError(t, migrateLegacySharedResource(context.Background(), kubefake.NewSimpleClientset(),
		dynfake.NewSimpleDynamicClient(runtime.NewScheme()), operatorClient, sr))
	assert.Equal(t, len(operatorClient.Actions()), 0)
}

func TestCSIMigration_PropagatesFailures(t *testing.T) {
	for _, stage := range []string{"detection", "annotation"} {
		t.Run(stage, func(t *testing.T) {
			sr := migratingSharedResource()
			dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), legacyBuildsParent())
			operatorClient := operatorfake.NewSimpleClientset(sr)
			want := errors.New("retry")
			reactor := func(clienttesting.Action) (bool, runtime.Object, error) { return true, nil, want }
			if stage == "detection" {
				dynamicClient.PrependReactor("get", "openshiftbuilds", reactor)
			} else {
				operatorClient.PrependReactor("patch", "sharedresources", reactor)
			}
			assert.Assert(t, errors.Is(migrateLegacySharedResource(context.Background(), kubefake.NewSimpleClientset(), dynamicClient, operatorClient, sr), want))
		})
	}
}

func TestWaitForNodePluginReady(t *testing.T) {
	for _, scenario := range []string{"ready", "absent", "generation", "no-nodes", "updating", "unready", "unavailable", "error"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			d := legacyCSI()
			d.Namespace = "openshift-pipelines"
			d.Generation = 2
			d.Status = appsv1.DaemonSetStatus{
				ObservedGeneration: 2, DesiredNumberScheduled: 3, UpdatedNumberScheduled: 3,
				NumberReady: 3, NumberAvailable: 3,
			}
			switch scenario {
			case "generation":
				d.Status.ObservedGeneration = 1
			case "no-nodes":
				d.Status = appsv1.DaemonSetStatus{ObservedGeneration: 2}
			case "updating":
				d.Status.UpdatedNumberScheduled = 2
			case "unready":
				d.Status.NumberReady = 2
			case "unavailable":
				d.Status.NumberUnavailable = 1
			}
			client := kubefake.NewSimpleClientset()
			if scenario != "absent" {
				_, err := client.AppsV1().DaemonSets(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
				assert.NilError(t, err)
			}
			want := errors.New("retry")
			if scenario == "error" {
				client.PrependReactor("get", "daemonsets", func(clienttesting.Action) (bool, runtime.Object, error) { return true, nil, want })
			}
			err := waitForNodePluginReady(ctx, client, d.Namespace)
			switch scenario {
			case "ready":
				assert.NilError(t, err)
			case "error":
				assert.Assert(t, errors.Is(err, want))
			default:
				assert.Assert(t, errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER))
			}
		})
	}
}
