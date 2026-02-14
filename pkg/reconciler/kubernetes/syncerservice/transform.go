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

package syncerservice

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
)

func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, ss *v1alpha1.SyncerService) error {
	// Filter out Namespace resources - namespace is managed by TektonConfig, not by component manifests
	// This prevents the namespace from being deleted when SyncerService CR is removed
	*manifest = manifest.Filter(mf.Not(mf.ByKind("Namespace")))

	syncerImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.SyncerServiceImagePrefix))

	extra := []mf.Transformer{
		common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandSyncerService),
		common.ApplyProxySettings,
		common.AddDeploymentRestrictedPSA(),
		common.AddConfiguration(ss.Spec.Config),
		common.DeploymentImages(syncerImages),
	}

	extra = append(extra, r.extension.Transformers(ss)...)
	err := common.Transform(ctx, manifest, ss, extra...)
	if err != nil {
		return err
	}

	// additional options transformer
	// always execute as last transformer, so that the values in options will be final update values on the manifests
	if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, ss.Spec.GetTargetNamespace(), ss.Spec.Options); err != nil {
		return err
	}

	return nil
}
