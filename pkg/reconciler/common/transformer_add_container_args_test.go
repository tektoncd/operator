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

package common

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// deploymentManifest builds a single-resource manifest holding a Deployment
// with one container that carries the given args.
func deploymentManifest(t *testing.T, deploymentName, containerName string, args []string) mf.Manifest {
	t.Helper()
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       KindDeployment,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: containerName,
							Args: args,
						},
					},
				},
			},
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deployment)
	assert.NilError(t, err)
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{{Object: obj}}))
	assert.NilError(t, err)
	return manifest
}

func containerArgs(t *testing.T, manifest mf.Manifest) []string {
	t.Helper()
	got := &appsv1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, got)
	assert.NilError(t, err)
	return got.Spec.Template.Spec.Containers[0].Args
}

func TestAddContainerArgs_Append(t *testing.T) {
	manifest := deploymentManifest(t, "webhook", "webhook", []string{"--existing"})

	out, err := manifest.Transform(AddContainerArgs("webhook", "webhook", []string{"-version", "v0.21.0"}, false))
	assert.NilError(t, err)

	want := []string{"--existing", "-version", "v0.21.0"}
	if d := cmp.Diff(want, containerArgs(t, out)); d != "" {
		t.Errorf("unexpected args (-want +got):\n%s", d)
	}
}

func TestAddContainerArgs_Replace(t *testing.T) {
	manifest := deploymentManifest(t, "webhook", "webhook", []string{"--existing"})

	out, err := manifest.Transform(AddContainerArgs("webhook", "webhook", []string{"-version", "v0.21.0"}, true))
	assert.NilError(t, err)

	want := []string{"-version", "v0.21.0"}
	if d := cmp.Diff(want, containerArgs(t, out)); d != "" {
		t.Errorf("unexpected args (-want +got):\n%s", d)
	}
}

func TestAddContainerArgs_ContainerNotFound(t *testing.T) {
	manifest := deploymentManifest(t, "webhook", "controller", []string{"--existing"})

	out, err := manifest.Transform(AddContainerArgs("webhook", "webhook", []string{"-version", "v0.21.0"}, false))
	assert.NilError(t, err)

	// The container name does not match, so args are left untouched.
	want := []string{"--existing"}
	if d := cmp.Diff(want, containerArgs(t, out)); d != "" {
		t.Errorf("unexpected args (-want +got):\n%s", d)
	}
}

func TestAddContainerArgs_DeploymentNameMismatch(t *testing.T) {
	manifest := deploymentManifest(t, "controller", "webhook", []string{"--existing"})

	out, err := manifest.Transform(AddContainerArgs("webhook", "webhook", []string{"-version", "v0.21.0"}, false))
	assert.NilError(t, err)

	// The deployment name does not match, so args are left untouched.
	want := []string{"--existing"}
	if d := cmp.Diff(want, containerArgs(t, out)); d != "" {
		t.Errorf("unexpected args (-want +got):\n%s", d)
	}
}

func TestAddContainerArgs_EmptyDeploymentNameMatchesAny(t *testing.T) {
	manifest := deploymentManifest(t, "any-deployment", "webhook", []string{"--existing"})

	out, err := manifest.Transform(AddContainerArgs("", "webhook", []string{"-version", "v0.21.0"}, false))
	assert.NilError(t, err)

	want := []string{"--existing", "-version", "v0.21.0"}
	if d := cmp.Diff(want, containerArgs(t, out)); d != "" {
		t.Errorf("unexpected args (-want +got):\n%s", d)
	}
}

func TestAddContainerArgs_NonDeploymentKind(t *testing.T) {
	svc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name": "webhook",
		},
	}}
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{*svc}))
	assert.NilError(t, err)

	out, err := manifest.Transform(AddContainerArgs("webhook", "webhook", []string{"-version"}, false))
	assert.NilError(t, err)

	// Non-Deployment resources are returned unchanged.
	if d := cmp.Diff(svc.Object, out.Resources()[0].Object); d != "" {
		t.Errorf("non-Deployment resource was modified (-want +got):\n%s", d)
	}
}
