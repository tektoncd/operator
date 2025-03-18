/*
Copyright 2025 The Tekton Authors

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
package common

import (
	"path"
	"testing"

	mf "github.com/manifestival/manifestival"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDeploymentEnvVars(t *testing.T) {
	t.Run("inject environment variables into deployments", func(t *testing.T) {
		envVars := []corev1.EnvVar{
			{
				Name:  "KUBERNETES_MIN_VERSION",
				Value: "v1.0.0",
			},
			{
				Name: "SECRET_VAR",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-secret",
						},
						Key: "secret-key",
					},
				},
			},
		}

		testData := path.Join("testdata", "test-inject-envvar.yaml")
		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)

		newManifest, err := manifest.Transform(deploymentEnvVars(envVars))
		assertNoError(t, err)

		assertDeploymentHasEnvVar(t, newManifest.Resources(), "controller", "KUBERNETES_MIN_VERSION", "v1.0.0")
		assertDeploymentHasSecretEnvVar(t, newManifest.Resources(), "controller", "SECRET_VAR", "test-secret", "secret-key")
	})

	t.Run("update existing environment variables", func(t *testing.T) {
		envVars := []corev1.EnvVar{
			{
				Name:  "EXISTING_VAR",
				Value: "value",
			},
		}

		testData := path.Join("testdata", "test-replace-envvar.yaml")
		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)

		newManifest, err := manifest.Transform(deploymentEnvVars(envVars))
		assertNoError(t, err)

		assertDeploymentHasEnvVar(t, newManifest.Resources(), "controller", "EXISTING_VAR", "value")
	})
}

func assertDeploymentHasEnvVar(t *testing.T, resources []unstructured.Unstructured, deploymentName, envName, expectedValue string) {
	t.Helper()
	for _, resource := range resources {
		if resource.GetKind() != "Deployment" {
			continue
		}

		deployment := deploymentFor(t, resource)
		containers := deployment.Spec.Template.Spec.Containers

		for _, container := range containers {
			for _, env := range container.Env {
				if env.Name == envName {
					if env.Value != expectedValue {
						t.Errorf("assertion failed; unexpected env var value: expected %s and got %s for env var %s in container %s",
							expectedValue, env.Value, envName, container.Name)
					}
					return
				}
			}
		}

		t.Errorf("Environment variable %s not found in any container of deployment %s", envName, deploymentName)
	}
}

func assertDeploymentHasSecretEnvVar(t *testing.T, resources []unstructured.Unstructured, deploymentName, envName, secretName, secretKey string) {
	t.Helper()
	for _, resource := range resources {
		if resource.GetKind() != "Deployment" {
			continue
		}

		deployment := deploymentFor(t, resource)
		containers := deployment.Spec.Template.Spec.Containers

		for _, container := range containers {
			for _, env := range container.Env {
				if env.Name == envName {
					if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
						t.Errorf("assertion failed; env var %s in container %s does not have SecretKeyRef", envName, container.Name)
						return
					}

					if env.ValueFrom.SecretKeyRef.Name != secretName {
						t.Errorf("assertion failed; unexpected secret name: expected %s and got %s for env var %s in container %s",
							secretName, env.ValueFrom.SecretKeyRef.Name, envName, container.Name)
					}

					if env.ValueFrom.SecretKeyRef.Key != secretKey {
						t.Errorf("assertion failed; unexpected secret key: expected %s and got %s for env var %s in container %s",
							secretKey, env.ValueFrom.SecretKeyRef.Key, envName, container.Name)
					}

					return
				}
			}
		}
		t.Errorf("Environment variable %s not found in any container of deployment %s", envName, deploymentName)
	}
}
