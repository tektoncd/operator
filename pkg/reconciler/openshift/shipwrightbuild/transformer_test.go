/*
Copyright 2026 The Tekton Authors

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

package shipwrightbuild

import (
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuildahSeccompProfile(t *testing.T) {
	for _, name := range []string{"buildah", "source-to-image", "buildpacks", "user-strategy"} {
		t.Run(name, func(t *testing.T) {
			stepName := "buildah"
			if name == "buildah" {
				stepName = "build-and-push"
			}
			resource := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "shipwright.io/v1beta1", "kind": "ClusterBuildStrategy",
				"metadata": map[string]interface{}{"name": name},
				"spec": map[string]interface{}{"steps": []interface{}{
					map[string]interface{}{"name": "s2i-generate", "securityContext": map[string]interface{}{
						"seccompProfile": map[string]interface{}{"type": "RuntimeDefault"},
					}},
					map[string]interface{}{"name": stepName, "securityContext": map[string]interface{}{
						"capabilities": map[string]interface{}{"add": []interface{}{"SETFCAP"}},
					}},
				}},
			}}
			before := resource.DeepCopy()
			assert.NilError(t, buildahSeccompProfile()(resource))
			if name != "buildah" && name != "source-to-image" {
				assert.DeepEqual(t, resource.Object, before.Object)
				return
			}
			steps, _, err := unstructured.NestedSlice(resource.Object, "spec", "steps")
			assert.NilError(t, err)
			beforeSteps, _, err := unstructured.NestedSlice(before.Object, "spec", "steps")
			assert.NilError(t, err)
			assert.DeepEqual(t, steps[0], beforeSteps[0])
			step := steps[1].(map[string]interface{})
			profile, _, err := unstructured.NestedString(step, "securityContext", "seccompProfile", "type")
			assert.NilError(t, err)
			assert.Equal(t, profile, "Unconfined")
			capabilities, _, err := unstructured.NestedStringSlice(step, "securityContext", "capabilities", "add")
			assert.NilError(t, err)
			assert.DeepEqual(t, capabilities, []string{"SETFCAP"})
			after := resource.DeepCopy()
			assert.NilError(t, buildahSeccompProfile()(resource))
			assert.DeepEqual(t, resource.Object, after.Object)
		})
	}
}

func TestBuildahSeccompProfile_IgnoresOtherResourceTypes(t *testing.T) {
	for _, gvk := range []struct{ apiVersion, kind string }{
		{"shipwright.io/v1beta1", "BuildStrategy"},
		{"other.example/v1", "ClusterBuildStrategy"},
		{"apps/v1", "Deployment"},
	} {
		t.Run(gvk.apiVersion+"/"+gvk.kind, func(t *testing.T) {
			resource := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": gvk.apiVersion, "kind": gvk.kind,
				"metadata": map[string]interface{}{"name": "buildah"},
				"spec": map[string]interface{}{"steps": []interface{}{
					map[string]interface{}{"name": "build-and-push"},
				}},
			}}
			before := resource.DeepCopy()
			assert.NilError(t, buildahSeccompProfile()(resource))
			assert.DeepEqual(t, resource.Object, before.Object)
		})
	}
}
