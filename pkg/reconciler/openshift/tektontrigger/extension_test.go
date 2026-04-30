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

package tektontrigger

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

// makeTriggersWebhookDeployment returns an unstructured triggers webhook Deployment for transformer tests.
func makeTriggersWebhookDeployment(t *testing.T) unstructured.Unstructured {
	t.Helper()

	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tektonTriggersWebhookDeployment,
			Namespace: "openshift-pipelines",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: webhookContainerName},
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

func TestTriggersTransformers_NoTLSConfig(t *testing.T) {
	ext := &openshiftExtension{
		resolvedTLSConfig: nil,
	}

	transformers := ext.Transformers(&v1alpha1.TektonTrigger{})

	u := makeTriggersWebhookDeployment(t)
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
		if c.Name != webhookContainerName {
			continue
		}
		for _, e := range c.Env {
			if e.Name == occommon.TLSMinVersionEnvVar || e.Name == occommon.TLSCipherSuitesEnvVar {
				t.Errorf("unexpected TLS env var %s set when resolvedTLSConfig is nil", e.Name)
			}
		}
	}
}

func TestTriggersTransformers_WithTLSConfig_InjectsEnvVarsIntoWebhook(t *testing.T) {
	tlsConfig := &occommon.TLSEnvVars{
		MinVersion:   "1.2",
		CipherSuites: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_AES_128_GCM_SHA256",
	}
	ext := &openshiftExtension{
		resolvedTLSConfig: tlsConfig,
	}

	transformers := ext.Transformers(&v1alpha1.TektonTrigger{})

	u := makeTriggersWebhookDeployment(t)
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

	envMap := map[string]string{}
	for _, c := range d.Spec.Template.Spec.Containers {
		if c.Name != webhookContainerName {
			continue
		}
		for _, e := range c.Env {
			envMap[e.Name] = e.Value
		}
	}

	if got := envMap[occommon.TLSMinVersionEnvVar]; got != tlsConfig.MinVersion {
		t.Errorf("%s = %q, want %q", occommon.TLSMinVersionEnvVar, got, tlsConfig.MinVersion)
	}
	if got := envMap[occommon.TLSCipherSuitesEnvVar]; got != tlsConfig.CipherSuites {
		t.Errorf("%s = %q, want %q", occommon.TLSCipherSuitesEnvVar, got, tlsConfig.CipherSuites)
	}
}

func TestTriggersTransformers_WithTLSConfig_DoesNotInjectIntoOtherDeployments(t *testing.T) {
	tlsConfig := &occommon.TLSEnvVars{
		MinVersion:   "1.3",
		CipherSuites: "TLS_AES_128_GCM_SHA256",
	}
	ext := &openshiftExtension{
		resolvedTLSConfig: tlsConfig,
	}

	transformers := ext.Transformers(&v1alpha1.TektonTrigger{})

	// Use a different deployment name — TLS env vars must NOT be injected.
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-triggers-controller",
			Namespace: "openshift-pipelines",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "controller"},
					},
				},
			},
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
	if err != nil {
		t.Fatalf("failed to convert: %v", err)
	}
	u := unstructured.Unstructured{Object: obj}
	u.SetKind("Deployment")
	u.SetAPIVersion("apps/v1")

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
			if e.Name == occommon.TLSMinVersionEnvVar || e.Name == occommon.TLSCipherSuitesEnvVar {
				t.Errorf("unexpected TLS env var %s injected into non-webhook deployment", e.Name)
			}
		}
	}
}
