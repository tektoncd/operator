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

package tektoninstallerset

import (
	"strings"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/manifestival/manifestival/fake"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestEnsureCRDs_UpdateLargeSchema(t *testing.T) {
	for _, size := range []int{100, validation.TotalAnnotationSizeLimitB + 1} {
		t.Run(stringSizeName(size), func(t *testing.T) {
			existing := namespacedResource("apiextensions.k8s.io/v1", "CustomResourceDefinition", "", "buildruns.shipwright.io")
			existing.SetUID("original-crd")
			existing.SetResourceVersion("17")
			existing.SetFinalizers([]string{"customresourcecleanup.apiextensions.k8s.io"})
			existing.SetAnnotations(map[string]string{
				corev1.LastAppliedConfigAnnotation:          "{}",
				"service.beta.openshift.io/inject-cabundle": "true",
			})
			existing.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "operator.openshift.io/v1alpha1", Kind: "OpenShiftBuild", Name: "cluster", UID: "legacy-parent"}})
			assert.NilError(t, unstructured.SetNestedSlice(existing.Object, []interface{}{"v1alpha1"}, "status", "storedVersions"))
			desired := namespacedResource(existing.GetAPIVersion(), existing.GetKind(), "", existing.GetName())
			assert.NilError(t, unstructured.SetNestedField(desired.Object, strings.Repeat("x", size), "spec", "schema"))
			client := fake.New(&existing)
			manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{desired}))
			assert.NilError(t, err)
			core, logs := observer.New(zap.InfoLevel)
			installer := NewInstaller(&manifest, client, k8sfake.NewSimpleClientset(), zap.New(core).Sugar())
			assert.NilError(t, installer.EnsureCRDs("update"))
			actual, err := client.Get(&existing)
			assert.NilError(t, err)
			assert.NilError(t, validation.ValidateAnnotationsSize(actual.GetAnnotations()))
			assert.Equal(t, actual.GetUID(), existing.GetUID())
			assert.Equal(t, actual.GetResourceVersion(), existing.GetResourceVersion())
			assert.DeepEqual(t, actual.GetFinalizers(), existing.GetFinalizers())
			assert.DeepEqual(t, actual.GetOwnerReferences(), existing.GetOwnerReferences())
			assert.DeepEqual(t, actual.Object["status"], existing.Object["status"])
			assert.DeepEqual(t, actual.Object["spec"], desired.Object["spec"])
			assert.Equal(t, actual.GetAnnotations()["service.beta.openshift.io/inject-cabundle"], "true")
			expectedHash, err := hash.Compute(desired.Object)
			assert.NilError(t, err)
			assert.Equal(t, actual.GetAnnotations()[v1alpha1.LastAppliedHashKey], expectedHash)
			_, hasLastApplied := actual.GetAnnotations()[corev1.LastAppliedConfigAnnotation]
			assert.Equal(t, hasLastApplied, size < validation.TotalAnnotationSizeLimitB)
			removals := logs.FilterMessageSnippet("Removing " + corev1.LastAppliedConfigAnnotation + " annotation from CRD " + existing.GetName())
			expectedLogs := 0
			if !hasLastApplied {
				expectedLogs = 1
			}
			assert.Equal(t, removals.Len(), expectedLogs)
		})
	}
}

func stringSizeName(size int) string {
	if size > validation.TotalAnnotationSizeLimitB {
		return "oversized"
	}
	return "small"
}
