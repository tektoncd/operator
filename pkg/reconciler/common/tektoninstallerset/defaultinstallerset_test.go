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
	utils "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetLabels(t *testing.T) {
	di := NewDefaultInstaller()
	labels := map[string]string{
		v1alpha1.CreatedByKey: "pipeline",
	}
	di.AddLabelsFromMap(labels)

	got := di.GetLabels(context.Background())

	if d := cmp.Diff(labels, got); d != "" {
		t.Errorf("Actual labels are different from the expected one %s", utils.PrintWantGot(d))
	}
}

func TestGetAnnotations(t *testing.T) {
	di := NewDefaultInstaller()
	annotations := map[string]string{
		"ns": "tekton-pipelines",
	}
	di.AddAnnotationsFromMap(annotations)

	got := di.GetAnnotations(context.Background())

	if d := cmp.Diff(annotations, got); d != "" {
		t.Errorf("Actual annotations are different from the expected one %s", utils.PrintWantGot(d))
	}
}

func TestGetOwnerRef(t *testing.T) {
	di := NewDefaultInstaller()

	component := &v1alpha1.TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-name",
		},
		Spec: v1alpha1.TektonPipelineSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}

	ownerRef := *metav1.NewControllerRef(component, component.GetGroupVersionKind())
	di.AddOwnerReferences(ownerRef)

	got := di.GetOwnerReferences(context.Background())

	if d := cmp.Diff(ownerRef, got[0]); d != "" {
		t.Errorf("Actual ownerRef are different from the expected one %s", utils.PrintWantGot(d))
	}
}

func TestGetManifest(t *testing.T) {
	di := NewDefaultInstaller()

	testData := path.Join("testdata", "release.yaml")

	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	di.AddManifest(manifest)

	got, err := di.GetManifest(context.Background())
	assert.Equal(t, err, nil)

	if d := cmp.Diff(manifest.Resources(), got.Resources()); d != "" {
		t.Errorf("Actual manifest is different from the expected one %s", utils.PrintWantGot(d))
	}
}

func TestAddLabelKeyVal(t *testing.T) {
	di := NewDefaultInstaller()

	di.AddLabelKeyVal(v1alpha1.CreatedByKey, "pipeline")

	assert.Equal(t, len(di.GetLabels(context.Background())), 1)
}

func TestAddLabelFromMap(t *testing.T) {
	di := NewDefaultInstaller()

	di.AddLabelsFromMap(map[string]string{
		v1alpha1.CreatedByKey: "pipeline",
	})

	assert.Equal(t, len(di.GetLabels(context.Background())), 1)
}

func TestAddAnnotationsKeyVal(t *testing.T) {
	di := NewDefaultInstaller()

	di.AddAnnotationsKeyVal("ns", "tekton-pipeline")

	assert.Equal(t, len(di.GetAnnotations(context.Background())), 1)
}

func TestAddAnnotationsFromMap(t *testing.T) {
	di := NewDefaultInstaller()

	di.AddAnnotationsFromMap(map[string]string{
		"ns": "tekton-pipelines",
	})

	assert.Equal(t, len(di.GetAnnotations(context.Background())), 1)
}
