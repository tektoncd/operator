/*
Copyright 2019 The Tekton Authors
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

//import (
//	"context"
//	"testing"
//
//	"github.com/google/go-cmp/cmp"
//	mf "github.com/manifestival/manifestival"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
//	"knative.dev/operator/pkg/apis/operator/v1alpha1"
//	"knative.dev/pkg/ptr"
//)
//
//func TestCommonTransformers(t *testing.T) {
//	component := &v1alpha1.KnativeEventing{
//		ObjectMeta: metav1.ObjectMeta{
//			Namespace: "test-ns",
//			Name:      "test-name",
//		},
//	}
//	in := []unstructured.Unstructured{*NamespacedResource("test/v1", "TestCR", "another-ns", "test-resource")}
//	manifest, err := mf.ManifestFrom(mf.Slice(in))
//	if err != nil {
//		t.Fatalf("Failed to generate manifest: %v", err)
//	}
//	if err := Transform(context.Background(), &manifest, component); err != nil {
//		t.Fatalf("Failed to transform manifest: %v", err)
//	}
//	resource := &manifest.Resources()[0]
//
//	// Verify namespace is carried over.
//	if got, want := resource.GetNamespace(), component.GetNamespace(); got != want {
//		t.Fatalf("GetNamespace() = %s, want %s", got, want)
//	}
//
//	// Transform with a platform extension
//	ext := TestExtension("fubar")
//	if err := Transform(context.Background(), &manifest, component, ext.Transformers(component)...); err != nil {
//		t.Fatalf("Failed to transform manifest: %v", err)
//	}
//	resource = &manifest.Resources()[0]
//
//	// Verify namespace is transformed
//	if got, want := resource.GetNamespace(), string(ext); got != want {
//		t.Fatalf("GetNamespace() = %s, want %s", got, want)
//	}
//
//	// Verify OwnerReference is set.
//	if len(resource.GetOwnerReferences()) < 0 {
//		t.Fatalf("len(GetOwnerReferences()) = 0, expected at least 1")
//	}
//	ownerRef := resource.GetOwnerReferences()[0]
//
//	apiVersion, kind := component.GroupVersionKind().ToAPIVersionAndKind()
//	wantOwnerRef := metav1.OwnerReference{
//		APIVersion:         apiVersion,
//		Kind:               kind,
//		Name:               component.GetName(),
//		Controller:         ptr.Bool(true),
//		BlockOwnerDeletion: ptr.Bool(true),
//	}
//
//	if !cmp.Equal(ownerRef, wantOwnerRef) {
//		t.Fatalf("Unexpected ownerRef: %s", cmp.Diff(ownerRef, wantOwnerRef))
//	}
//}
