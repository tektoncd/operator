package tektonaddon

import (
	"path"
	"testing"

	mf "github.com/manifestival/manifestival"
	"gotest.tools/v3/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPipelineRunToConfigMapConverter(t *testing.T) {
	testData := path.Join("testdata", "test-pac-pr-template.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	cmManifest, err := pipelineRunToConfigMapConverter(&manifest)
	assert.NilError(t, err)

	got := &v1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(cmManifest.Resources()[0].Object, got)
	if err != nil {
		assert.NilError(t, err)
	}

	testData = path.Join("testdata", "test-expected-pac-pr-template.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	expected := &v1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expected)
	if err != nil {
		assert.NilError(t, err)
	}

	assert.DeepEqual(t, expected.GetName(), got.GetName())
	assert.DeepEqual(t, expected.GetLabels(), got.GetLabels())
}
