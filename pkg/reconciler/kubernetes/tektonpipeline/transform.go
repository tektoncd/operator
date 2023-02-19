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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
)

const (
	// Pipelines ConfigMap
	FeatureFlag           = "feature-flags"
	ConfigDefaults        = "config-defaults"
	ConfigMetrics         = "config-observability"
	ResolverFeatureFlag   = "resolvers-feature-flags"
	bundleResolverConfig  = "bundleresolver-config"
	clusterResolverConfig = "cluster-resolver-config"
	hubResolverConfig     = "hubresolver-config"
	gitResolverConfig     = "git-resolver-config"
)

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		pipeline := comp.(*v1alpha1.TektonPipeline)

		filteredManifest := manifest.Filter(mf.Not(mf.ByKind("PodSecurityPolicy")), mf.Not(mf.ByKind("HorizontalPodAutoscaler")))

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
			common.ApplyProxySettings,
			common.DeploymentImages(images),
			common.InjectLabelOnNamespace(proxyLabel),
			common.AddConfiguration(pipeline.Spec.Config),
			common.HighAvailabilityTransform(pipeline.Spec.Config.HighAvailability),
			common.DeploymentOverrideTransform(pipeline.Spec.Config.DeploymentOverride),
			common.CopyConfigMap(bundleResolverConfig, pipeline.Spec.BundlesResolverConfig),
			common.CopyConfigMap(hubResolverConfig, pipeline.Spec.HubResolverConfig),
			common.CopyConfigMap(clusterResolverConfig, pipeline.Spec.ClusterResolverConfig),
			common.CopyConfigMap(gitResolverConfig, pipeline.Spec.GitResolverConfig),
		}
		trns = append(trns, extra...)

		if err := common.Transform(ctx, &filteredManifest, instance, trns...); err != nil {
			return &mf.Manifest{}, err
		}
		return &filteredManifest, nil
	}
}
