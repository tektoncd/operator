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
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	fakeoperatorclient "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	installersetclient "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	openshiftcommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func controllerDeployment(t *testing.T) unstructured.Unstructured {
	t.Helper()
	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebhookDeploymentName,
			Namespace: "openshift-builds",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "manager"}},
				},
			},
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
	if err != nil {
		t.Fatalf("failed to convert deployment: %v", err)
	}
	return unstructured.Unstructured{Object: obj}
}

func namespaceResource(t *testing.T) unstructured.Unstructured {
	t.Helper()
	return unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]interface{}{"name": "openshift-builds"},
	}}
}

// baseTransformerCount is the number of transformers the extension always applies
// (RemoveRunAsUser, RemoveRunAsGroup, UpdateServiceMonitorTargetNamespace,
// WebhookReadiness), independent of TLS policy or metrics-mTLS.
const baseTransformerCount = 4

func webhookDeployment(t *testing.T) unstructured.Unstructured {
	t.Helper()
	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebhookDeploymentName,
			Namespace: "openshift-builds",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: WebhookContainerName}},
				},
			},
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
	if err != nil {
		t.Fatalf("failed to convert deployment: %v", err)
	}
	return unstructured.Unstructured{Object: obj}
}

func webhookArgs(t *testing.T, m mf.Manifest) []string {
	t.Helper()
	for _, res := range m.Resources() {
		if res.GetKind() != "Deployment" || res.GetName() != WebhookDeploymentName {
			continue
		}
		d := &appsv1.Deployment{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.Object, d); err != nil {
			t.Fatalf("failed to convert back: %v", err)
		}
		for _, c := range d.Spec.Template.Spec.Containers {
			if c.Name == WebhookContainerName {
				return c.Args
			}
		}
	}
	return nil
}

func containsSequence(args []string, first, second string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == first && args[i+1] == second {
			return true
		}
	}
	return false
}

func TestOpenShiftExtensionTransformers(t *testing.T) {
	oe := &openshiftExtension{}
	// With no TLS policy and metrics mTLS not ready, only the base SCC transformers run.
	transformers := oe.Transformers(&v1alpha1.SharedResource{})
	util.AssertEqual(t, len(transformers), baseTransformerCount)
}

func TestOpenShiftExtensionTransformers_WithTLSConfig(t *testing.T) {
	tlsConfig := &openshiftcommon.TLSEnvVars{
		MinVersion:   "1.2",
		CipherSuites: "TLS_AES_128_GCM_SHA256",
	}
	oe := &openshiftExtension{resolvedTLSConfig: tlsConfig}
	transformers := oe.Transformers(&v1alpha1.SharedResource{})

	// A resolved TLS policy adds exactly one transformer (the webhook args).
	util.AssertEqual(t, len(transformers), baseTransformerCount+1)

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{webhookDeployment(t)}))
	util.AssertEqual(t, err, nil)

	out, err := manifest.Transform(transformers...)
	util.AssertEqual(t, err, nil)

	// Shared Resource passes MinVersion through unchanged (unlike Shipwright, which
	// rewrites it to the "VersionTLS12" form).
	args := webhookArgs(t, out)
	if !containsSequence(args, tlsMinVersion, tlsConfig.MinVersion) {
		t.Errorf("expected webhook args to contain %q %q, got %v", tlsMinVersion, tlsConfig.MinVersion, args)
	}
	if !containsSequence(args, tlsCipherSuites, tlsConfig.CipherSuites) {
		t.Errorf("expected webhook args to contain %q %q, got %v", tlsCipherSuites, tlsConfig.CipherSuites, args)
	}
}

func TestOpenShiftExtensionTransformers_MetricsMTLSReady(t *testing.T) {
	oe := &openshiftExtension{metricsMTLSReady: true}
	transformers := oe.Transformers(&v1alpha1.SharedResource{})

	// Metrics-mTLS readiness adds two transformers (metrics port rename and the
	// ServiceMonitor mTLS update).
	util.AssertEqual(t, len(transformers), baseTransformerCount+2)
}

func TestOpenShiftExtensionTransformers_TLSAndMetrics(t *testing.T) {
	oe := &openshiftExtension{
		resolvedTLSConfig: &openshiftcommon.TLSEnvVars{MinVersion: "1.2", CipherSuites: "TLS_AES_128_GCM_SHA256"},
		metricsMTLSReady:  true,
	}
	transformers := oe.Transformers(&v1alpha1.SharedResource{})

	// Both branches contribute: base + 2 metrics + 1 TLS.
	util.AssertEqual(t, len(transformers), baseTransformerCount+3)
}

// TestOpenShiftExtensionPostReconcileNetworkPolicyDisabled verifies that when
// spec.networkPolicy.disabled is set, PostReconcile removes the NetworkPolicy
// custom InstallerSet (and only that set).
func TestOpenShiftExtensionPostReconcileNetworkPolicyDisabled(t *testing.T) {
	ctx := context.Background()

	npSetType := installersetclient.InstallerTypeCustom + "-" + NetworkPolicySet
	npSet := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: NetworkPolicySet + "-abcde",
			Labels: map[string]string{
				v1alpha1.CreatedByKey:     v1alpha1.KindSharedResource,
				v1alpha1.InstallerSetType: npSetType,
			},
		},
	}
	mainSet := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shared-resource-main-deployment-xyz",
			Labels: map[string]string{
				v1alpha1.CreatedByKey:     v1alpha1.KindSharedResource,
				v1alpha1.InstallerSetType: installersetclient.InstallerTypeMain,
			},
		},
	}

	fakeClient := fakeoperatorclient.NewSimpleClientset(npSet, mainSet)
	tisClient := fakeClient.OperatorV1alpha1().TektonInstallerSets()
	one := int32(1)
	nodePlugin := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: NodeDaemonSetName},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 1, UpdatedNumberScheduled: 1,
			NumberReady: 1, NumberAvailable: 1,
		},
	}
	webhook := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: WebhookDeploymentName},
		Spec:       appsv1.DeploymentSpec{Replicas: &one},
		Status:     appsv1.DeploymentStatus{UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1},
	}
	oe := &openshiftExtension{
		installerSetClient: installersetclient.NewInstallerSetClient(
			tisClient, "test-version", OperandName, v1alpha1.KindSharedResource, nil),
		kubernetesClient:  kubefake.NewSimpleClientset(nodePlugin, webhook),
		operatorClientset: fakeClient,
		dynClient:         dynfake.NewSimpleDynamicClient(runtime.NewScheme()),
	}

	sr := &v1alpha1.SharedResource{
		Spec: v1alpha1.SharedResourceSpec{
			NetworkPolicy: v1alpha1.NetworkPolicyConfig{Disabled: true},
		},
	}

	util.AssertEqual(t, oe.PostReconcile(ctx, sr), nil)

	list, err := tisClient.List(ctx, metav1.ListOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, len(list.Items), 1)
	util.AssertEqual(t, list.Items[0].GetName(), mainSet.GetName())
}

func TestOpenShiftExtensionGetPlatformData(t *testing.T) {
	oe := &openshiftExtension{}
	util.AssertEqual(t, oe.GetPlatformData(), "")
}

// TestOpenShiftExtensionFinalize verifies that Finalize cleans up the
// SharedResource NetworkPolicy custom InstallerSet (and only that set), and is a
// no-op once it is already gone.
func TestOpenShiftExtensionFinalize(t *testing.T) {
	ctx := context.Background()

	// InstallerSetType value that CustomSet assigns to the NetworkPolicy set.
	npSetType := installersetclient.InstallerTypeCustom + "-" + NetworkPolicySet

	// The NetworkPolicy custom set owned by this component; Finalize must delete it.
	npSet := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: NetworkPolicySet + "-abcde",
			Labels: map[string]string{
				v1alpha1.CreatedByKey:     v1alpha1.KindSharedResource,
				v1alpha1.InstallerSetType: npSetType,
			},
		},
	}
	// An unrelated main set that must survive Finalize.
	mainSet := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shared-resource-main-deployment-xyz",
			Labels: map[string]string{
				v1alpha1.CreatedByKey:     v1alpha1.KindSharedResource,
				v1alpha1.InstallerSetType: installersetclient.InstallerTypeMain,
			},
		},
	}

	fakeClient := fakeoperatorclient.NewSimpleClientset(npSet, mainSet)
	tisClient := fakeClient.OperatorV1alpha1().TektonInstallerSets()
	oe := &openshiftExtension{
		installerSetClient: installersetclient.NewInstallerSetClient(
			tisClient, "test-version", OperandName, v1alpha1.KindSharedResource, nil),
	}

	// Finalize removes the NetworkPolicy custom set and returns nil.
	util.AssertEqual(t, oe.Finalize(ctx, &v1alpha1.SharedResource{}), nil)

	list, err := tisClient.List(ctx, metav1.ListOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, len(list.Items), 1)
	util.AssertEqual(t, list.Items[0].GetName(), mainSet.GetName())

	// A second Finalize is a no-op (nothing left to clean up) and still succeeds.
	util.AssertEqual(t, oe.Finalize(ctx, &v1alpha1.SharedResource{}), nil)
}

func TestFilterAndTransform(t *testing.T) {
	ctx := context.Background()
	r := &Reconciler{extension: common.NoExtension(ctx)}
	sr := &v1alpha1.SharedResource{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.SharedResourceResourceName},
		Spec: v1alpha1.SharedResourceSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "openshift-builds"},
		},
	}

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{namespaceResource(t), controllerDeployment(t)}))
	util.AssertEqual(t, err, nil)

	out, err := FilterAndTransform(r)(ctx, &manifest, sr)
	util.AssertEqual(t, err, nil)

	// The Namespace resource is filtered out.
	for _, res := range out.Resources() {
		if res.GetKind() == "Namespace" {
			t.Errorf("expected Namespace to be filtered out, but it is present")
		}
	}

	// The operand name label is injected onto the controller Deployment.
	d := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(out.Resources()[0].Object, d)
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, d.GetLabels()[v1alpha1.LabelOperandName], OperandName)
}
