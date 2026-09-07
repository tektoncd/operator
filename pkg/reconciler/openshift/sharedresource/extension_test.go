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
	"github.com/tektoncd/operator/pkg/reconciler/common"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func controllerDeployment(t *testing.T) unstructured.Unstructured {
	t.Helper()
	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ControllerDeploymentName,
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

func TestOpenShiftExtensionTransformers(t *testing.T) {
	oe := &openshiftExtension{}
	// The Shared Resource extension always applies the two SCC transformers.
	transformers := oe.Transformers(&v1alpha1.SharedResource{})
	util.AssertEqual(t, len(transformers), 2)
}

func TestOpenShiftExtensionNoOpHooks(t *testing.T) {
	oe := &openshiftExtension{}
	ctx := context.Background()
	sr := &v1alpha1.SharedResource{}

	util.AssertEqual(t, oe.PreReconcile(ctx, sr), nil)
	util.AssertEqual(t, oe.PostReconcile(ctx, sr), nil)
	util.AssertEqual(t, oe.Finalize(ctx, sr), nil)
	util.AssertEqual(t, oe.GetPlatformData(), "")
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
