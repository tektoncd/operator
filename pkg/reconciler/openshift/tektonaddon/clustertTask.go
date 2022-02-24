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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"knative.dev/pkg/logging"
)

var clusterTaskLS = metav1.LabelSelector{
	MatchLabels: map[string]string{
		v1alpha1.InstallerSetType: ClusterTaskInstallerSet,
	},
}

// byContains returns resources with specific string in name
func byContains(name string) mf.Predicate {
	return func(u *unstructured.Unstructured) bool {
		return strings.Contains(u.GetName(), name)
	}
}

var communityResourceURLs = []string{
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/jib-maven/0.4/jib-maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/maven/0.2/maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-source/0.3/helm-upgrade-from-source.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-repo/0.2/helm-upgrade-from-repo.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/trigger-jenkins-job/0.1/trigger-jenkins-job.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/git-cli/0.3/git-cli.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/pull-request/0.1/pull-request.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/kubeconfig-creator/0.1/kubeconfig-creator.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/main/task/argocd-task-sync-and-wait/0.1/argocd-task-sync-and-wait.yaml",
}

func (r *Reconciler) EnsureClusterTask(ctx context.Context, enable string, ta *v1alpha1.TektonAddon) error {

	clusterTaskLabelSelector, err := common.LabelSelector(clusterTaskLS)
	if err != nil {
		return err
	}

	if enable == "true" {

		exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.operatorVersion, clusterTaskLabelSelector)
		if err != nil {
			return err
		}

		if !exist {
			msg := fmt.Sprintf("%s being created/upgraded", ClusterTaskInstallerSet)
			ta.Status.MarkInstallerSetNotReady(msg)
			return r.ensureClusterTasks(ctx, ta)
		}

		if err := r.checkComponentStatus(ctx, clusterTaskLabelSelector); err != nil {
			ta.Status.MarkInstallerSetNotReady(err.Error())
			return nil
		}

	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, clusterTaskLabelSelector); err != nil {
			return err
		}
	}

	return nil
}

// installerset for non versioned clustertask like buildah and community clustertask
func (r *Reconciler) ensureClusterTasks(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	clusterTaskManifest := mf.Manifest{}
	// Read clusterTasks from ko data
	if err := applyAddons(&clusterTaskManifest, "02-clustertasks"); err != nil {
		return err
	}
	// Run transformers
	if err := r.addonTransform(ctx, &clusterTaskManifest, ta); err != nil {
		return err
	}

	clusterTaskManifest = clusterTaskManifest.Filter(
		mf.Not(byContains(getFormattedVersion(r.operatorVersion))),
	)

	communityClusterTaskManifest := r.manifest
	if err := r.appendCommunityTarget(ctx, &communityClusterTaskManifest, ta); err != nil {
		// Continue if failed to resolve community task URL.
		// (Ex: on disconnected cluster community tasks won't be reachable because of proxy).
		logging.FromContext(ctx).Error("Failed to get community task: Skipping community tasks installation  ", err)
	} else {
		if err := r.communityTransform(ctx, &communityClusterTaskManifest, ta); err != nil {
			return err
		}

		clusterTaskManifest = clusterTaskManifest.Append(communityClusterTaskManifest)
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, clusterTaskManifest,
		r.operatorVersion, ClusterTaskInstallerSet, "addon-clustertasks"); err != nil {
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

// To get the version in the format as in clustertask name i.e. 1-6
func getFormattedVersion(version string) string {
	version = strings.TrimPrefix(getPatchVersionTrimmed(version), "v")
	return strings.Replace(version, ".", "-", -1)
}

// To get the minor major version for label i.e. v1.6
func getPatchVersionTrimmed(version string) string {
	endIndex := strings.LastIndex(version, ".")
	if endIndex != -1 {
		version = version[:endIndex]
	}
	return version
}
