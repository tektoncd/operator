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

package common

import (
	mf "github.com/manifestival/manifestival"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
)

// AddDefaultResourceRequirements returns a transformer that injects default resource
// requests and limits into Deployments and StatefulSets based on their name.
//
// The resourceMap parameter maps deployment/statefulset names to their default
// ResourceRequirements. Only resources with empty or missing resource specifications
// are updated - existing resource configurations are preserved.
//
// This allows components to define resource defaults without overriding user-specified
// values or values set by the additional options transformer.
func AddDefaultResourceRequirements(resourceMap map[string]corev1.ResourceRequirements) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := u.GetKind()
		if kind != "Deployment" && kind != "StatefulSet" {
			return nil
		}

		// Check if this resource has defaults defined
		defaultResources, found := resourceMap[u.GetName()]
		if !found {
			return nil
		}

		// Both Deployment and StatefulSet have containers at spec.template.spec.containers
		containers, found, err := unstructured.NestedSlice(u.Object, "spec", "template", "spec", "containers")
		if !found || err != nil {
			return err
		}

		modified := false
		for i := range containers {
			containerMap := containers[i].(map[string]interface{})

			// Check if resources field exists
			resources, hasResources := containerMap["resources"]
			if !hasResources {
				// No resources field, add defaults
				resourcesMap, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(&defaultResources)
				if err != nil {
					return err
				}
				containerMap["resources"] = resourcesMap
				modified = true
			} else {
				// Has resources field, check if both requests and limits are empty
				resourcesMap := resources.(map[string]interface{})
				_, hasRequests := resourcesMap["requests"]
				_, hasLimits := resourcesMap["limits"]
				if !hasRequests && !hasLimits {
					// Both nil, replace with defaults
					resourcesMap, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(&defaultResources)
					if err != nil {
						return err
					}
					containerMap["resources"] = resourcesMap
					modified = true
				}
			}
		}

		if modified {
			if err := unstructured.SetNestedSlice(u.Object, containers, "spec", "template", "spec", "containers"); err != nil {
				return err
			}
		}

		return nil
	}
}
