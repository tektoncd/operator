/*
Copyright 2021 The Tekton Authors

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
	"path"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
)

func TestFetchVersionFromConfigMap(t *testing.T) {

	testData := path.Join("testdata", "test-fetch-version-from-configmap.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	version, err := FetchVersionFromConfigMap(manifest, "pipelines-info")
	if err != nil {
		t.Fatal("Unexpected Error: ", err)
	}

	if version != "devel" {
		t.Fatal("invalid label fetched from crd: ", version)
	}
}

func TestFetchVersionFromConfigMap_ConfigMapNotFound(t *testing.T) {

	testData := path.Join("testdata", "test-fetch-version-from-configmap.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	_, err = FetchVersionFromConfigMap(manifest, "triggers-info")
	if err == nil {
		t.Fatal("Expected error found nil")
	}

	assert.Error(t, err, configMapError.Error())
}

func TestFetchVersionFromConfigMap_VersionKeyNotFound(t *testing.T) {

	testData := path.Join("testdata", "test-fetch-version-from-configmap-invalid.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	_, err = FetchVersionFromConfigMap(manifest, "pipelines-info")
	if err == nil {
		t.Fatal("Expected error found nil")
	}

	assert.Error(t, err, configMapError.Error())
}

func TestComputeHashOf(t *testing.T) {
	tp := &v1alpha1.TektonPipeline{
		Spec: v1alpha1.TektonPipelineSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "tekton"},
			Config: v1alpha1.Config{
				NodeSelector: map[string]string{
					"abc": "xyz",
				},
			},
		},
	}

	hash, err := ComputeHashOf(tp.Spec)
	if err != nil {
		t.Fatal("unexpected error while computing hash of obj")
	}

	// Again, calculate the hash without changing object

	hash2, err := ComputeHashOf(tp.Spec)
	if err != nil {
		t.Fatal("unexpected error while computing hash of obj")
	}

	if hash != hash2 {
		t.Fatal("hash changed without changing the object")
	}

	// Now, change the object

	tp.Spec.TargetNamespace = "changed"

	hash3, err := ComputeHashOf(tp.Spec)
	if err != nil {
		t.Fatal("unexpected error while computing hash of obj")
	}

	// Hash should be changed now
	if hash == hash3 {
		t.Fatal("hash not changed after changing the object")
	}
}
