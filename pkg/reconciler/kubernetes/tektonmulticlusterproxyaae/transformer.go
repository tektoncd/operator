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

package tektonmulticlusterproxyaae

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
)

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		proxyCR := comp.(*v1alpha1.TektonMulticlusterProxyAAE)

		imagesRaw := common.ToLowerCaseKeys(common.ImagesFromEnv(common.MulticlusterProxyAAEImagePrefix))
		images := common.ImageRegistryDomainOverride(imagesRaw)
		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.MultiClusterProxyAAEResourceName),
			common.DeploymentImages(images),
			common.AddDeploymentRestrictedPSA(),
		}
		extra = append(extra, extension.Transformers(proxyCR)...)
		err := common.Transform(ctx, manifest, proxyCR, extra...)
		if err != nil {
			return &mf.Manifest{}, err
		}

		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, proxyCR.Spec.GetTargetNamespace(), proxyCR.Spec.Options); err != nil {
			return &mf.Manifest{}, err
		}

		// Now Remove the TargetNamespace from manifest as same is not owned by TektonMulticlusterProxyAAE.
		filteredManifest := manifest.Filter(mf.Not(mf.ByKind("Namespace")), mf.Not(mf.ByName(proxyCR.Spec.GetTargetNamespace())))

		return &filteredManifest, nil
	}
}
