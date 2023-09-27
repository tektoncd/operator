/*
Copyright 2023 The Tekton Authors

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

package upgrade

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apixFake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	"knative.dev/pkg/logging"
)

func TestMigrateStorageVersion(t *testing.T) {
	fakeKind := schema.GroupKind{
		Kind:  "Fake",
		Group: "group.dev",
	}

	fakeGroup := schema.GroupResource{
		Resource: "fakes",
		Group:    "group.dev",
	}

	fakeCRD := &apix.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeGroup.String(),
		},
		Spec: apix.CustomResourceDefinitionSpec{
			Group: fakeKind.Group,
			Versions: []apix.CustomResourceDefinitionVersion{
				{Name: "v1alpha1", Served: true, Storage: false},
				{Name: "v1beta1", Served: true, Storage: false},
				{Name: "v1", Served: true, Storage: true},
			},
		},
		Status: apix.CustomResourceDefinitionStatus{
			StoredVersions: []string{
				"v1alpha1",
				"v1beta1",
				"v1",
			},
		},
	}

	ctx := context.TODO()
	resources := []runtime.Object{
		fakeCrdResource("resource-1", "group.dev/v1alpha1"),
		fakeCrdResource("resource-2", "group.dev/v1beta1"),
		fakeCrdResource("resource-3", "group.dev/v1beta2"),
		fakeCrdResource("resource-4", "group.dev/v1"),
	}

	dclient := dynamicFake.NewSimpleDynamicClient(runtime.NewScheme(), resources...)
	cclient := apixFake.NewSimpleClientset(fakeCRD)
	migrator := NewMigrator(dclient, cclient, logging.FromContext(ctx))
	logger := logging.FromContext(ctx)

	// TEST
	// expects only "v1"
	MigrateStorageVersion(ctx, logger, migrator, []string{"fakes.group.dev", "unknown.group.dev"})
	crd, err := cclient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, fakeCRD.GetName(), metav1.GetOptions{})
	assert.NoError(t, err)
	storageVersions := crd.Status.StoredVersions
	assert.Len(t, storageVersions, 1)
	assert.Equal(t, "v1", storageVersions[0])

}

func fakeCrdResource(name, apiVersion string) runtime.Object {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       "Fake",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "default",
			},
		},
	}
}
