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

package shipwrightbuild

import (
	"context"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	fakeoperatorclient "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/shipwrightbuild"
	installersetclient "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	openshiftcommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// baseTransformerCount is the number of transformers the extension always applies
// (RemoveRunAsUser, RemoveRunAsGroup, AddServiceCABundle, AddServiceCertificate),
// independent of TLS policy or metrics-mTLS readiness.
const baseTransformerCount = 4

func webhookDeployment(t *testing.T) unstructured.Unstructured {
	t.Helper()
	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      shipwrightbuild.WebhookDeploymentName,
			Namespace: "openshift-builds",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: shipwrightbuild.WebhookContainerName},
					},
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

// controllerService builds the metrics Service the metrics-mTLS transformers act
// on; its name matches the controller Deployment name the extension passes to the
// metrics helpers.
func controllerService(t *testing.T) unstructured.Unstructured {
	t.Helper()
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      shipwrightbuild.ControllerDeploymentName,
			Namespace: "openshift-builds",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Name: openshiftcommon.MetricsHTTPPort, Port: 9090}},
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(svc)
	if err != nil {
		t.Fatalf("failed to convert service: %v", err)
	}
	return unstructured.Unstructured{Object: obj}
}

func webhookArgs(t *testing.T, m mf.Manifest) []string {
	t.Helper()
	for _, res := range m.Resources() {
		if res.GetKind() != "Deployment" || res.GetName() != shipwrightbuild.WebhookDeploymentName {
			continue
		}
		d := &appsv1.Deployment{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.Object, d); err != nil {
			t.Fatalf("failed to convert back: %v", err)
		}
		for _, c := range d.Spec.Template.Spec.Containers {
			if c.Name == shipwrightbuild.WebhookContainerName {
				return c.Args
			}
		}
	}
	return nil
}

func TestOpenShiftTransformers_NoTLSNoMetrics(t *testing.T) {
	oe := &openshiftExtension{resolvedTLSConfig: nil, metricsMTLSReady: false}
	transformers := oe.Transformers(&v1alpha1.ShipwrightBuild{})

	// Only the base transformers run when there is no TLS policy and metrics mTLS
	// is not yet ready.
	util.AssertEqual(t, len(transformers), baseTransformerCount)

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{webhookDeployment(t)}))
	util.AssertEqual(t, err, nil)

	out, err := manifest.Transform(transformers...)
	util.AssertEqual(t, err, nil)

	if args := webhookArgs(t, out); len(args) != 0 {
		t.Errorf("expected no webhook args without TLS config, got %v", args)
	}
}

func TestOpenShiftTransformers_WithTLSConfig(t *testing.T) {
	// MinVersion is carried in the dotted form produced by the TLS resolver (e.g.
	// "1.2"); the extension rewrites it to the "VersionTLS12" form the webhook flag
	// expects.
	tlsConfig := &openshiftcommon.TLSEnvVars{
		MinVersion:   "1.2",
		CipherSuites: "TLS_AES_128_GCM_SHA256",
	}
	const wantMinVersion = "VersionTLS12"
	oe := &openshiftExtension{resolvedTLSConfig: tlsConfig}
	transformers := oe.Transformers(&v1alpha1.ShipwrightBuild{})

	// A resolved TLS policy adds exactly one transformer (the webhook args).
	util.AssertEqual(t, len(transformers), baseTransformerCount+1)

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{webhookDeployment(t)}))
	util.AssertEqual(t, err, nil)

	out, err := manifest.Transform(transformers...)
	util.AssertEqual(t, err, nil)

	args := webhookArgs(t, out)
	if !containsSequence(args, tlsMinVersion, wantMinVersion) {
		t.Errorf("expected webhook args to contain %q %q, got %v", tlsMinVersion, wantMinVersion, args)
	}
	if !containsSequence(args, tlsCipherSuites, tlsConfig.CipherSuites) {
		t.Errorf("expected webhook args to contain %q %q, got %v", tlsCipherSuites, tlsConfig.CipherSuites, args)
	}
}

func TestOpenShiftTransformers_MetricsMTLSReady(t *testing.T) {
	oe := &openshiftExtension{metricsMTLSReady: true}
	transformers := oe.Transformers(&v1alpha1.ShipwrightBuild{})

	// Metrics over mTLS is currently disabled pending upstream support
	// (shipwright-io/build#2411): the metrics block is commented out, so
	// metricsMTLSReady contributes no additional transformers.
	util.AssertEqual(t, len(transformers), baseTransformerCount)

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{controllerService(t)}))
	util.AssertEqual(t, err, nil)

	out, err := manifest.Transform(transformers...)
	util.AssertEqual(t, err, nil)

	// The controller Service is NOT annotated for a metrics serving certificate.
	wantSecret := openshiftcommon.MetricsServingCertSecretName(shipwrightbuild.ControllerDeploymentName)
	if annotationValuePresent(out.Resources()[0], wantSecret) {
		t.Errorf("expected controller Service to carry no metrics serving-cert annotation while mTLS is disabled, annotations: %v",
			out.Resources()[0].GetAnnotations())
	}

	// The metrics port is left on plain HTTP (no rename to the HTTPS variant).
	svc := &corev1.Service{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(out.Resources()[0].Object, svc)
	util.AssertEqual(t, err, nil)
	if svc.Spec.Ports[0].Name != openshiftcommon.MetricsHTTPPort {
		t.Errorf("expected metrics port unchanged (%q), got %q", openshiftcommon.MetricsHTTPPort, svc.Spec.Ports[0].Name)
	}
}

func TestOpenShiftTransformers_TLSAndMetrics(t *testing.T) {
	oe := &openshiftExtension{
		resolvedTLSConfig: &openshiftcommon.TLSEnvVars{MinVersion: "1.2", CipherSuites: "TLS_AES_128_GCM_SHA256"},
		metricsMTLSReady:  true,
	}
	transformers := oe.Transformers(&v1alpha1.ShipwrightBuild{})

	// Metrics-mTLS is disabled, so only the base transformers plus the TLS webhook
	// args apply.
	util.AssertEqual(t, len(transformers), baseTransformerCount+1)
}

func TestOpenShiftExtensionGetPlatformData(t *testing.T) {
	oe := &openshiftExtension{}
	util.AssertEqual(t, oe.GetPlatformData(), "")
}

// fakeTektonConfigLister is a minimal openshiftcommon.TektonConfigLister used to
// drive PreReconcile without a real informer.
type fakeTektonConfigLister struct {
	tc *v1alpha1.TektonConfig
}

func (f fakeTektonConfigLister) Get(string) (*v1alpha1.TektonConfig, error) {
	return f.tc, nil
}

// TestOpenShiftExtensionPreReconcile verifies that PreReconcile resolves the
// central TLS config and leaves metrics-mTLS disabled (the metrics path is
// commented out pending shipwright-io/build#2411). With central TLS disabled on
// the TektonConfig, no cluster APIServer lookup is needed, so resolvedTLSConfig
// stays nil.
func TestOpenShiftExtensionPreReconcile(t *testing.T) {
	ctx := context.Background()
	disabled := false
	oe := &openshiftExtension{
		metricsMTLSReady: true, // must be forced back to false by PreReconcile
		tektonConfigLister: fakeTektonConfigLister{tc: &v1alpha1.TektonConfig{
			ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName},
			Spec: v1alpha1.TektonConfigSpec{
				Platforms: v1alpha1.Platforms{
					OpenShift: &v1alpha1.OpenShift{EnableCentralTLSConfig: &disabled},
				},
			},
		}},
	}

	util.AssertEqual(t, oe.PreReconcile(ctx, &v1alpha1.ShipwrightBuild{}), nil)

	// Central TLS is disabled, so no TLS policy is resolved.
	if oe.resolvedTLSConfig != nil {
		t.Errorf("expected resolvedTLSConfig to be nil when central TLS is disabled, got %+v", oe.resolvedTLSConfig)
	}
	// Metrics-mTLS is unconditionally disabled for now.
	util.AssertEqual(t, oe.metricsMTLSReady, false)
}

// TestOpenShiftExtensionFinalize verifies that Finalize cleans up the
// ShipwrightBuild NetworkPolicy custom InstallerSet (and only that set), and is a
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
				v1alpha1.CreatedByKey:     v1alpha1.KindShipwrightBuild,
				v1alpha1.InstallerSetType: npSetType,
			},
		},
	}
	// An unrelated main set that must survive Finalize.
	mainSet := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shipwright-main-deployment-xyz",
			Labels: map[string]string{
				v1alpha1.CreatedByKey:     v1alpha1.KindShipwrightBuild,
				v1alpha1.InstallerSetType: installersetclient.InstallerTypeMain,
			},
		},
	}

	fakeClient := fakeoperatorclient.NewSimpleClientset(npSet, mainSet)
	tisClient := fakeClient.OperatorV1alpha1().TektonInstallerSets()
	oe := &openshiftExtension{
		installerSetClient: installersetclient.NewInstallerSetClient(
			tisClient, "test-version", shipwrightbuild.OperandName, v1alpha1.KindShipwrightBuild, nil),
	}

	// Finalize removes the NetworkPolicy custom set and returns nil.
	util.AssertEqual(t, oe.Finalize(ctx, &v1alpha1.ShipwrightBuild{}), nil)

	list, err := tisClient.List(ctx, metav1.ListOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, len(list.Items), 1)
	util.AssertEqual(t, list.Items[0].GetName(), mainSet.GetName())

	// A second Finalize is a no-op (nothing left to clean up) and still succeeds.
	util.AssertEqual(t, oe.Finalize(ctx, &v1alpha1.ShipwrightBuild{}), nil)
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
				v1alpha1.CreatedByKey:     v1alpha1.KindShipwrightBuild,
				v1alpha1.InstallerSetType: npSetType,
			},
		},
	}
	mainSet := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shipwright-main-deployment-xyz",
			Labels: map[string]string{
				v1alpha1.CreatedByKey:     v1alpha1.KindShipwrightBuild,
				v1alpha1.InstallerSetType: installersetclient.InstallerTypeMain,
			},
		},
	}

	fakeClient := fakeoperatorclient.NewSimpleClientset(npSet, mainSet)
	tisClient := fakeClient.OperatorV1alpha1().TektonInstallerSets()
	oe := &openshiftExtension{
		installerSetClient: installersetclient.NewInstallerSetClient(
			tisClient, "test-version", shipwrightbuild.OperandName, v1alpha1.KindShipwrightBuild, nil),
	}

	sb := &v1alpha1.ShipwrightBuild{
		Spec: v1alpha1.ShipwrightBuildSpec{
			NetworkPolicy: v1alpha1.NetworkPolicyConfig{Disabled: true},
		},
	}

	util.AssertEqual(t, oe.PostReconcile(ctx, sb), nil)

	list, err := tisClient.List(ctx, metav1.ListOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, len(list.Items), 1)
	util.AssertEqual(t, list.Items[0].GetName(), mainSet.GetName())
}

func containsSequence(args []string, first, second string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == first && args[i+1] == second {
			return true
		}
	}
	return false
}

func annotationValuePresent(u unstructured.Unstructured, value string) bool {
	for _, v := range u.GetAnnotations() {
		if v == value {
			return true
		}
	}
	return false
}
