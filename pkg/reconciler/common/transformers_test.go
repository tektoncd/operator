/*
Copyright 2020 The Tekton Authors

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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"knative.dev/pkg/ptr"
)

func TestCommonTransformers(t *testing.T) {
	targetNamespace := "test-ns"
	component := &v1alpha1.TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-name",
		},
		Spec: v1alpha1.TektonPipelineSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: targetNamespace,
			},
		},
	}
	in := []unstructured.Unstructured{*NamespacedResource("test/v1", "TestCR", "another-ns", "test-resource")}
	manifest, err := mf.ManifestFrom(mf.Slice(in))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}
	if err := Transform(context.Background(), &manifest, component); err != nil {
		t.Fatalf("Failed to transform manifest: %v", err)
	}
	t.Log(manifest.Resources())
	resource := &manifest.Resources()[0]

	// Verify namespace is carried over.
	if got, want := resource.GetNamespace(), targetNamespace; got != want {
		t.Fatalf("GetNamespace() = %s, want %s", got, want)
	}

	// Transform with a platform extension
	ext := TestExtension("fubar")
	if err := Transform(context.Background(), &manifest, component, ext.Transformers(component)...); err != nil {
		t.Fatalf("Failed to transform manifest: %v", err)
	}
	resource = &manifest.Resources()[0]

	// Verify namespace is transformed
	if got, want := resource.GetNamespace(), string(ext); got != want {
		t.Fatalf("GetNamespace() = %s, want %s", got, want)
	}

	// Verify OwnerReference is set.
	if len(resource.GetOwnerReferences()) < 0 {
		t.Fatalf("len(GetOwnerReferences()) = 0, expected at least 1")
	}
	ownerRef := resource.GetOwnerReferences()[0]

	apiVersion, kind := component.GroupVersionKind().ToAPIVersionAndKind()
	wantOwnerRef := metav1.OwnerReference{
		APIVersion:         apiVersion,
		Kind:               kind,
		Name:               component.GetName(),
		Controller:         ptr.Bool(true),
		BlockOwnerDeletion: ptr.Bool(true),
	}

	if !cmp.Equal(ownerRef, wantOwnerRef) {
		t.Fatalf("Unexpected ownerRef: %s", cmp.Diff(ownerRef, wantOwnerRef))
	}
}
