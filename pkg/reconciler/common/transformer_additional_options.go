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
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"go.uber.org/zap"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

const (
	KindConfigMap                      = "ConfigMap"
	KindDeployment                     = "Deployment"
	KindStatefulSet                    = "StatefulSet"
	KindHorizontalPodAutoscaler        = "HorizontalPodAutoscaler"
	KindValidatingWebhookConfiguration = "ValidatingWebhookConfiguration"
	KindMutatingWebhookConfiguration   = "MutatingWebhookConfiguration"
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

	if additionalOptions.Disabled != nil && *additionalOptions.Disabled {
		return nil
	}

	// execute transformer
	finalManifest, err := manifest.Transform(ot.transform)
	if err != nil {
		return err
	}
	*manifest = finalManifest

	// create config map, if not found in the existing manifest
	extraConfigMaps, err := ot.createConfigMaps(manifest, targetNamespace, additionalOptions)
	if err != nil {
		return err
	}
	// update into the manifests
	if err = ot.addInToManifest(manifest, extraConfigMaps); err != nil {
		return err
	}

	// create HorizontalPodAutoscaler, if not found in the existing manifest
	extraHPAs, err := ot.createHorizontalPodAutoscalers(manifest, targetNamespace, additionalOptions)
	if err != nil {
		return err
	}
	// update into the manifests
	if err = ot.addInToManifest(manifest, extraHPAs); err != nil {
		return err
	}

	return nil
}

func (ot *OptionsTransformer) addInToManifest(manifest *mf.Manifest, additionalResources []unstructured.Unstructured) error {
	if len(additionalResources) > 0 {
		additionalManifest, err := mf.ManifestFrom(mf.Slice(additionalResources))
		if err != nil {
			return err
		}
		*manifest = manifest.Append(additionalManifest)
	}
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

	case KindHorizontalPodAutoscaler:
		return ot.updateHorizontalPodAutoscalers(u)

	case KindValidatingWebhookConfiguration, KindMutatingWebhookConfiguration:
		return ot.updateWebhookConfiguration(u)
	}

	return nil
}

func (ot *OptionsTransformer) updateLabels(u *unstructured.Unstructured, labels map[string]string) error {
	return ot.updateMapField(u, labels, "metadata", "labels")
}

func (ot *OptionsTransformer) updateAnnotations(u *unstructured.Unstructured, annotations map[string]string) error {
	return ot.updateMapField(u, annotations, "metadata", "annotations")
}

func (ot *OptionsTransformer) updateMapField(u *unstructured.Unstructured, extraData map[string]string, locationKey ...string) error {
	if len(extraData) == 0 || len(locationKey) == 0 {
		return nil
	}

	// get source map data
	sourceData, _, err := unstructured.NestedMap(u.Object, locationKey...)
	if err != nil {
		return err
	}

	if sourceData == nil {
		sourceData = make(map[string]interface{})
	}

	// update source map data
	for key, value := range extraData {
		sourceData[key] = value
	}

	// update source map data into the target object
	unstructured.RemoveNestedField(u.Object, locationKey...)
	return unstructured.SetNestedMap(u.Object, sourceData, locationKey...)
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

	// update pod template labels
	err = ot.updateMapField(u, deploymentOptions.Spec.Template.Labels, "spec", "template", "metadata", "labels")
	if err != nil {
		return err
	}

	// update pod template annotations
	err = ot.updateMapField(u, deploymentOptions.Spec.Template.Annotations, "spec", "template", "metadata", "annotations")
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

	// update PriorityClassName
	if deploymentOptions.Spec.Template.Spec.PriorityClassName != "" {
		targetDeployment.Spec.Template.Spec.PriorityClassName = deploymentOptions.Spec.Template.Spec.PriorityClassName
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

	// update runTimeClassName
	if deploymentOptions.Spec.Template.Spec.RuntimeClassName != nil {
		targetDeployment.Spec.Template.Spec.RuntimeClassName = deploymentOptions.Spec.Template.Spec.RuntimeClassName
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
	containersToAdd := []corev1.Container{}
	for _, containerOptions := range containersOptions {
		containerFound := false
		for containerIndex := range targetContainers {
			targetContainer := targetContainers[containerIndex]
			if containerOptions.Name != targetContainer.Name {
				continue
			}
			containerFound = true

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

			// update arguments: replace by key, support pair-form, preserve existing style, avoid duplicates
			if len(containerOptions.Args) > 0 {
				existing := targetContainers[containerIndex].Args
				keyIndex := make(map[string]int)
				seenExact := make(map[string]bool)
				// index existing args by key; track pair-style positions
				for i := 0; i < len(existing); i++ {
					a := existing[i]
					seenExact[a] = true
					if !strings.HasPrefix(a, "-") {
						continue
					}
					if eq := strings.Index(a, "="); eq > 0 {
						keyIndex[a[:eq]] = i
						continue
					}
					// pair-style key then value
					if i+1 < len(existing) && !strings.HasPrefix(existing[i+1], "-") {
						keyIndex[a] = i
					}
				}
				// merge options
				for i := 0; i < len(containerOptions.Args); {
					a := containerOptions.Args[i]
					if strings.HasPrefix(a, "-") {
						// key=value from options
						if strings.Contains(a, "=") {
							k := a[:strings.Index(a, "=")]
							if pos, ok := keyIndex[k]; ok {
								// preserve existing style
								if existing[pos] == k {
									val := a[strings.Index(a, "=")+1:]
									if pos+1 < len(existing) && !strings.HasPrefix(existing[pos+1], "-") {
										existing[pos+1] = val
									} else {
										existing = append(existing[:pos+1], append([]string{val}, existing[pos+1:]...)...)
									}
								} else {
									existing[pos] = a
								}
							} else {
								keyIndex[k] = len(existing)
								existing = append(existing, a)
							}
							i++
							continue
						}
						// pair-style from options: key then value
						if i+1 < len(containerOptions.Args) && !strings.HasPrefix(containerOptions.Args[i+1], "-") {
							k, val := a, containerOptions.Args[i+1]
							if pos, ok := keyIndex[k]; ok {
								if existing[pos] == k {
									if pos+1 < len(existing) && !strings.HasPrefix(existing[pos+1], "-") {
										existing[pos+1] = val
									} else {
										existing = append(existing[:pos+1], append([]string{val}, existing[pos+1:]...)...)
									}
								} else {
									existing[pos] = k + "=" + val
								}
							} else {
								keyIndex[k] = len(existing)
								existing = append(existing, k, val)
							}
							i += 2
							continue
						}
						// standalone flag: exact dedupe
						if !seenExact[a] {
							seenExact[a] = true
							existing = append(existing, a)
						}
						i++
						continue
					}
					// non-flag token: exact-string dedupe
					if !seenExact[a] {
						seenExact[a] = true
						existing = append(existing, a)
					}
					i++
				}
				targetContainers[containerIndex].Args = existing
			}
		}
		// add the new container from the options list
		if !containerFound {
			containersToAdd = append(containersToAdd, containerOptions)
		}
	}
	targetContainers = append(targetContainers, containersToAdd...)
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

	// Compute a stable hash of the deployment spec and set it in a pod-template label
	// to force a rollout on spec changes.
	//
	// Label values are limited to 63 characters; SHA-256 hex output is 64 chars.
	// Use SHA-256 for FIPS compliance and truncate the hex string to fit as a label.
	fullHash, err := hash.Compute(deployment.Spec)
	if err != nil {
		return err
	}
	// Use a safe truncated prefix for the label value
	const hashLabelLen = 32
	hashValue := fullHash
	if len(hashValue) > hashLabelLen {
		hashValue = hashValue[:hashLabelLen]
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

	// update pod template labels
	err = ot.updateMapField(u, statefulSetOptions.Spec.Template.Labels, "spec", "template", "metadata", "labels")
	if err != nil {
		return err
	}

	// update pod template annotations
	err = ot.updateMapField(u, statefulSetOptions.Spec.Template.Annotations, "spec", "template", "metadata", "annotations")
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

	// update priorityClassName
	if statefulSetOptions.Spec.Template.Spec.PriorityClassName != "" {
		targetStatefulSet.Spec.Template.Spec.PriorityClassName = statefulSetOptions.Spec.Template.Spec.PriorityClassName

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

	// update runTimeClassName
	if statefulSetOptions.Spec.Template.Spec.RuntimeClassName != nil {
		targetStatefulSet.Spec.Template.Spec.RuntimeClassName = statefulSetOptions.Spec.Template.Spec.RuntimeClassName
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

func (ot *OptionsTransformer) updateHorizontalPodAutoscalers(u *unstructured.Unstructured) error {
	hpaOptions, found := ot.options.HorizontalPodAutoscalers[u.GetName()]
	if !found {
		return nil
	}

	// update labels
	err := ot.updateLabels(u, hpaOptions.Labels)
	if err != nil {
		return err
	}

	// update annotations
	err = ot.updateAnnotations(u, hpaOptions.Annotations)
	if err != nil {
		return err
	}

	// convert unstructured object to statefulSet
	targetHpa := autoscalingv2.HorizontalPodAutoscaler{}
	err = apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &targetHpa)
	if err != nil {
		return err
	}

	// update scaling target reference
	if hpaOptions.Spec.ScaleTargetRef.Kind != "" && hpaOptions.Spec.ScaleTargetRef.Name != "" {
		targetHpa.Spec.ScaleTargetRef = hpaOptions.Spec.ScaleTargetRef
	}

	// updates min replicas
	if hpaOptions.Spec.MinReplicas != nil {
		targetHpa.Spec.MinReplicas = ptr.Int32(*hpaOptions.Spec.MinReplicas)
	}

	// updates max replicas
	if hpaOptions.Spec.MaxReplicas > 0 {
		targetHpa.Spec.MaxReplicas = hpaOptions.Spec.MaxReplicas
	}

	// update metrics
	if len(hpaOptions.Spec.Metrics) > 0 {
		targetHpa.Spec.Metrics = hpaOptions.Spec.Metrics
	}

	// update behavior
	if hpaOptions.Spec.Behavior != nil {
		// update behavior, if empty
		if targetHpa.Spec.Behavior == nil {
			targetHpa.Spec.Behavior = &autoscalingv2.HorizontalPodAutoscalerBehavior{}
		}

		// update scaling down
		if hpaOptions.Spec.Behavior.ScaleDown != nil {
			targetHpa.Spec.Behavior.ScaleDown = hpaOptions.Spec.Behavior.ScaleDown.DeepCopy()
		}

		// update scaling up
		if hpaOptions.Spec.Behavior.ScaleUp != nil {
			targetHpa.Spec.Behavior.ScaleUp = hpaOptions.Spec.Behavior.ScaleUp.DeepCopy()
		}
	}

	// convert HorizontalPodAutoscaler to unstructured object
	obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(&targetHpa)
	if err != nil {
		return err
	}
	u.SetUnstructuredContent(obj)

	return nil
}

func (ot *OptionsTransformer) createHorizontalPodAutoscalers(manifest *mf.Manifest, targetNamespace string, additionalOptions v1alpha1.AdditionalOptions) ([]unstructured.Unstructured, error) {
	newHPAs := []unstructured.Unstructured{}
	existingHPAs := manifest.Filter(mf.Any(mf.ByKind(KindHorizontalPodAutoscaler)))
	for hpaName, newHPA := range additionalOptions.HorizontalPodAutoscalers {
		found := false
		for _, resource := range existingHPAs.Resources() {
			if resource.GetName() == hpaName {
				found = true
				break
			}
		}
		if found {
			continue
		}

		// update name
		newHPA.SetName(hpaName)

		// update the namespace to targetNamespace
		newHPA.SetNamespace(targetNamespace)

		// update kind
		if newHPA.TypeMeta.Kind == "" {
			newHPA.TypeMeta.Kind = KindHorizontalPodAutoscaler
		}

		// update api version
		if newHPA.TypeMeta.APIVersion == "" {
			newHPA.TypeMeta.APIVersion = autoscalingv2.SchemeGroupVersion.String()
		}

		// convert hpa to unstructured object
		obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(&newHPA)
		if err != nil {
			return nil, err
		}
		u := unstructured.Unstructured{}
		u.SetUnstructuredContent(obj)
		newHPAs = append(newHPAs, u)
	}

	return newHPAs, nil
}

func (ot *OptionsTransformer) updateWebhookConfiguration(u *unstructured.Unstructured) error {
	webhookOptions, found := ot.options.WebhookConfigurationOptions[u.GetName()]
	if !found {
		return nil
	}

	switch u.GetKind() {
	case KindValidatingWebhookConfiguration:
		targetWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{}
		err := apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, targetWebhookConfiguration)
		if err != nil {
			return err
		}
		for i, w := range targetWebhookConfiguration.Webhooks {
			if u.GetName() == w.Name {
				if webhookOptions.FailurePolicy != nil {
					targetWebhookConfiguration.Webhooks[i].FailurePolicy = webhookOptions.FailurePolicy
				}
				if webhookOptions.TimeoutSeconds != nil {
					targetWebhookConfiguration.Webhooks[i].TimeoutSeconds = webhookOptions.TimeoutSeconds
				}
				if webhookOptions.SideEffects != nil {
					targetWebhookConfiguration.Webhooks[i].SideEffects = webhookOptions.SideEffects
				}
			}
		}
		// convert webhookconfigurtion to unstructured object
		obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(targetWebhookConfiguration)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(obj)

	case KindMutatingWebhookConfiguration:
		targetWebhookConfiguration := &admissionregistrationv1.MutatingWebhookConfiguration{}
		err := apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, targetWebhookConfiguration)
		if err != nil {
			return err
		}
		for i, w := range targetWebhookConfiguration.Webhooks {
			if u.GetName() == w.Name {
				if webhookOptions.FailurePolicy != nil {
					targetWebhookConfiguration.Webhooks[i].FailurePolicy = webhookOptions.FailurePolicy
				}
				if webhookOptions.TimeoutSeconds != nil {
					targetWebhookConfiguration.Webhooks[i].TimeoutSeconds = webhookOptions.TimeoutSeconds
				}
				if webhookOptions.SideEffects != nil {
					targetWebhookConfiguration.Webhooks[i].SideEffects = webhookOptions.SideEffects
				}
			}
		}
		// convert webhookconfigurtion to unstructured object
		obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(targetWebhookConfiguration)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(obj)
	}

	return nil
}
