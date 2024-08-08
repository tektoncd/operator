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

package tektonaddon

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
)

func (r *Reconciler) EnsureVersionedClusterTask(ctx context.Context, enable string, ta *v1alpha1.TektonAddon) error {
	manifest := *r.clusterTaskManifest
	if enable == "true" {
		addonImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.AddonsImagePrefix))
		tfs := []mf.Transformer{
			replaceKind(KindTask, KindClusterTask),
			injectLabel(labelProviderType, providerTypeRedHat, overwrite, KindClusterTask),
			common.TaskImages(ctx, addonImages),
			setVersionedNames(r.operatorVersion),
		}
		if err := r.installerSetClient.VersionedTaskSet(ctx, ta, &manifest, filterAndTransformClusterTask(tfs),
			VersionedClusterTaskInstallerSet, installerSetNameForClusterTasks); err != nil {
			return err
		}
	} else {
		if err := r.installerSetClient.CleanupCustomSet(ctx, VersionedClusterTaskInstallerSet); err != nil {
			return err
		}
	}
	return nil
}
