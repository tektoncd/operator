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

package tektonkueue

import (
	"context"
	"fmt"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	isfake "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client/fake"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	discoveryfake "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
)

func pluginKueue() *v1alpha1.TektonKueue {
	return &v1alpha1.TektonKueue{
		ObjectMeta: metav1.ObjectMeta{Name: "kueue", UID: "kueue-owner"},
		Spec: v1alpha1.TektonKueueSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "pipelines"},
			Kueue: v1alpha1.Kueue{
				Disabled:           ptr.To(false),
				MultiClusterConfig: v1alpha1.MultiClusterConfig{MultiClusterRole: v1alpha1.MultiClusterRoleHub, FleetManager: v1alpha1.FleetManagerACM},
			},
		},
	}
}

func pluginManifest(t *testing.T) mf.Manifest {
	t.Helper()
	m, err := mf.NewManifest("testdata/plugin.yaml")
	assert.NilError(t, err)
	return m
}

func pluginDiscovery() *discoveryfake.FakeDiscovery {
	d := k8sfake.NewSimpleClientset().Discovery().(*discoveryfake.FakeDiscovery)
	for _, gv := range []string{
		"cluster.open-cluster-management.io/v1", "authentication.open-cluster-management.io/v1beta1",
		"work.open-cluster-management.io/v1", "operators.coreos.com/v1", "operators.coreos.com/v1alpha1",
		"kueue.openshift.io/v1", "kueue.x-k8s.io/v1beta2",
	} {
		d.Resources = append(d.Resources, &metav1.APIResourceList{GroupVersion: gv})
	}
	return d
}

func TestPluginLifecycle(t *testing.T) {
	ctx := t.Context()
	kueue := pluginKueue()
	isClient := isfake.NewFakeISClient()
	oe := openshiftExtension{
		manifest: pluginManifest(t), discovery: pluginDiscovery(),
		installerSetClient: client.NewInstallerSetClient(isClient, "v1", "plugin", v1alpha1.KindTektonKueue, nil),
	}
	assert.NilError(t, oe.PreReconcile(ctx, kueue))
	assert.Equal(t, oe.PostReconcile(ctx, kueue), v1alpha1.REQUEUE_EVENT_AFTER)
	sets, err := isClient.List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(sets.Items), 1)
	set := sets.Items[0]
	assert.Equal(t, set.OwnerReferences[0].UID, kueue.UID)
	assert.Equal(t, set.Labels[v1alpha1.InstallerSetType], "custom-multikueue-plugin")
	assert.Assert(t, !kueue.Status.IsReady())
	set.Status.MarkCRDsInstalled()
	set.Status.MarkNamespaceScopedResourcesInstalled()
	set.Status.MarkClustersScopedResourcesInstalled()
	set.Status.MarkDeploymentsAvailable()
	set.Status.MarkStatefulSetReady()
	set.Status.MarkDaemonSetReady()
	set.Status.MarkControllerReady()
	set.Status.MarkWebhookReady()
	set.Status.MarkAllDeploymentsReady()
	set.Status.MarkJobsInstalled()
	_, err = isClient.Update(ctx, &set, metav1.UpdateOptions{})
	assert.NilError(t, err)
	assert.NilError(t, oe.PostReconcile(ctx, kueue))
	sets, err = isClient.List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, sets.Items[0].Name, set.Name)

	// An operator upgrade replaces the plugin set using the pinned payload.
	oe.installerSetClient = client.NewInstallerSetClient(isClient, "v2", "plugin", v1alpha1.KindTektonKueue, nil)
	assert.Equal(t, oe.PostReconcile(ctx, kueue), v1alpha1.REQUEUE_EVENT_AFTER)
	assert.Equal(t, oe.PostReconcile(ctx, kueue), v1alpha1.REQUEUE_EVENT_AFTER)
	sets, err = isClient.List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(sets.Items), 1)
	assert.Equal(t, sets.Items[0].Labels[v1alpha1.ReleaseVersionKey], "v2")

	// Deselecting does not need discovery, even if ACM was removed first.
	oe.discovery = nil
	kueue.Spec.FleetManager = v1alpha1.FleetManagerManual
	assert.NilError(t, oe.PreReconcile(ctx, kueue))
	assert.NilError(t, oe.PostReconcile(ctx, kueue))
	sets, err = isClient.List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(sets.Items), 0)
}

func TestPluginInactiveAndFinalize(t *testing.T) {
	for _, mode := range []string{"omitted", "manual", "disabled", "single-cluster", "spoke", "finalize"} {
		t.Run(mode, func(t *testing.T) {
			kueue := pluginKueue()
			set := &v1alpha1.TektonInstallerSet{ObjectMeta: metav1.ObjectMeta{Name: "plugin", Labels: map[string]string{
				v1alpha1.CreatedByKey: v1alpha1.KindTektonKueue, v1alpha1.InstallerSetType: "custom-multikueue-plugin",
			}}}
			isClient := isfake.NewFakeISClient(set)
			oe := openshiftExtension{installerSetClient: client.NewInstallerSetClient(isClient, "v1", "plugin", v1alpha1.KindTektonKueue, nil)}
			switch mode {
			case "omitted":
				kueue.Spec.FleetManager = ""
			case "manual":
				kueue.Spec.FleetManager = v1alpha1.FleetManagerManual
			case "disabled":
				kueue.Spec.Disabled = ptr.To(true)
			case "single-cluster":
				kueue.Spec.MultiClusterDisabled = true
			case "spoke":
				kueue.Spec.MultiClusterRole = v1alpha1.MultiClusterRoleSpoke
			}
			if mode == "finalize" {
				assert.NilError(t, oe.Finalize(t.Context(), kueue))
			} else {
				assert.NilError(t, oe.PreReconcile(t.Context(), kueue))
				assert.NilError(t, oe.PostReconcile(t.Context(), kueue))
			}
			sets, err := isClient.List(t.Context(), metav1.ListOptions{})
			assert.NilError(t, err)
			assert.Equal(t, len(sets.Items), 0)
		})
	}
}

func TestPluginMissingDependency(t *testing.T) {
	kueue := pluginKueue()
	kueue.Status.InitializeConditions()
	kueue.Status.MarkPostReconcilerComplete()
	oe := openshiftExtension{discovery: k8sfake.NewSimpleClientset().Discovery()}
	assert.Equal(t, oe.PostReconcile(t.Context(), kueue), v1alpha1.REQUEUE_EVENT_AFTER)
	condition := kueue.Status.GetCondition(v1alpha1.PostReconciler)
	assert.Equal(t, condition.Status, corev1.ConditionFalse)
	assert.Assert(t, condition.Message != "")
}

func TestPluginCleanupRetries(t *testing.T) {
	set := &v1alpha1.TektonInstallerSet{ObjectMeta: metav1.ObjectMeta{Name: "plugin", Labels: map[string]string{
		v1alpha1.CreatedByKey: v1alpha1.KindTektonKueue, v1alpha1.InstallerSetType: "custom-multikueue-plugin",
	}}}
	for _, fail := range []bool{false, true} {
		op := operatorfake.NewSimpleClientset(set.DeepCopy())
		op.PrependReactor("delete", "tektoninstallersets", func(clienttesting.Action) (bool, runtime.Object, error) {
			if fail {
				return true, nil, fmt.Errorf("delete failed")
			}
			return true, nil, nil // Simulate foreground deletion still in progress.
		})
		oe := openshiftExtension{installerSetClient: client.NewInstallerSetClient(op.OperatorV1alpha1().TektonInstallerSets(), "v1", "plugin", v1alpha1.KindTektonKueue, nil)}
		// A failed delete and a delete still in progress are both retried.
		assert.Equal(t, oe.cleanupPlugin(t.Context()), v1alpha1.REQUEUE_EVENT_AFTER)
		// Finalize only starts the removal, so it reports a failed delete and nothing else.
		assert.Equal(t, oe.Finalize(t.Context(), pluginKueue()) != nil, fail)
	}
}

func TestPluginTransforms(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(fmt.Sprint(override), func(t *testing.T) {
			t.Setenv("IMAGE_MULTIKUEUE_PLUGIN_CONTROLLER", "quay.io/example/plugin:custom")
			kueue := pluginKueue()
			kueue.Spec.Options.ConfigMaps = map[string]corev1.ConfigMap{"main-config": {Data: map[string]string{"key": "value"}}}
			wantNamespace := "openshift-kueue-operator"
			if override {
				wantNamespace = "custom-kueue"
				kueue.Spec.Options.Deployments = map[string]appsv1.Deployment{
					"pipelines-multikueue-plugin-controller": {Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "controller", Env: []corev1.EnvVar{{Name: "KUEUE_NAMESPACE", Value: wantNamespace}}}},
					}}}},
				}
			}
			m := pluginManifest(t)
			transformed, err := transformPlugin(context.Background(), &m, kueue)
			assert.NilError(t, err)
			for _, obj := range transformed.Resources() {
				assert.Assert(t, obj.GetKind() != "ConfigMap", "plugin must not duplicate main-set ConfigMaps")
				switch obj.GetKind() {
				case "ClusterRoleBinding":
					subjects, _, err := unstructured.NestedSlice(obj.Object, "subjects")
					assert.NilError(t, err)
					want := "pipelines"
					if obj.GetName() == "pipelines-multikueue-plugin-kueue-rolebinding" {
						want = "openshift-kueue-operator"
					}
					assert.Equal(t, subjects[0].(map[string]interface{})["namespace"], want)
				case "Deployment":
					assert.Equal(t, obj.GetNamespace(), "pipelines")
					deployment := &appsv1.Deployment{}
					assert.NilError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, deployment))
					container := deployment.Spec.Template.Spec.Containers[0]
					assert.Equal(t, container.Image, "quay.io/example/plugin:custom")
					assert.Equal(t, container.Env[0].Name, "KUEUE_NAMESPACE")
					assert.Equal(t, container.Env[0].Value, wantNamespace)
					assert.Equal(t, obj.GetOwnerReferences()[0].UID, kueue.UID)
				}
			}
		})
	}
}
