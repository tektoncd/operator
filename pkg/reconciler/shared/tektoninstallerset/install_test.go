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

package tektoninstallerset

import (
	"context"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	installer "github.com/tektoncd/operator/pkg/reconciler/common/tektoninstallerset"
	utils "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateInstallerset(t *testing.T) {
	testData := path.Join("testdata", "release.yaml")

	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	di := installer.NewDefaultInstaller()
	di.AddManifest(manifest)

	di.AddLabelsFromMap(map[string]string{
		v1alpha1.CreatedByKey: "pipeline",
	})

	di.AddAnnotationsFromMap(map[string]string{
		"ns": "tekton-pipelines",
	})

	fakeClientset := fake.NewSimpleClientset()

	client := fakeClientset.OperatorV1alpha1().TektonInstallerSets()

	newISM := newTisMetaWithName("pipeline")

	generateIs, err := generateInstallerSet(context.Background(), di, newISM)
	assert.NilError(t, err)

	createdIs, err := createWithClient(context.Background(), client, generateIs)
	assert.Equal(t, err, nil)

	labels := map[string]string{v1alpha1.CreatedByKey: "pipeline"}

	specHash, err := getHash(installerSpec(&manifest))
	assert.NilError(t, err)

	annotations := map[string]string{
		"ns":                                    "tekton-pipelines",
		"operator.tekton.dev/last-applied-hash": specHash,
	}

	expectedIs := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "pipeline",
			Labels:          labels,
			Annotations:     annotations,
			OwnerReferences: []metav1.OwnerReference{},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}

	if d := cmp.Diff(createdIs, expectedIs); d != "" {
		t.Errorf("Actual created installerset is different from the expected one %s", utils.PrintWantGot(d))
	}
}

func TestMakeInstallerset(t *testing.T) {
	testData := path.Join("testdata", "release.yaml")

	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	tis := newTisMetaWithName("pipeline")

	tis.Labels = map[string]string{v1alpha1.CreatedByKey: "pipeline"}
	tis.Annotations = map[string]string{"ns": "tekton-pipelines"}

	actual, err := makeInstallerSet(&manifest, tis)
	assert.NilError(t, err)

	labels := map[string]string{v1alpha1.CreatedByKey: "pipeline"}

	specHash, err := getHash(installerSpec(&manifest))
	assert.NilError(t, err)

	annotations := map[string]string{
		"ns":                                    "tekton-pipelines",
		"operator.tekton.dev/last-applied-hash": specHash,
	}

	expected := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "pipeline",
			Labels:          labels,
			Annotations:     annotations,
			OwnerReferences: nil,
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}

	if d := cmp.Diff(actual, expected); d != "" {
		t.Errorf("Actual installerset is different from the expected one %s", utils.PrintWantGot(d))
	}
}
