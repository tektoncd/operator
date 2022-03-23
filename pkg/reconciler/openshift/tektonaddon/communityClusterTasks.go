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
	"fmt"
	"strings"
	"time"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

var communityClusterTaskLS = metav1.LabelSelector{
	MatchLabels: map[string]string{
		v1alpha1.InstallerSetType: CommunityClusterTaskInstallerSet,
	},
}

// retryWaitTime holds value until the pod lives.
// ones the pod restart resets the value back to ""
// this is useful if the pod is not restarted and community task urls are not reachable
// we see comparable less forbidden events
var (
	layout        = "2006-01-02 15:04:05"
	retryWaitTime = ""
)

var communityResourceURLs = []string{
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/jib-maven/0.4/jib-maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-source/0.3/helm-upgrade-from-source.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-repo/0.2/helm-upgrade-from-repo.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/trigger-jenkins-job/0.1/trigger-jenkins-job.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/git-cli/0.3/git-cli.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/pull-request/0.1/pull-request.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/kubeconfig-creator/0.1/kubeconfig-creator.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/main/task/argocd-task-sync-and-wait/0.2/argocd-task-sync-and-wait.yaml",
}

func (r *Reconciler) EnsureCommunityClusterTask(ctx context.Context, enable string, ta *v1alpha1.TektonAddon) error {

	communityClusterTaskLabelSelector, err := common.LabelSelector(communityClusterTaskLS)
	if err != nil {
		return err
	}

	if enable == "true" {

		exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.operatorVersion, communityClusterTaskLabelSelector)
		if err != nil {
			return err
		}

		if !exist {
			msg := fmt.Sprintf("%s being created/upgraded", CommunityClusterTaskInstallerSet)
			ta.Status.MarkInstallerSetNotReady(msg)
			return r.ensureCommunityClusterTasks(ctx, ta)
		}

		if err := r.checkComponentStatus(ctx, communityClusterTaskLabelSelector); err != nil {
			ta.Status.MarkInstallerSetNotReady(err.Error())
			return nil
		}

	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, communityClusterTaskLabelSelector); err != nil {
			return err
		}
	}

	return nil
}

// installerset for communityclustertask
func (r *Reconciler) ensureCommunityClusterTasks(ctx context.Context, ta *v1alpha1.TektonAddon) error {

	if SkipCommunityTaskFetch(retryWaitTime) {
		return nil
	}

	communityClusterTaskManifest := mf.Manifest{}
	if err := r.appendCommunityTarget(ctx, &communityClusterTaskManifest, ta); err != nil {
		retryWaitTime = time.Now().Add(15 * time.Minute).Format(layout)

		// Continue if failed to resolve community task URL.
		// (Ex: on disconnected cluster community tasks won't be reachable because of proxy).
		logging.FromContext(ctx).Error("Failed to get community task: Skipping community tasks installation  ", err)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	if err := r.communityTransform(ctx, &communityClusterTaskManifest, ta); err != nil {
		return err
	}

	communityClusterTaskManifest = communityClusterTaskManifest.Append(communityClusterTaskManifest)

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, communityClusterTaskManifest, r.operatorVersion, CommunityClusterTaskInstallerSet, "addon-communityclustertasks"); err != nil {
		return err
	}

	return nil
}

// appendCommunityTarget mutates the passed manifest by appending one
// appropriate for the passed TektonComponent
func (r *Reconciler) appendCommunityTarget(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	urls := strings.Join(communityResourceURLs, ",")
	m, err := mf.ManifestFrom(mf.Path(urls))
	if err != nil {
		return err
	}
	*manifest = manifest.Append(m)
	return nil
}

// communityTransform mutates the passed manifest to one with common component
// and platform transformations applied
func (r *Reconciler) communityTransform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	instance := comp.(*v1alpha1.TektonAddon)
	extra := []mf.Transformer{
		replaceKind("Task", "ClusterTask"),
		injectLabel(labelProviderType, providerTypeCommunity, overwrite, "ClusterTask"),
	}
	extra = append(extra, r.extension.Transformers(instance)...)
	return common.Transform(ctx, manifest, instance, extra...)
}

// SkipCommunityTaskFetch skips community task fetch until retryWaitTime has passed
// if there is no value int retryWaitTime that means no error occurred prior to the call
// if there is some value that means error has occurred earlier and must try after 15 minutes
func SkipCommunityTaskFetch(until string) bool {
	skip := false
	if until != "" {
		t, err := time.Parse(layout, until)
		if err != nil {
			return false
		}
		if t.After(time.Now()) {
			skip = true
		}
	}
	return skip
}
