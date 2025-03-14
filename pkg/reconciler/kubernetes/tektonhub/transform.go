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

package tektonhub

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"knative.dev/pkg/logging"
)

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		logger := logging.FromContext(ctx)
		hubCR := comp.(*v1alpha1.TektonHub)

		images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.HubImagePrefix))
		trans := extension.Transformers(hubCR)
		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdHub),
			mf.InjectOwner(hubCR),
			mf.InjectNamespace(hubCR.Spec.GetTargetNamespace()),
			common.DeploymentImages(images),
			common.DeploymentEnvVarKubernetesMinVersion(),
			common.JobImages(images),
			updateApiConfigMap(hubCR, apiConfigMapName),
			addConfigMapKeyValue(uiConfigMapName, "API_URL", hubCR.Status.ApiRouteUrl),
			addConfigMapKeyValue(uiConfigMapName, "AUTH_BASE_URL", hubCR.Status.AuthRouteUrl),
			addConfigMapKeyValue(uiConfigMapName, "API_VERSION", "v1"),
			addConfigMapKeyValue(uiConfigMapName, "REDIRECT_URI", hubCR.Status.UiRouteUrl),
			addConfigMapKeyValue(uiConfigMapName, "CUSTOM_LOGO_BASE64_DATA", hubCR.Spec.CustomLogo.Base64Data),
			addConfigMapKeyValue(uiConfigMapName, "CUSTOM_LOGO_MEDIA_TYPE", hubCR.Spec.CustomLogo.MediaType),
			common.AddDeploymentRestrictedPSA(),
			common.AddJobRestrictedPSA(),
		}

		trans = append(trans, extra...)

		err := common.Transform(ctx, manifest, hubCR, trans...)
		if err != nil {
			logger.Error("failed to transform manifest")
			return &mf.Manifest{}, err
		}

		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, hubCR.Spec.GetTargetNamespace(), hubCR.Spec.Options); err != nil {
			return &mf.Manifest{}, err
		}

		return manifest, nil
	}
}
