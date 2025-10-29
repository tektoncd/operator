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
	ConfigTracing                                = "config-tracing"
	ResolverFeatureFlag                          = "resolvers-feature-flags"
	bundleResolverConfig                         = "bundleresolver-config"
	clusterResolverConfig                        = "cluster-resolver-config"
	hubResolverConfig                            = "hubresolver-config"
	gitResolverConfig                            = "git-resolver-config"
	leaderElectionPipelineConfig                 = "config-leader-election-controller"
	leaderElectionResolversConfig                = "config-leader-election-resolvers"
	pipelinesControllerDeployment                = "tekton-pipelines-controller"
	pipelinesControllerContainer                 = "tekton-pipelines-controller"
	pipelinesRemoteResolversControllerDeployment = "tekton-pipelines-remote-resolvers"
	pipelinesRemoteResolverControllerContainer   = "controller"
	resolverEnvKeyTektonHubApi                   = "tekton-hub-api"
	resolverEnvKeyArtifactHubApi                 = "artifact-hub-api"

	tektonPipelinesControllerName                      = "tekton-pipelines-controller"
	tektonPipelinesServiceName                         = "tekton-pipelines-controller"
	tektonRemoteResolversControllerName                = "tekton-pipelines-remote-resolvers"
	tektonRemoteResolversServiceName                   = "tekton-pipelines-remote-resolvers"
	tektonPipelinesControllerStatefulServiceName       = "STATEFUL_SERVICE_NAME"
	tektonPipelinesControllerStatefulControllerOrdinal = "STATEFUL_CONTROLLER_ORDINAL"
)

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		pipeline := comp.(*v1alpha1.TektonPipeline)

		// not in use, see: https://github.com/tektoncd/pipeline/pull/7789
		// this field is removed from pipeline component
		// still keeping types to maintain the API compatibility
		pipeline.Spec.Pipeline.EnableTektonOciBundles = nil

		imagesRaw := common.ToLowerCaseKeys(common.ImagesFromEnv(common.PipelinesImagePrefix))
		images := common.ImageRegistryDomainOverride(imagesRaw)
		instance := comp.(*v1alpha1.TektonPipeline)
		// adding extension's transformers first to run them before `extra` transformers
		trns := extension.Transformers(instance)
		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdPipeline),
			common.AddConfigMapValues(FeatureFlag, pipeline.Spec.PipelineProperties),
			common.AddConfigMapValues(ConfigDefaults, pipeline.Spec.OptionalPipelineProperties),
			common.AddConfigMapValues(ConfigMetrics, pipeline.Spec.PipelineMetricsProperties),
			addTracingConfigValues(pipeline),
			common.AddConfigMapValues(ResolverFeatureFlag, pipeline.Spec.Resolvers),
			common.DeploymentImages(images),
			common.StatefulSetImages(images),
			common.DeploymentEnvVarKubernetesMinVersion(),
			common.InjectLabelOnNamespace(proxyLabel),
			common.AddConfiguration(pipeline.Spec.Config),
			common.CopyConfigMap(bundleResolverConfig, pipeline.Spec.BundlesResolverConfig),
			common.CopyConfigMap(hubResolverConfig, pipeline.Spec.HubResolverConfig),
			common.CopyConfigMap(clusterResolverConfig, pipeline.Spec.ClusterResolverConfig),
			common.CopyConfigMap(gitResolverConfig, pipeline.Spec.GitResolverConfig),
			common.AddConfigMapValues(leaderElectionPipelineConfig, pipeline.Spec.Performance.PerformanceLeaderElectionConfig),
			common.AddConfigMapValues(leaderElectionResolversConfig, pipeline.Spec.Performance.PerformanceLeaderElectionConfig),
			common.UpdatePerformanceFlagsInDeploymentAndLeaderConfigMap(&pipeline.Spec.Performance, leaderElectionPipelineConfig, pipelinesControllerDeployment, pipelinesControllerContainer),
			common.UpdatePerformanceFlagsInDeploymentAndLeaderConfigMap(&pipeline.Spec.Performance, leaderElectionResolversConfig, pipelinesRemoteResolversControllerDeployment, pipelinesRemoteResolverControllerContainer),
			updateResolverConfigEnvironmentsInDeployment(pipeline),
		}
		if pipeline.Spec.Performance.StatefulsetOrdinals != nil && *pipeline.Spec.Performance.StatefulsetOrdinals {
			extra = append(extra, common.ConvertDeploymentToStatefulSet(tektonPipelinesControllerName, tektonPipelinesServiceName), common.AddStatefulEnvVars(
				tektonPipelinesControllerName, tektonPipelinesServiceName, tektonPipelinesControllerStatefulServiceName, tektonPipelinesControllerStatefulControllerOrdinal))
			extra = append(extra, common.ConvertDeploymentToStatefulSet(tektonRemoteResolversControllerName, tektonRemoteResolversServiceName), common.AddStatefulEnvVars(
				tektonRemoteResolversControllerName, tektonRemoteResolversServiceName, tektonPipelinesControllerStatefulServiceName, tektonPipelinesControllerStatefulControllerOrdinal))
		}

		trns = append(trns, extra...)

		if err := common.Transform(ctx, manifest, instance, trns...); err != nil {
			return &mf.Manifest{}, err
		}

		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, pipeline.Spec.GetTargetNamespace(), pipeline.Spec.Options); err != nil {
			return &mf.Manifest{}, err
		}

		return manifest, nil
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

// addTracingConfigValues adds tracing configuration to config-tracing ConfigMap
// It strips the "traces." prefix from the JSON tags to match upstream expectations
func addTracingConfigValues(pipelineCR *v1alpha1.TektonPipeline) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" || u.GetName() != ConfigTracing {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}

		// Map traces.enabled -> enabled
		if pipelineCR.Spec.TracingProperties.Enabled != nil {
			if *pipelineCR.Spec.TracingProperties.Enabled {
				cm.Data["enabled"] = "true"
			} else {
				cm.Data["enabled"] = "false"
			}
		}

		if pipelineCR.Spec.TracingProperties.Endpoint != "" {
			cm.Data["endpoint"] = pipelineCR.Spec.TracingProperties.Endpoint
		}

		if pipelineCR.Spec.TracingProperties.CredentialsSecret != "" {
			cm.Data["credentialsSecret"] = pipelineCR.Spec.TracingProperties.CredentialsSecret
		}

		obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(obj)

		return nil
	}
}
