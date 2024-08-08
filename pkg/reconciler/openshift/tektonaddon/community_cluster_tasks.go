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
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
)

var communityResourceURLs = []string{
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/jib-maven/0.5/jib-maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-source/0.3/helm-upgrade-from-source.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-repo/0.2/helm-upgrade-from-repo.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/trigger-jenkins-job/0.1/trigger-jenkins-job.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/git-cli/0.4/git-cli.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/pull-request/0.1/pull-request.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/kubeconfig-creator/0.1/kubeconfig-creator.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/main/task/argocd-task-sync-and-wait/0.2/argocd-task-sync-and-wait.yaml",
}

func (r *Reconciler) EnsureCommunityClusterTask(ctx context.Context, enable string, ta *v1alpha1.TektonAddon) error {
	if len(r.communityClusterTaskManifest.Resources()) == 0 {
		return nil
	}
	manifest := *r.communityClusterTaskManifest
	if enable == "true" {
		if err := r.installerSetClient.CustomSet(ctx, ta, CommunityClusterTaskInstallerSet, &manifest, filterAndTransformCommunityClusterTask(), nil); err != nil {
			return err
		}
	} else {
		if err := r.installerSetClient.CleanupCustomSet(ctx, CommunityClusterTaskInstallerSet); err != nil {
			return err
		}
	}
	return nil
}

func filterAndTransformCommunityClusterTask() client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		instance := comp.(*v1alpha1.TektonAddon)
		addonImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.AddonsImagePrefix))

		extra := []mf.Transformer{
			replaceKind("Task", "ClusterTask"),
			injectLabel(labelProviderType, providerTypeCommunity, overwrite, "ClusterTask"),
			common.TaskImages(ctx, addonImages),
		}
		if err := common.Transform(ctx, manifest, instance, extra...); err != nil {
			return nil, err
		}
		return manifest, nil
	}
}

func appendCommunityTasks(manifest *mf.Manifest) error {
	urls := strings.Join(communityResourceURLs, ",")
	m, err := mf.ManifestFrom(mf.Path(urls))
	if err != nil {
		return err
	}
	*manifest = manifest.Append(m)
	return nil
}

func fetchCommunityTasks(manifest *mf.Manifest) error {
	if err := appendCommunityTasks(manifest); err != nil {
		return err
	}
	return nil
}
