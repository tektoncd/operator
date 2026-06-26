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

package manualapprovalgate

import (
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// makeMAGDeployment returns an unstructured MAG Deployment for transformer tests.
func makeMAGDeployment(t *testing.T, deploymentName, containerName string) unstructured.Unstructured {
	t.Helper()

	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: "openshift-pipelines",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: containerName},
					},
				},
			},
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
	if err != nil {
		t.Fatalf("failed to convert deployment to unstructured: %v", err)
	}
	u := unstructured.Unstructured{Object: obj}
	u.SetKind("Deployment")
	u.SetAPIVersion("apps/v1")
	return u
}

func makeMAGWebhookDeployment(t *testing.T) unstructured.Unstructured {
	t.Helper()
	return makeMAGDeployment(t, magWebhookDeployment, magWebhookContainerName)
}

func TestMAGTransformers_NoTLSConfig(t *testing.T) {
	ext := &openshiftExtension{
		resolvedTLSConfig: nil,
	}

	transformers := ext.Transformers(&v1alpha1.ManualApprovalGate{})

	u := makeMAGWebhookDeployment(t)
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{u}))
	if err != nil {
		t.Fatalf("failed to build manifest: %v", err)
	}

	transformed, err := manifest.Transform(transformers...)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}

	d := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(transformed.Resources()[0].Object, d); err != nil {
		t.Fatalf("failed to convert back: %v", err)
	}
	for _, c := range d.Spec.Template.Spec.Containers {
		if c.Name != magWebhookContainerName {
			continue
		}
		for _, e := range c.Env {
			if e.Name == occommon.WebhookEnvVarPrefix+occommon.TLSMinVersionEnvVar ||
				e.Name == occommon.WebhookEnvVarPrefix+occommon.TLSCipherSuitesEnvVar {
				t.Errorf("unexpected TLS env var %s set when resolvedTLSConfig is nil", e.Name)
			}
		}
	}
}

func TestMAGTransformers_WithTLSConfig_InjectsEnvVarsIntoWebhook(t *testing.T) {
	tlsConfig := &occommon.TLSEnvVars{
		MinVersion:   "1.2",
		CipherSuites: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_AES_128_GCM_SHA256",
	}
	// MAG webhook uses the Knative webhook framework → reads WEBHOOK_TLS_* env vars.
	assertMAGTLSInjected(t, &openshiftExtension{resolvedTLSConfig: tlsConfig}, magWebhookDeployment, magWebhookContainerName, occommon.WebhookEnvVarPrefix, tlsConfig)
}

func TestMAGTransformers_WithTLSConfig_DoesNotInjectIntoUnknownDeployment(t *testing.T) {
	tlsConfig := &occommon.TLSEnvVars{
		MinVersion:   "1.3",
		CipherSuites: "TLS_AES_128_GCM_SHA256",
	}
	ext := &openshiftExtension{resolvedTLSConfig: tlsConfig}
	transformers := ext.Transformers(&v1alpha1.ManualApprovalGate{})

	// An unrelated deployment must not receive TLS env vars.
	u := makeMAGDeployment(t, "some-other-deployment", "some-container")
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{u}))
	if err != nil {
		t.Fatalf("failed to build manifest: %v", err)
	}
	transformed, err := manifest.Transform(transformers...)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}

	result := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(transformed.Resources()[0].Object, result); err != nil {
		t.Fatalf("failed to convert back: %v", err)
	}
	for _, c := range result.Spec.Template.Spec.Containers {
		for _, e := range c.Env {
			if e.Name == occommon.TLSMinVersionEnvVar || e.Name == occommon.TLSCipherSuitesEnvVar ||
				e.Name == occommon.WebhookEnvVarPrefix+occommon.TLSMinVersionEnvVar ||
				e.Name == occommon.WebhookEnvVarPrefix+occommon.TLSCipherSuitesEnvVar {
				t.Errorf("unexpected TLS env var %s injected into unrelated deployment", e.Name)
			}
		}
	}
}

// assertMAGTLSInjected runs the Transformers against a single-resource manifest and
// checks that TLS env vars are present in the named container.
func assertMAGTLSInjected(t *testing.T, ext *openshiftExtension, deploymentName, containerName, envVarPrefix string, tlsConfig *occommon.TLSEnvVars) {
	t.Helper()

	u := makeMAGDeployment(t, deploymentName, containerName)
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{u}))
	if err != nil {
		t.Fatalf("failed to build manifest: %v", err)
	}

	transformed, err := manifest.Transform(ext.Transformers(&v1alpha1.ManualApprovalGate{})...)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}

	d := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(transformed.Resources()[0].Object, d); err != nil {
		t.Fatalf("failed to convert back: %v", err)
	}

	envMap := map[string]string{}
	for _, c := range d.Spec.Template.Spec.Containers {
		if c.Name != containerName {
			continue
		}
		for _, e := range c.Env {
			envMap[e.Name] = e.Value
		}
	}

	minVersionKey := envVarPrefix + occommon.TLSMinVersionEnvVar
	cipherSuitesKey := envVarPrefix + occommon.TLSCipherSuitesEnvVar

	if got := envMap[minVersionKey]; got != tlsConfig.MinVersion {
		t.Errorf("[%s/%s] %s = %q, want %q", deploymentName, containerName, minVersionKey, got, tlsConfig.MinVersion)
	}
	if got := envMap[cipherSuitesKey]; got != tlsConfig.CipherSuites {
		t.Errorf("[%s/%s] %s = %q, want %q", deploymentName, containerName, cipherSuitesKey, got, tlsConfig.CipherSuites)
	}
}
