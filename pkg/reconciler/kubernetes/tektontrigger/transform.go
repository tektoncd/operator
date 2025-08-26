/*
Copyright 2020 The Tekton Authors

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
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
)

// Triggers ConfigMap
const (
	ConfigDefaults = "config-defaults-triggers"
	FeatureFlag    = "feature-flags-triggers"
)

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		trigger := comp.(*v1alpha1.TektonTrigger)

		imagesRaw := common.ToLowerCaseKeys(common.ImagesFromEnv(common.TriggersImagePrefix))
		triggerImages := common.ImageRegistryDomainOverride(imagesRaw)

		// adding extension's transformers first to run them before `extra` transformers
		trns := extension.Transformers(trigger)
		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdTriggers),
			common.AddConfigMapValues(ConfigDefaults, trigger.Spec.OptionalTriggersProperties),
			common.AddConfigMapValues(FeatureFlag, trigger.Spec.TriggersProperties),
			common.DeploymentImages(triggerImages),
			common.DeploymentEnvVarKubernetesMinVersion(),
			common.AddConfiguration(trigger.Spec.Config),
		}
		trns = append(trns, extra...)
		if err := common.Transform(ctx, manifest, trigger, trns...); err != nil {
			return &mf.Manifest{}, err
		}

		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, trigger.Spec.GetTargetNamespace(), trigger.Spec.Options); err != nil {
			return &mf.Manifest{}, err
		}

		return manifest, nil
	}
}
