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
)

var clusterTaskLS = metav1.LabelSelector{
	MatchLabels: map[string]string{
		v1alpha1.InstallerSetType: ClusterTaskInstallerSet,
	},
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
	tfs := []mf.Transformer{
		replaceKind(KindTask, KindClusterTask),
		injectLabel(labelProviderType, providerTypeRedHat, overwrite, "ClusterTask"),
	}
	if err := r.addonTransform(ctx, &clusterTaskManifest, ta, tfs...); err != nil {
		return err
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, clusterTaskManifest,
		r.operatorVersion, ClusterTaskInstallerSet, "addon-clustertasks"); err != nil {
		return err
	}

	return nil
}

func formattedVersionMajorMinorX(version, x string) string {
	ver := getPatchVersionTrimmed(version)
	ver = fmt.Sprintf("%s.%s", ver, x)
	return formattedVersionSnake(ver)
}

func formattedVersionMajorMinor(version string) string {
	ver := getPatchVersionTrimmed(version)
	return formattedVersionSnake(ver)
}

func formattedVersionSnake(version string) string {
	ver := strings.TrimPrefix(version, "v")
	return strings.Replace(ver, ".", "-", -1)
}

// To get the minor major version for label i.e. v1.6
func getPatchVersionTrimmed(version string) string {
	endIndex := strings.LastIndex(version, ".")
	if endIndex != -1 {
		version = version[:endIndex]
	}
	return version
}
