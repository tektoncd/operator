package common

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/labels"
)

func TestInjectOperandNameLabel(t *testing.T) {
	inPath := filepath.Join("testdata", "inject-label", "01-sample-tektoncd-pipelines-release.yaml")
	inputManifest, err := mf.ManifestFrom(mf.Recursive(inPath))
	assert.NilError(t, err)

	preserveExisting := false
	tr := injectOperandNameLabel("tektoncd-pipeline", preserveExisting)

	got, err := inputManifest.Transform(tr)
	assert.NilError(t, err)

	expectedOutputPath := filepath.Join("testdata", "inject-label", "04-operand-label-expected-result.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(expectedOutputPath))
	assert.NilError(t, err)

	if d := cmp.Diff(expectedManifest.Resources(), got.Resources()); d != "" {
		t.Errorf("inject label tranformation failed, +expected,-got: %s", d)
	}
}

func TestInjectLabel(t *testing.T) {
	testCases := []struct {
		description        string
		testDataPath       string
		labels             labels.Set
		preserveExisting   bool
		skipChecks         []mf.Predicate
		expectedOutputPath string
	}{
		{
			description:  "add labels to all resources when no skipChecks are provided",
			testDataPath: "inject-label/01-sample-tektoncd-pipelines-release.yaml",
			labels: labels.Set{
				"foo": "bar",
			},
			preserveExisting:   false,
			skipChecks:         nil,
			expectedOutputPath: "inject-label/02-no-predicates-test-expected-result.yaml",
		},
		{
			description:  "add labels to resources skipping the ones which pass skip predicates provided",
			testDataPath: "inject-label/01-sample-tektoncd-pipelines-release.yaml",
			labels: labels.Set{
				"foo": "bar",
			},
			preserveExisting: false,
			skipChecks: []mf.Predicate{
				mf.ByName("tekton-pipelines-controller"),
				mf.ByKind("Service"),
			},
			expectedOutputPath: "inject-label/03-with-skipchecks-test-expected-result.yaml",
		},
		{
			description:  "preserve values of labels which are already existing, when preserveExisting == true",
			testDataPath: "inject-label/05-existing-labels-release.yaml",
			labels: labels.Set{
				"foo": "new_val",
			},
			preserveExisting:   true,
			skipChecks:         []mf.Predicate{},
			expectedOutputPath: "inject-label/06-existing-labels-release-expected.yaml",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			inPath := filepath.Join("testdata", tc.testDataPath)
			inputManifest, err := mf.ManifestFrom(mf.Recursive(inPath))
			assert.NilError(t, err)

			tr := injectLabel(tc.labels, tc.preserveExisting, tc.skipChecks...)

			got, err := inputManifest.Transform(tr)
			assert.NilError(t, err)

			expectedOutputPath := filepath.Join("testdata", tc.expectedOutputPath)
			expectedManifest, err := mf.ManifestFrom(mf.Recursive(expectedOutputPath))
			assert.NilError(t, err)

			if d := cmp.Diff(expectedManifest.Resources(), got.Resources()); d != "" {
				t.Errorf("inject label tranformation failed, +expected,-got: %s", d)
			}
		})
	}
}
