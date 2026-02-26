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

package tektonmulticlusterproxyaae

import (
	"context"
	"path"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestOpenShiftExtensionTransformers(t *testing.T) {
	t.Run("sets WORKERS_SECRET_NAMESPACE to openshift-kueue-operator", func(t *testing.T) {
		manifest := loadTestManifest(t)
		cr := &v1alpha1.TektonMulticlusterProxyAAE{}

		transformers := openshiftExtension{}.Transformers(cr)
		newManifest, err := manifest.Transform(transformers...)
		if err != nil {
			t.Fatalf("unexpected error applying transformers: %v", err)
		}

		assertDeploymentEnvVar(t, newManifest, "WORKERS_SECRET_NAMESPACE", "openshift-kueue-operator")
	})

	// spec.options.deployments allows users to override env vars set by the extension.
	// Since ExecuteAdditionalOptionsTransformer runs after the extension transformers,
	// the options value takes precedence.
	t.Run("spec.options can override WORKERS_SECRET_NAMESPACE", func(t *testing.T) {
		manifest := loadTestManifest(t)
		cr := &v1alpha1.TektonMulticlusterProxyAAE{
			Spec: v1alpha1.TektonMulticlusterProxyAAESpec{
				MulticlusterProxyAAEOptions: v1alpha1.MulticlusterProxyAAEOptions{
					Options: v1alpha1.AdditionalOptions{
						Deployments: map[string]appsv1.Deployment{
							"proxy-aae": {
								Spec: appsv1.DeploymentSpec{
									Template: corev1.PodTemplateSpec{
										Spec: corev1.PodSpec{
											Containers: []corev1.Container{
												{
													Name: "proxy-aae",
													Env: []corev1.EnvVar{
														{
															Name:  "WORKERS_SECRET_NAMESPACE",
															Value: "custom-namespace",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		// Apply extension transformers first — sets openshift-kueue-operator.
		transformers := openshiftExtension{}.Transformers(cr)
		newManifest, err := manifest.Transform(transformers...)
		if err != nil {
			t.Fatalf("unexpected error applying transformers: %v", err)
		}

		// Apply options transformer — overrides with the user-supplied value.
		if err := common.ExecuteAdditionalOptionsTransformer(
			context.Background(), &newManifest,
			cr.Spec.GetTargetNamespace(), cr.Spec.Options,
		); err != nil {
			t.Fatalf("unexpected error from options transformer: %v", err)
		}

		assertDeploymentEnvVar(t, newManifest, "WORKERS_SECRET_NAMESPACE", "custom-namespace")
	})
}

func loadTestManifest(t *testing.T) mf.Manifest {
	t.Helper()
	manifest, err := mf.ManifestFrom(mf.Recursive(path.Join("testdata", "proxy-aae-deployment.yaml")))
	if err != nil {
		t.Fatalf("failed to load test manifest: %v", err)
	}
	return manifest
}

func assertDeploymentEnvVar(t *testing.T, manifest mf.Manifest, envName, expectedValue string) {
	t.Helper()
	for _, resource := range manifest.Resources() {
		if resource.GetKind() != "Deployment" {
			continue
		}

		d := &appsv1.Deployment{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, d); err != nil {
			t.Fatalf("failed to convert resource to Deployment: %v", err)
		}

		for _, container := range d.Spec.Template.Spec.Containers {
			for _, env := range container.Env {
				if env.Name == envName {
					if env.Value != expectedValue {
						t.Errorf("deployment %q container %q: env %q = %q, want %q",
							d.Name, container.Name, envName, env.Value, expectedValue)
					}
					return
				}
			}
		}
		t.Errorf("deployment %q: env var %q not found in any container", d.Name, envName)
	}
}
