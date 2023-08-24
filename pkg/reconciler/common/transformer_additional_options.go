/*
Copyright 2023 The Tekton Authors

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
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

const (
	KindConfigMap   = "ConfigMap"
	KindDeployment  = "Deployment"
	KindStatefulSet = "StatefulSet"
)

type OptionsTransformer struct {
	options v1alpha1.AdditionalOptions
	logger  *zap.SugaredLogger
}

func ExecuteAdditionalOptionsTransformer(ctx context.Context, manifest *mf.Manifest, targetNamespace string, additionalOptions v1alpha1.AdditionalOptions) error {
	ot := &OptionsTransformer{
		options: additionalOptions,
		logger:  logging.FromContext(ctx),
	}

	if additionalOptions.Disabled {
		return nil
	}

	// execute transformer
	finalManifest, err := manifest.Transform(ot.transform)
	if err != nil {
		return err
	}
	*manifest = finalManifest

	// create config map if not found in the manifest
	extraConfigMaps, err := ot.createConfigMaps(manifest, targetNamespace, additionalOptions)
	if err != nil {
		return err
	}

	additionalManifest, err := mf.ManifestFrom(mf.Slice(extraConfigMaps))
	if err != nil {
		return err
	}

	*manifest = manifest.Append(additionalManifest)

	return nil
}

func (ot *OptionsTransformer) transform(u *unstructured.Unstructured) error {
	switch u.GetKind() {
	case KindConfigMap:
		return ot.updateConfigMaps(u)

	case KindDeployment:
		err := ot.updateDeployments(u)
		if err != nil {
			return err
		}
		// update deployment hash value in to template labels
		// this will recreate the pods, if there is a change detected in deployment.spec
		return ot.updateDeploymentHashValue(u)

	case KindStatefulSet:
		return ot.updateStatefulSets(u)

	}

	return nil
}

func (ot *OptionsTransformer) updateLabels(u *unstructured.Unstructured, labels map[string]string) error {
	if len(labels) == 0 {
		return nil
	}

	actualLabels := u.GetLabels()
	if actualLabels == nil {
		actualLabels = make(map[string]string)
	}

	for labelKey, labelValue := range labels {
		actualLabels[labelKey] = labelValue
	}

	u.SetLabels(actualLabels)

	return nil
}

func (ot *OptionsTransformer) updateAnnotations(u *unstructured.Unstructured, annotations map[string]string) error {
	if len(annotations) == 0 {
		return nil
	}

	actualAnnotations := u.GetAnnotations()
	if actualAnnotations == nil {
		actualAnnotations = make(map[string]string)
	}

	for annotationKey, annotationValue := range annotations {
		actualAnnotations[annotationKey] = annotationValue
	}

	u.SetAnnotations(actualAnnotations)

	return nil
}

func (ot *OptionsTransformer) updateConfigMaps(u *unstructured.Unstructured) error {

	optionsConfigMap, found := ot.options.ConfigMaps[u.GetName()]
	if !found {
		return nil
	}

	// update labels
	err := ot.updateLabels(u, optionsConfigMap.Labels)
	if err != nil {
		return err
	}

	// update annotations
	err = ot.updateAnnotations(u, optionsConfigMap.Annotations)
	if err != nil {
		return err
	}

	// convert unstructured object to configMap
	targetConfigMap := &corev1.ConfigMap{}
	err = apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, targetConfigMap)
	if err != nil {
		return err
	}

	// update data part of the target config map
	for dataKey, newValue := range optionsConfigMap.Data {
		targetConfigMap.Data[dataKey] = newValue
	}

	// convert configMap to unstructured object
	obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(targetConfigMap)
	if err != nil {
		return err
	}
	u.SetUnstructuredContent(obj)

	return nil
}

func (ot *OptionsTransformer) createConfigMaps(manifest *mf.Manifest, targetNamespace string, additionalOptions v1alpha1.AdditionalOptions) ([]unstructured.Unstructured, error) {
	extraConfigMaps := []unstructured.Unstructured{}
	existingConfigMaps := manifest.Filter(mf.Any(mf.ByKind(KindConfigMap)))
	for configMapName, providedConfigMap := range additionalOptions.ConfigMaps {
		found := false
		for _, resource := range existingConfigMaps.Resources() {
			if resource.GetName() == configMapName {
				found = true
				break
			}
		}
		if found {
			continue
		}

		// update name
		providedConfigMap.SetName(configMapName)

		// always update namespace to targetNamespace
		providedConfigMap.SetNamespace(targetNamespace)

		// update kind
		if providedConfigMap.TypeMeta.Kind == "" {
			providedConfigMap.TypeMeta.Kind = KindConfigMap
		}

		// update api version
		if providedConfigMap.TypeMeta.APIVersion == "" {
			providedConfigMap.TypeMeta.APIVersion = corev1.SchemeGroupVersion.Version
		}

		// convert configMap to unstructured object
		obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(&providedConfigMap)
		if err != nil {
			return nil, err
		}
		u := unstructured.Unstructured{}
		u.SetUnstructuredContent(obj)
		extraConfigMaps = append(extraConfigMaps, u)
	}

	return extraConfigMaps, nil
}

func (ot *OptionsTransformer) updateDeployments(u *unstructured.Unstructured) error {
	// verify the deployment has changes
	deploymentOptions, found := ot.options.Deployments[u.GetName()]
	if !found {
		return nil
	}

	// update labels
	err := ot.updateLabels(u, deploymentOptions.Labels)
	if err != nil {
		return err
	}

	// update annotations
	err = ot.updateAnnotations(u, deploymentOptions.Annotations)
	if err != nil {
		return err
	}

	// convert unstructured object to deployment
	targetDeployment := &appsv1.Deployment{}
	err = apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, targetDeployment)
	if err != nil {
		return err
	}

	// update replicas
	if deploymentOptions.Spec.Replicas != nil {
		targetDeployment.Spec.Replicas = ptr.Int32(*deploymentOptions.Spec.Replicas)
	}

	// update affinity
	if deploymentOptions.Spec.Template.Spec.Affinity != nil {
		targetDeployment.Spec.Template.Spec.Affinity = deploymentOptions.Spec.Template.Spec.Affinity
	}

	// update node selectors
	if len(deploymentOptions.Spec.Template.Spec.NodeSelector) > 0 {
		targetDeployment.Spec.Template.Spec.NodeSelector = deploymentOptions.Spec.Template.Spec.NodeSelector
	}

	// update tolerations
	if len(deploymentOptions.Spec.Template.Spec.Tolerations) > 0 {
		targetDeployment.Spec.Template.Spec.Tolerations = deploymentOptions.Spec.Template.Spec.Tolerations
	}

	// update Topology Spread Constraints
	if len(deploymentOptions.Spec.Template.Spec.TopologySpreadConstraints) > 0 {
		targetDeployment.Spec.Template.Spec.TopologySpreadConstraints = deploymentOptions.Spec.Template.Spec.TopologySpreadConstraints
	}

	// update volumes
	targetDeployment.Spec.Template.Spec.Volumes = ot.updateVolumes(targetDeployment.Spec.Template.Spec.Volumes, deploymentOptions.Spec.Template.Spec.Volumes)

	// update init containers
	targetDeployment.Spec.Template.Spec.InitContainers = ot.updateContainers(targetDeployment.Spec.Template.Spec.InitContainers, deploymentOptions.Spec.Template.Spec.InitContainers)

	// update containers
	targetDeployment.Spec.Template.Spec.Containers = ot.updateContainers(targetDeployment.Spec.Template.Spec.Containers, deploymentOptions.Spec.Template.Spec.Containers)

	// convert deployment to unstructured object
	obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(targetDeployment)
	if err != nil {
		return err
	}
	u.SetUnstructuredContent(obj)

	return nil
}

func (ot *OptionsTransformer) updateVolumes(sourceVolumes, additionalVolumes []corev1.Volume) []corev1.Volume {
	for _, newVolume := range additionalVolumes {
		itemFound := false
		for volumeIndex, oldVolume := range sourceVolumes {
			if oldVolume.Name == newVolume.Name {
				sourceVolumes[volumeIndex] = newVolume
				itemFound = true
				break
			}
		}
		if !itemFound {
			sourceVolumes = append(sourceVolumes, newVolume)
		}
	}
	return sourceVolumes
}

func (ot *OptionsTransformer) updateContainers(targetContainers, containersOptions []corev1.Container) []corev1.Container {
	for _, containerOptions := range containersOptions {
		for containerIndex := range targetContainers {
			targetContainer := targetContainers[containerIndex]
			if containerOptions.Name != targetContainer.Name {
				continue
			}

			// update resource requirements
			if containerOptions.Resources.Size() != 0 {
				targetContainers[containerIndex].Resources = containerOptions.Resources
			}

			// update environments
			{
				envVariables := targetContainer.Env
				for _, newEnv := range containerOptions.Env {
					itemFound := false
					for envIndex, oldEnv := range envVariables {
						if oldEnv.Name == newEnv.Name {
							envVariables[envIndex] = newEnv
							itemFound = true
							break
						}
					}
					if !itemFound {
						envVariables = append(envVariables, newEnv)
					}
				}
				targetContainers[containerIndex].Env = envVariables
			}

			// update volume mounts
			{
				volumeMounts := targetContainer.VolumeMounts
				for _, newVolumeMount := range containerOptions.VolumeMounts {
					itemFound := false
					for volumeMountIndex, oldVolumeMount := range volumeMounts {
						if oldVolumeMount.Name == newVolumeMount.Name {
							volumeMounts[volumeMountIndex] = newVolumeMount
							itemFound = true
							break
						}
					}
					if !itemFound {
						volumeMounts = append(volumeMounts, newVolumeMount)
					}
				}
				targetContainers[containerIndex].VolumeMounts = volumeMounts
			}

			// update arguments
			// currently arguments are only appending with existing args
			// NOTE: This action may cause duplication of arguments
			targetContainers[containerIndex].Args = append(targetContainers[containerIndex].Args, containerOptions.Args...)

		}
	}

	return targetContainers
}

// calculate deployment spec hash value and update it under pods label(under template).
// If there is change detected in deployment spec, all pods will be recreated, as we the pods label(hash value label) is updated
func (ot *OptionsTransformer) updateDeploymentHashValue(u *unstructured.Unstructured) error {
	// convert unstructured object to deployment
	deployment := &appsv1.Deployment{}
	err := apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, deployment)
	if err != nil {
		return err
	}

	// remove some of the fields, that we do not want to calculate hash value
	deployment.Spec.Selector = nil
	deployment.Spec.Strategy = appsv1.DeploymentStrategy{}
	// remove existing hash value from template
	if len(deployment.Spec.Template.Labels) == 0 {
		deployment.Spec.Template.Labels = map[string]string{}
	}
	deployment.Spec.Template.Labels[v1alpha1.DeploymentSpecHashValueLabelKey] = ""

	// label value max limit is 63 chars, sha256 hash produces 64 chars
	// use md5 hash which is 16 chars
	hashValue, err := hash.ComputeMd5(deployment.Spec)
	if err != nil {
		return err
	}

	// update hash value
	obj := u.Object
	if err := unstructured.SetNestedField(obj, hashValue, "spec", "template", "metadata", "labels", v1alpha1.DeploymentSpecHashValueLabelKey); err != nil {
		return err
	}

	u.SetUnstructuredContent(obj)
	return nil
}

func (ot *OptionsTransformer) updateStatefulSets(u *unstructured.Unstructured) error {
	// verify the statefulSet has changes
	statefulSetOptions, found := ot.options.StatefulSets[u.GetName()]
	if !found {
		return nil
	}

	// update labels
	err := ot.updateLabels(u, statefulSetOptions.Labels)
	if err != nil {
		return err
	}

	// update annotations
	err = ot.updateAnnotations(u, statefulSetOptions.Annotations)
	if err != nil {
		return err
	}

	// convert unstructured object to statefulSet
	targetStatefulSet := &appsv1.StatefulSet{}
	err = apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, targetStatefulSet)
	if err != nil {
		return err
	}

	// update replicas
	if statefulSetOptions.Spec.Replicas != nil {
		targetStatefulSet.Spec.Replicas = ptr.Int32(*statefulSetOptions.Spec.Replicas)
	}

	// update affinity
	if statefulSetOptions.Spec.Template.Spec.Affinity != nil {
		targetStatefulSet.Spec.Template.Spec.Affinity = statefulSetOptions.Spec.Template.Spec.Affinity
	}

	// update node selectors
	if len(statefulSetOptions.Spec.Template.Spec.NodeSelector) > 0 {
		targetStatefulSet.Spec.Template.Spec.NodeSelector = statefulSetOptions.Spec.Template.Spec.NodeSelector
	}

	// update tolerations
	if len(statefulSetOptions.Spec.Template.Spec.Tolerations) > 0 {
		targetStatefulSet.Spec.Template.Spec.Tolerations = statefulSetOptions.Spec.Template.Spec.Tolerations
	}

	// update Topology Spread Constraints
	if len(statefulSetOptions.Spec.Template.Spec.TopologySpreadConstraints) > 0 {
		targetStatefulSet.Spec.Template.Spec.TopologySpreadConstraints = statefulSetOptions.Spec.Template.Spec.TopologySpreadConstraints
	}

	// update pod management policy
	if statefulSetOptions.Spec.PodManagementPolicy != "" {
		targetStatefulSet.Spec.PodManagementPolicy = statefulSetOptions.Spec.PodManagementPolicy
	}

	// update service name
	if statefulSetOptions.Spec.ServiceName != "" {
		targetStatefulSet.Spec.ServiceName = statefulSetOptions.Spec.ServiceName
	}

	// update volume claim templates
	if len(statefulSetOptions.Spec.VolumeClaimTemplates) > 0 {
		for _, newVolumeClaimTpl := range statefulSetOptions.Spec.VolumeClaimTemplates {
			itemFound := false
			for volumeClaimTplIndex, oldVolumeClaimTpl := range targetStatefulSet.Spec.VolumeClaimTemplates {
				if oldVolumeClaimTpl.Name == newVolumeClaimTpl.Name {
					targetStatefulSet.Spec.VolumeClaimTemplates[volumeClaimTplIndex] = newVolumeClaimTpl
					itemFound = true
					break
				}
			}
			if !itemFound {
				targetStatefulSet.Spec.VolumeClaimTemplates = append(targetStatefulSet.Spec.VolumeClaimTemplates, newVolumeClaimTpl)
			}
		}
	}

	// update volumes
	targetStatefulSet.Spec.Template.Spec.Volumes = ot.updateVolumes(targetStatefulSet.Spec.Template.Spec.Volumes, statefulSetOptions.Spec.Template.Spec.Volumes)

	// update init containers
	targetStatefulSet.Spec.Template.Spec.InitContainers = ot.updateContainers(targetStatefulSet.Spec.Template.Spec.InitContainers, statefulSetOptions.Spec.Template.Spec.InitContainers)

	// update containers
	targetStatefulSet.Spec.Template.Spec.Containers = ot.updateContainers(targetStatefulSet.Spec.Template.Spec.Containers, statefulSetOptions.Spec.Template.Spec.Containers)

	// convert statefulSet to unstructured object
	obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(targetStatefulSet)
	if err != nil {
		return err
	}
	u.SetUnstructuredContent(obj)

	return nil
}
