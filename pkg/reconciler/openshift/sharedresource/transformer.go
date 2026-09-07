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

package sharedresource

import (
	"context"

	"github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
)

// FilterAndTransform applies the standard component transformers to the Shared
// Resource manifests before they are handed to the InstallerSet client.
func FilterAndTransform(reconciler *Reconciler) client.FilterAndTransform {
	return func(ctx context.Context, manifest *manifestival.Manifest, component v1alpha1.TektonComponent) (*manifestival.Manifest, error) {
		sr := component.(*v1alpha1.SharedResource)
		*manifest = manifest.Filter(manifestival.Not(manifestival.ByKind("Namespace")))
		images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.ShipwrightBuildImagePrefix))

		transformers := []manifestival.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(OperandName),
			common.DeploymentImages(images),
			common.DeploymentEnvVarKubernetesMinVersion(),
		}

		transformers = append(transformers, reconciler.extension.Transformers(sr)...)
		if err := common.Transform(ctx, manifest, sr, transformers...); err != nil {
			return nil, err
		}

		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, sr.Spec.GetTargetNamespace(), sr.Spec.Options); err != nil {
			return nil, err
		}
		return manifest, nil
	}
}
