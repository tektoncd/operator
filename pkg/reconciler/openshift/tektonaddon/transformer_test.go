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
	"bytes"
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	console "github.com/openshift/api/console/v1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	triggersv1beta1 "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
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

func TestReplacePACTriggerTemplateImages(t *testing.T) {
	testData := path.Join("testdata", "test-triggertemplate-taskrun.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	stepName := "apply-and-launch"
	pacTestImage := "registry.test.io/openshift-pipelines/pipelines-as-code"
	err = os.Setenv("IMAGE_PAC_TRIGGERTEMPLATE_APPLY_AND_LAUNCH", pacTestImage)
	assert.NilError(t, err)

	pacImages := pacTriggerTemplateStepImages()
	t.Log(pacImages)
	assert.Equal(t, len(pacImages), 1)

	transformedManifest, err := manifest.Transform(replacePACTriggerTemplateImages(pacImages))
	assert.NilError(t, err)

	for _, resource := range transformedManifest.Resources() {
		// replacement is only done for TriggerTemplates
		if resource.GetKind() != "TriggerTemplate" {
			continue
		}
		tt := &triggersv1beta1.TriggerTemplate{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, tt)
		assert.NilError(t, err)

		for _, resourceTemplate := range tt.Spec.ResourceTemplates {
			taskRun := pipelinev1beta1.TaskRun{}
			decoder := json.NewDecoder(bytes.NewBuffer(resourceTemplate.RawExtension.Raw))
			err := decoder.Decode(&taskRun)
			assert.NilError(t, err)

			for _, step := range taskRun.Spec.TaskSpec.Steps {
				if step.Name == stepName {
					assert.Equal(t, step.Container.Image, pacTestImage)
				}
			}
		}
	}
}
