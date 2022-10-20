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

package openshiftpipelinesascode

import (
	"path"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/manifestival/manifestival/fake"
	"gotest.tools/v3/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestUpdateControllerRouteInConfigMap(t *testing.T) {
	testNamspace := "osp"
	testData := path.Join("testdata", "test-update-route.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	res := []runtime.Object{}
	for _, r := range manifest.Resources() {
		// change the namespace while adding in client
		// namespace in manifest should be different from one installed on cluster
		// to test this properly
		r.SetNamespace(testNamspace)
		newRes := r
		res = append(res, &newRes)
	}

	fakeClient := fake.New(res...)
	manifest.Client = fakeClient

	err = updateControllerRouteInConfigMap(&manifest, testNamspace)
	assert.NilError(t, err)

	// check if configmap is updated
	var unstructuredRes unstructured.Unstructured
	if manifest.Resources()[0].GetKind() == "ConfigMap" {
		unstructuredRes = manifest.Resources()[0]
	} else {
		unstructuredRes = manifest.Resources()[1]
	}
	unstructuredRes.SetNamespace(testNamspace)

	cmUnstr, err := fakeClient.Get(&unstructuredRes)
	assert.NilError(t, err)

	cm := &v1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(cmUnstr.Object, cm)
	assert.NilError(t, err)
	assert.Equal(t, cm.Data["controller-url"], "https://pac.controller.test.com")
}
