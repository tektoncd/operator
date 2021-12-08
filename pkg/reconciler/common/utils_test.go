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
