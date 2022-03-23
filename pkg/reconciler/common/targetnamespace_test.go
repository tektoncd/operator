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
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

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
