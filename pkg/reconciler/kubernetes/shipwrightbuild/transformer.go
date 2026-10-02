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

package shipwrightbuild

import (
	"context"

	"github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
)

// FilterAndTransform installs the Builds manifests into the openshift-builds
// namespace. The upstream operator applies the same InjectNamespace transform;
// removeRunAsUserRunAsGroup mirrors its behavior of letting the OpenShift SCC
// controller assign the runAsUser/runAsGroup so the pods pass the restricted
// SCC admission.
func FilterAndTransform(reconciler *Reconciler) client.FilterAndTransform {
	return func(ctx context.Context, manifest *manifestival.Manifest, component v1alpha1.TektonComponent) (*manifestival.Manifest, error) {
		sb := component.(*v1alpha1.ShipwrightBuild)
		*manifest = manifest.Filter(manifestival.Not(manifestival.ByKind("Namespace")))
		webhookArgs := []string{WebhookVersionFlag, reconciler.componentVersion}
		images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.ShipwrightBuildImagePrefix))

		transformers := []manifestival.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(OperandName),
			common.DeploymentImages(images),
			common.DeploymentEnvVarKubernetesMinVersion(),
			common.AddContainerArgs(WebhookDeploymentName, WebhookContainerName, webhookArgs, false),
		}

		transformers = append(transformers, reconciler.extension.Transformers(sb)...)
		err := common.Transform(ctx, manifest, sb, transformers...)
		if err != nil {
			return nil, err
		}

		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, sb.Spec.GetTargetNamespace(), sb.Spec.Options); err != nil {
			return nil, err
		}
		return manifest, nil
	}
}

func NilFilterAndTransform(reconciler *Reconciler) client.FilterAndTransform {
	return func(ctx context.Context, manifest *manifestival.Manifest, component v1alpha1.TektonComponent) (*manifestival.Manifest, error) {
		return manifest, nil
	}
}
