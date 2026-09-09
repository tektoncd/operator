/*
Copyright 2025 The Tekton Authors

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

package tektonpruner

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
)

// addDefaultResourceLimits injects pruner-specific resource limits based on benchmark data
// Overridable via TektonPruner.Spec.Options (options transformer runs last).
func addDefaultResourceLimits() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := u.GetKind()
		if kind != "Deployment" && kind != "StatefulSet" {
			return nil
		}

		deploymentName := u.GetName()

		// Only apply to pruner controller and webhook
		if deploymentName != "tekton-pruner-controller" && deploymentName != "tekton-pruner-webhook" {
			return nil
		}

		// Define default resources based on benchmark results
		var defaultResources corev1.ResourceRequirements

		if deploymentName == "tekton-pruner-controller" {
			// Controller: handles PipelineRun/TaskRun informer cache
			// Benchmark: 846-1126 MB for 26k-38k runs
			// 2Gi limit provides safety margin for production scale
			defaultResources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			}
		} else if deploymentName == "tekton-pruner-webhook" {
			// Webhook: stateless, minimal memory footprint
			defaultResources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			}
		}

		// Both Deployment and StatefulSet have containers at spec.template.spec.containers
		// Use unstructured path access to avoid duplication
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

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		prunerCR := comp.(*v1alpha1.TektonPruner)

		imagesRaw := common.ToLowerCaseKeys(common.ImagesFromEnv(common.PrunerImagePrefix))
		prunerImages := common.ImageRegistryDomainOverride(imagesRaw)
		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.TektonPrunerResourceName),
			common.DeploymentImages(prunerImages),
			common.AddDeploymentRestrictedPSA(),
			common.AddConfigMapValues(PrunerConfigMapName, prunerCR.Spec.TektonPrunerConfig),
			addDefaultResourceLimits(), // Add default resource limits (can be overridden via Options)
		}
		extra = append(extra, extension.Transformers(prunerCR)...)
		err := common.Transform(ctx, manifest, prunerCR, extra...)
		if err != nil {
			return &mf.Manifest{}, err
		}

		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, prunerCR.Spec.GetTargetNamespace(), prunerCR.Spec.Options); err != nil {
			return &mf.Manifest{}, err
		}

		return manifest, nil
	}
}
