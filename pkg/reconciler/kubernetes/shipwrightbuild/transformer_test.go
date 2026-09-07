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
	"github.com/tektoncd/operator/pkg/reconciler/common"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

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
					Containers: []corev1.Container{
						{Name: WebhookContainerName},
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

func namespaceResource(t *testing.T) unstructured.Unstructured {
	t.Helper()
	return unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]interface{}{"name": "openshift-builds"},
	}}
}

func TestFilterAndTransform(t *testing.T) {
	ctx := context.Background()
	r := &Reconciler{
		extension:        common.NoExtension(ctx),
		componentVersion: "v0.21.0",
	}
	sb := &v1alpha1.ShipwrightBuild{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ShipwrightBuildResourceName},
		Spec: v1alpha1.ShipwrightBuildSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "openshift-builds"},
		},
	}

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{namespaceResource(t), webhookDeployment(t)}))
	util.AssertEqual(t, err, nil)

	out, err := FilterAndTransform(r)(ctx, &manifest, sb)
	util.AssertEqual(t, err, nil)

	// The Namespace is filtered out; only the Deployment remains.
	for _, res := range out.Resources() {
		if res.GetKind() == "Namespace" {
			t.Errorf("expected Namespace to be filtered out, but it is present")
		}
	}

	// The webhook container receives the -version flag with the component version.
	d := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(out.Resources()[0].Object, d)
	util.AssertEqual(t, err, nil)

	var args []string
	for _, c := range d.Spec.Template.Spec.Containers {
		if c.Name == WebhookContainerName {
			args = c.Args
		}
	}

	if !containsSequence(args, WebhookVersionFlag, "v0.21.0") {
		t.Errorf("expected webhook args to contain %q %q, got %v", WebhookVersionFlag, "v0.21.0", args)
	}
}

func TestNopFilterAndTransform(t *testing.T) {
	ctx := context.Background()
	sb := &v1alpha1.ShipwrightBuild{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ShipwrightBuildResourceName},
	}

	in := []unstructured.Unstructured{namespaceResource(t), webhookDeployment(t)}
	manifest, err := mf.ManifestFrom(mf.Slice(in))
	util.AssertEqual(t, err, nil)

	out, err := NopFilterAndTransform()(ctx, &manifest, sb)
	util.AssertEqual(t, err, nil)

	// NilFilterAndTransform returns the manifest untouched.
	util.AssertEqual(t, len(out.Resources()), len(in))
}

func containsSequence(args []string, first, second string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == first && args[i+1] == second {
			return true
		}
	}
	return false
}
