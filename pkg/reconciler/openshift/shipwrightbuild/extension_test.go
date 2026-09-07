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
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	kshipwrightbuild "github.com/tektoncd/operator/pkg/reconciler/kubernetes/shipwrightbuild"
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
			Name:      kshipwrightbuild.WebhookDeploymentName,
			Namespace: "openshift-builds",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: kshipwrightbuild.WebhookContainerName},
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
			Name:      kshipwrightbuild.ControllerDeploymentName,
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
		if res.GetKind() != "Deployment" || res.GetName() != kshipwrightbuild.WebhookDeploymentName {
			continue
		}
		d := &appsv1.Deployment{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.Object, d); err != nil {
			t.Fatalf("failed to convert back: %v", err)
		}
		for _, c := range d.Spec.Template.Spec.Containers {
			if c.Name == kshipwrightbuild.WebhookContainerName {
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
	if !containsSequence(args, tlsMinVersionFlag, wantMinVersion) {
		t.Errorf("expected webhook args to contain %q %q, got %v", tlsMinVersionFlag, wantMinVersion, args)
	}
	if !containsSequence(args, tlsCipherSuites, tlsConfig.CipherSuites) {
		t.Errorf("expected webhook args to contain %q %q, got %v", tlsCipherSuites, tlsConfig.CipherSuites, args)
	}
}

func TestOpenShiftTransformers_MetricsMTLSReady(t *testing.T) {
	oe := &openshiftExtension{metricsMTLSReady: true}
	transformers := oe.Transformers(&v1alpha1.ShipwrightBuild{})

	// Metrics-mTLS readiness adds three transformers (serving-cert annotation,
	// port rename, and the mTLS deployment patch).
	util.AssertEqual(t, len(transformers), baseTransformerCount+3)

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{controllerService(t)}))
	util.AssertEqual(t, err, nil)

	out, err := manifest.Transform(transformers...)
	util.AssertEqual(t, err, nil)

	// The controller Service is annotated so OpenShift provisions its metrics
	// serving certificate.
	wantSecret := openshiftcommon.MetricsServingCertSecretName(kshipwrightbuild.ControllerDeploymentName)
	if !annotationValuePresent(out.Resources()[0], wantSecret) {
		t.Errorf("expected controller Service to carry the metrics serving-cert secret %q, annotations: %v",
			wantSecret, out.Resources()[0].GetAnnotations())
	}

	// The plain-HTTP metrics port is renamed to the HTTPS variant.
	svc := &corev1.Service{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(out.Resources()[0].Object, svc)
	util.AssertEqual(t, err, nil)
	if svc.Spec.Ports[0].Name != openshiftcommon.MetricsHTTPSPort {
		t.Errorf("expected metrics port renamed to %q, got %q", openshiftcommon.MetricsHTTPSPort, svc.Spec.Ports[0].Name)
	}
}

func TestOpenShiftTransformers_TLSAndMetrics(t *testing.T) {
	oe := &openshiftExtension{
		resolvedTLSConfig: &openshiftcommon.TLSEnvVars{MinVersion: "1.2", CipherSuites: "TLS_AES_128_GCM_SHA256"},
		metricsMTLSReady:  true,
	}
	transformers := oe.Transformers(&v1alpha1.ShipwrightBuild{})

	// Both branches contribute: base + 3 metrics + 1 TLS.
	util.AssertEqual(t, len(transformers), baseTransformerCount+4)
}

func TestOpenShiftExtensionNoOpHooks(t *testing.T) {
	oe := &openshiftExtension{}
	ctx := context.Background()
	sb := &v1alpha1.ShipwrightBuild{}

	util.AssertEqual(t, oe.PostReconcile(ctx, sb), nil)
	util.AssertEqual(t, oe.Finalize(ctx, sb), nil)
	util.AssertEqual(t, oe.GetPlatformData(), "")
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
