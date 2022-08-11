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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) EnsureVersionedClusterTask(ctx context.Context, enable string, ta *v1alpha1.TektonAddon) error {

	versionedClusterTaskLS := metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.InstallerSetType:       VersionedClusterTaskInstallerSet,
			v1alpha1.ReleaseMinorVersionKey: getPatchVersionTrimmed(r.operatorVersion),
		},
	}

	versionedClusterTaskLabelSelector, err := common.LabelSelector(versionedClusterTaskLS)
	if err != nil {
		return err
	}

	if enable == "true" {

		// here pass two labels one for type and other for minor release version to remove the previous minor release installerset only not all
		exist, _, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.operatorVersion, versionedClusterTaskLabelSelector)
		if err != nil {
			return err
		}

		if !exist {
			msg := fmt.Sprintf("%s being created/upgraded", VersionedClusterTaskInstallerSet)
			ta.Status.MarkInstallerSetNotReady(msg)
			return r.ensureVersionedClusterTasks(ctx, ta)
		}

		// here pass two labels one for type and other for operator release version to get the latest installerset of current version
		vClusterTaskLS := metav1.LabelSelector{
			MatchLabels: map[string]string{
				v1alpha1.InstallerSetType:  VersionedClusterTaskInstallerSet,
				v1alpha1.ReleaseVersionKey: r.operatorVersion,
			},
		}
		vClusterTaskLabelSelector, err := common.LabelSelector(vClusterTaskLS)
		if err != nil {
			return err
		}
		if err := r.checkComponentStatus(ctx, vClusterTaskLabelSelector); err != nil {
			ta.Status.MarkInstallerSetNotReady(err.Error())
			return nil
		}

	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, versionedClusterTaskLabelSelector); err != nil {
			return err
		}
	}

	return nil
}

// installerset for versioned clustertask like buildah-1-6-0
func (r *Reconciler) ensureVersionedClusterTasks(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	clusterTaskManifest := mf.Manifest{}
	// Read clusterTasks from ko data
	if err := applyAddons(&clusterTaskManifest, "02-clustertasks"); err != nil {
		return err
	}
	// Run transformers
	tfs := []mf.Transformer{
		replaceKind(KindTask, KindClusterTask),
		injectLabel(labelProviderType, providerTypeRedHat, overwrite, "ClusterTask"),
		setVersionedNames(r.operatorVersion),
	}
	if err := r.addonTransform(ctx, &clusterTaskManifest, ta, tfs...); err != nil {
		return err
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, clusterTaskManifest,
		r.operatorVersion, VersionedClusterTaskInstallerSet, "addon-versioned-clustertasks"); err != nil {
		return err
	}

	return nil
}
