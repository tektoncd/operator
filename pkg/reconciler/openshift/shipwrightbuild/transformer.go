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
	"github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// buildahSeccompProfile permits Buildah's user-namespace syscalls without
// relaxing the seccomp profile of source-fetching or other build steps.
func buildahSeccompProfile() manifestival.Transformer {
	return func(resource *unstructured.Unstructured) error {
		if resource.GroupVersionKind().Group != "shipwright.io" || resource.GetKind() != "ClusterBuildStrategy" {
			return nil
		}
		switch resource.GetName() {
		case "buildah", "source-to-image":
		default:
			return nil
		}
		steps, found, err := unstructured.NestedSlice(resource.Object, "spec", "steps")
		if err != nil || !found {
			return err
		}
		stepName := "buildah"
		if resource.GetName() == "buildah" {
			stepName = "build-and-push"
		}
		for _, value := range steps {
			step, ok := value.(map[string]interface{})
			if !ok || step["name"] != stepName {
				continue
			}
			if err := unstructured.SetNestedField(step, "Unconfined", "securityContext", "seccompProfile", "type"); err != nil {
				return err
			}
		}
		return unstructured.SetNestedSlice(resource.Object, steps, "spec", "steps")
	}
}
