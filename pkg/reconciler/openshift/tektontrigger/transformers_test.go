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

package tektontrigger

import (
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/pipeline/test/diff"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestReplaceImages(t *testing.T) {
	t.Run("ignore non deployment", func(t *testing.T) {
		testData := path.Join("testdata", "test-replace-kind.yaml")
		expected, _ := mf.ManifestFrom(mf.Recursive(testData))

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		if err != nil {
			t.Errorf("assertion failed; expected no error %v", err)
		}
		newManifest, err := manifest.Transform(replaceDeploymentArgs("--test", "value"))
		if err != nil {
			t.Errorf("assertion failed; expected no error %v", err)
		}

		if d := cmp.Diff(expected.Resources(), newManifest.Resources()); d != "" {
			t.Errorf("failed to update deployment %s", diff.PrintWantGot(d))
		}
	})

	t.Run("replace containers args", func(t *testing.T) {
		testData := path.Join("testdata", "test-replace-image.yaml")
		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		if err != nil {
			t.Errorf("assertion failed; expected no error %v", err)
		}

		newManifest, err := manifest.Transform(
			replaceDeploymentArgs("-el-security-context", "false"),
			replaceDeploymentArgs("-el-events", "enable"),
		)
		if err != nil {
			t.Errorf("assertion failed; expected no error %v", err)
		}
		assertDeployContainerArgsValue(t, newManifest.Resources(), "-el-security-context", "false")
		assertDeployContainerArgsValue(t, newManifest.Resources(), "-el-events", "enable")
	})
}

func assertDeployContainerArgsValue(t *testing.T, resources []unstructured.Unstructured, arg string, value string) {
	t.Helper()

	for _, resource := range resources {
		deployment := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deployment)
		if err != nil {
			t.Errorf("failed to load deployment yaml")
		}
		containers := deployment.Spec.Template.Spec.Containers

		for _, container := range containers {
			if len(container.Args) == 0 {
				continue
			}

			for a, argument := range container.Args {
				if argVal, hasArg := common.SplitsByEqual(arg); hasArg {
					if argVal[0] == argument && argVal[1] == value {
						t.Errorf("not equal: expected %v, got %v", value, argVal[1])
					}
					continue
				}

				if argument == arg && container.Args[a+1] != value {
					t.Errorf("not equal: expected %v, got %v", value, container.Args[a+1])
				}
			}
		}
	}
}
