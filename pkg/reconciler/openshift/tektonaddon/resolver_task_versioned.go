/*
Copyright 2024 The Tekton Authors

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

package tektonaddon

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
)

func (r *Reconciler) EnsureVersionedResolverTask(ctx context.Context, enable string, ta *v1alpha1.TektonAddon) error {
	manifest := r.resolverTaskManifest
	return r.ensureVersionedCustomSet(ctx, enable, VersionedResolverTaskInstallerSet, installerSetNameForResolverTasks, KindTask, ta, manifest)
}

func (r *Reconciler) EnsureVersionedResolverStepAction(ctx context.Context, enable string, ta *v1alpha1.TektonAddon) error {
	manifest := r.resolverStepActionManifest
	return r.ensureVersionedCustomSet(ctx, enable, VersionedResolverStepActionInstallerSet, installerSetNameForResolverStepAction, KindStepAction, ta, manifest)
}

func (r *Reconciler) ensureVersionedCustomSet(ctx context.Context, enable, installerSetType, installerSetName, kind string, ta *v1alpha1.TektonAddon, manifest *mf.Manifest) error {
	if enable == "true" {
		addonImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.AddonsImagePrefix))
		tfs := []mf.Transformer{
			injectLabel(labelProviderType, providerTypeRedHat, overwrite, kind),
			common.TaskImages(addonImages),
			setVersionedNames(r.operatorVersion),
		}
		if err := r.installerSetClient.VersionedTaskSet(ctx, ta, manifest, filterAndTransformResolverTask(tfs),
			installerSetType, installerSetName); err != nil {
			return err
		}
	} else {
		if err := r.installerSetClient.CleanupCustomSet(ctx, installerSetType); err != nil {
			return err
		}
	}
	return nil
}
