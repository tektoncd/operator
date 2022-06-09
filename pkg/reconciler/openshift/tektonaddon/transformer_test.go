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

package tektonaddon

import (
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	console "github.com/openshift/api/console/v1"
	"github.com/tektoncd/pipeline/test/diff"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestUpdateConsoleCLIDownload(t *testing.T) {
	testData := path.Join("testdata", "test-console-cli.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	testData = path.Join("testdata", "test-console-cli-expected.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	newManifest, err := manifest.Transform(replaceURLCCD("testserver.com", "1.2.3"))
	assert.NilError(t, err)

	got := &console.ConsoleCLIDownload{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(newManifest.Resources()[0].Object, got)
	if err != nil {
		t.Errorf("failed to load consoleclidownload yaml")
	}

	expected := &console.ConsoleCLIDownload{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expected)
	if err != nil {
		t.Errorf("failed to load consoleclidownload yaml")
	}

	if d := cmp.Diff(expected, got); d != "" {
		t.Errorf("failed to update consoleclidownload %s", diff.PrintWantGot(d))
	}
}

func TestSetVersionedNames(t *testing.T) {
	testData := path.Join("testdata", "test-versioned-clustertask-name.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	testData = path.Join("testdata", "test-versioned-clustertask-name-expected.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	operatorVersion := "v1.7.0"
	newManifest, err := manifest.Transform(setVersionedNames(operatorVersion))
	assert.NilError(t, err)

	if d := cmp.Diff(expectedManifest.Resources(), newManifest.Resources()); d != "" {
		t.Errorf("failed to update versioned clustertask name %s", diff.PrintWantGot(d))
	}
}
