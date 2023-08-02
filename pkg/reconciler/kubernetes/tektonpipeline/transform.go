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

package tektonpipeline

import (
	"context"
	"fmt"
	"sort"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
)

const (
	// Pipelines ConfigMap
	FeatureFlag                                  = "feature-flags"
	ConfigDefaults                               = "config-defaults"
	ConfigMetrics                                = "config-observability"
	ResolverFeatureFlag                          = "resolvers-feature-flags"
	bundleResolverConfig                         = "bundleresolver-config"
	clusterResolverConfig                        = "cluster-resolver-config"
	hubResolverConfig                            = "hubresolver-config"
	gitResolverConfig                            = "git-resolver-config"
	leaderElectionConfig                         = "config-leader-election"
	pipelinesControllerDeployment                = "tekton-pipelines-controller"
	pipelinesControllerContainer                 = "tekton-pipelines-controller"
	pipelinesRemoteResolversControllerDeployment = "tekton-pipelines-remote-resolvers"
	pipelinesRemoteResolverControllerContainer   = "controller"
	resolverEnvKeyTektonHubApi                   = "tekton-hub-api"
	resolverEnvKeyArtifactHubApi                 = "artifact-hub-api"
)

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		pipeline := comp.(*v1alpha1.TektonPipeline)

		images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.PipelinesImagePrefix))
		instance := comp.(*v1alpha1.TektonPipeline)
		// adding extension's transformers first to run them before `extra` transformers
		trns := extension.Transformers(instance)
		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdPipeline),
			common.AddConfigMapValues(FeatureFlag, pipeline.Spec.PipelineProperties),
			common.AddConfigMapValues(ConfigDefaults, pipeline.Spec.OptionalPipelineProperties),
			common.AddConfigMapValues(ConfigMetrics, pipeline.Spec.PipelineMetricsProperties),
			common.AddConfigMapValues(ResolverFeatureFlag, pipeline.Spec.Resolvers),
			common.DeploymentImages(images),
			common.InjectLabelOnNamespace(proxyLabel),
			common.AddConfiguration(pipeline.Spec.Config),
			common.CopyConfigMap(bundleResolverConfig, pipeline.Spec.BundlesResolverConfig),
			common.CopyConfigMap(hubResolverConfig, pipeline.Spec.HubResolverConfig),
			common.CopyConfigMap(clusterResolverConfig, pipeline.Spec.ClusterResolverConfig),
			common.CopyConfigMap(gitResolverConfig, pipeline.Spec.GitResolverConfig),
			common.AddConfigMapValues(leaderElectionConfig, pipeline.Spec.Performance.PipelinePerformanceLeaderElectionConfig),
			updatePerformanceFlagsInDeployment(pipeline),
			updateResolverConfigEnvironmentsInDeployment(pipeline),
		}
		trns = append(trns, extra...)

		if err := common.Transform(ctx, manifest, instance, trns...); err != nil {
			return &mf.Manifest{}, err
		}
		return manifest, nil
	}
}

// updates performance flags/args into pipelines controller container
func updatePerformanceFlagsInDeployment(pipelineCR *v1alpha1.TektonPipeline) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" || u.GetName() != pipelinesControllerDeployment {
			return nil
		}

		// holds the flags needs to be added in the container args section
		flags := map[string]interface{}{}

		// convert struct to map with json tag
		// so that, we can map the arguments as is
		performanceSpec := pipelineCR.Spec.Performance
		if err := common.StructToMap(&performanceSpec.PipelineDeploymentPerformanceArgs, &flags); err != nil {
			return err
		}

		// if there is no flags to update, return from here
		if len(flags) == 0 {
			return nil
		}

		// convert unstructured object to deployment
		dep := &appsv1.Deployment{}
		err := apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, dep)
		if err != nil {
			return err
		}

		// include config-leader-election data into deployment pod label
		// so that pods will be recreated, if there is a change in "buckets"
		// NOTE: when writing this code replicas can not be handled from installersets,
		// user needs to scale the deployment manually
		leaderElectionConfigMapData := map[string]interface{}{}
		if err = common.StructToMap(&performanceSpec.PipelinePerformanceLeaderElectionConfig, &leaderElectionConfigMapData); err != nil {
			return err
		}
		podLabels := dep.Spec.Template.Labels
		if podLabels == nil {
			podLabels = map[string]string{}
		}
		// sort data keys in an order, to get the consistent hash value in installerset
		labelKeys := getSortedKeys(leaderElectionConfigMapData)
		for _, key := range labelKeys {
			value := leaderElectionConfigMapData[key]
			labelKey := fmt.Sprintf("%s.data.%s", leaderElectionConfig, key)
			podLabels[labelKey] = fmt.Sprintf("%v", value)
		}
		dep.Spec.Template.Labels = podLabels

		// sort flag keys in an order, to get the consistent hash value in installerset
		flagKeys := getSortedKeys(flags)
		// update performance arguments into target container
		for containerIndex, container := range dep.Spec.Template.Spec.Containers {
			if container.Name != pipelinesControllerContainer {
				continue
			}
			for _, flagKey := range flagKeys {
				// update the arg name with "-" prefix
				expectedArg := fmt.Sprintf("-%s", flagKey)
				argStringValue := fmt.Sprintf("%v", flags[flagKey])
				argUpdated := false
				for argIndex, existingArg := range container.Args {
					if strings.HasPrefix(existingArg, expectedArg) {
						container.Args[argIndex] = fmt.Sprintf("%s=%s", expectedArg, argStringValue)
						argUpdated = true
						break
					}
				}
				if !argUpdated {
					container.Args = append(container.Args, fmt.Sprintf("%s=%s", expectedArg, argStringValue))
				}
			}
			dep.Spec.Template.Spec.Containers[containerIndex] = container
		}

		// convert deployment to unstructured object
		obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(dep)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(obj)

		return nil
	}
}

// updates resolver config environment variables
func updateResolverConfigEnvironmentsInDeployment(pipelineCR *v1alpha1.TektonPipeline) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" || u.GetName() != pipelinesRemoteResolversControllerDeployment {
			return nil
		}

		// holds the variables needs to be added in the container environment section
		envVariables := map[string]string{}

		// collect all the required environment keys
		rawEnvKeys := []string{resolverEnvKeyTektonHubApi, resolverEnvKeyArtifactHubApi}
		// get values from resolver config
		for _, rawEnvKey := range rawEnvKeys {
			if value, found := pipelineCR.Spec.ResolversConfig.HubResolverConfig[rawEnvKey]; found && value != "" {
				envVariables[rawEnvKey] = value
			}
		}

		// if there is no variables available to update, return from here
		if len(envVariables) == 0 {
			return nil
		}

		// update environment key to actual format
		// example: tekton-hub-api => TEKTON_HUB_API
		envKeys := []string{}
		for key, value := range envVariables {
			newKey := strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
			delete(envVariables, key)
			envVariables[newKey] = value
			envKeys = append(envKeys, newKey)
		}
		// sort the keys
		sort.Strings(envKeys)

		// convert unstructured object to deployment
		dep := &appsv1.Deployment{}
		err := apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, dep)
		if err != nil {
			return err
		}

		// update environment keys into the target container
		for containerIndex, container := range dep.Spec.Template.Spec.Containers {
			if container.Name != pipelinesRemoteResolverControllerContainer {
				continue
			}
			for _, envKey := range envKeys {
				envUpdated := false
				envVar := corev1.EnvVar{
					Name:  envKey,
					Value: envVariables[envKey],
				}
				for envIndex, existingEnv := range container.Env {
					if existingEnv.Name == envKey {
						container.Env[envIndex] = envVar
						envUpdated = true
						break
					}
				}
				if !envUpdated {
					container.Env = append(container.Env, envVar)
				}
			}
			dep.Spec.Template.Spec.Containers[containerIndex] = container
		}

		// convert deployment to unstructured object
		obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(dep)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(obj)

		return nil
	}
}

// sort keys in an order, to get the consistent hash value in installerset
func getSortedKeys(input map[string]interface{}) []string {
	keys := []string{}
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
