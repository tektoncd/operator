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
)

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
