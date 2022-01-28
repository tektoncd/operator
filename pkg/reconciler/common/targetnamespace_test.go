/*
Copyright 2022 The Tekton Authors

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
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeClient struct {
	err            error
	getErr         error
	createErr      error
	resourcesExist bool
	creates        []unstructured.Unstructured
	deletes        []unstructured.Unstructured
}

var configMap = namespacedResource("v1", "ConfigMap", "test-ns", "operators-info")

func (f *fakeClient) Get(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var resource *unstructured.Unstructured
	if f.resourcesExist {
		resource = &unstructured.Unstructured{}
	}
	return resource, f.getErr
}

func (f *fakeClient) Create(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	obj.SetAnnotations(nil) // Deleting the extra annotation. Irrelevant for the test.
	f.creates = append(f.creates, *obj)
	return f.createErr
}

func (f *fakeClient) Delete(obj *unstructured.Unstructured, options ...mf.DeleteOption) error {
	f.deletes = append(f.deletes, *obj)
	return f.err
}

func (f *fakeClient) Update(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	return f.err
}

func TestCreateTargetNamespace(t *testing.T) {
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
	fakeClientset := fake.NewSimpleClientset()

	err := CreateTargetNamespace(context.Background(), nil, component, fakeClientset)
	assert.Equal(t, err, nil)

	ns, err := fakeClientset.CoreV1().Namespaces().Get(context.Background(), targetNamespace, metav1.GetOptions{})
	assert.Equal(t, err, nil)
	assert.Equal(t, ns.ObjectMeta.Name, targetNamespace)
}

func TestTargetNamespaceNotFound(t *testing.T) {
	component := &v1alpha1.TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-name",
		},
		Spec: v1alpha1.TektonPipelineSpec{
			CommonSpec: v1alpha1.CommonSpec{},
		},
	}
	fakeClientset := fake.NewSimpleClientset()

	err := CreateTargetNamespace(context.Background(), nil, component, fakeClientset)
	assert.Equal(t, err, nil)

	_, err = fakeClientset.CoreV1().Namespaces().Get(context.Background(), "foo", metav1.GetOptions{})
	assert.Equal(t, err.Error(), `namespaces "foo" not found`)
}

func TestOperatorVersionCreateConfigMap(t *testing.T) {
	os.Setenv(KoEnvKey, "testdata/kodata")
	defer os.Unsetenv(KoEnvKey)

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
	fakeClientset := fake.NewSimpleClientset()

	err := CreateTargetNamespace(context.Background(), nil, component, fakeClientset)
	assert.Equal(t, err, nil)

	var manifest mf.Manifest

	client := &fakeClient{}
	manifest, err = mf.ManifestFrom(mf.Slice(manifest.Resources()), mf.UseClient(client))
	assert.Equal(t, err, nil)

	err = CreateOperatorVersionConfigMap(manifest, component)
	assert.Equal(t, err, nil)

	want := []unstructured.Unstructured{configMap}

	if len(want) != len(client.creates) {
		t.Fatalf("Unexpected creates: %s", fmt.Sprintf("(-got, +want): %s", cmp.Diff(client.creates, want)))
	}
}
