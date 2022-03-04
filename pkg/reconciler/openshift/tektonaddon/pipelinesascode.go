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
	"os"
	"path/filepath"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var pacLS = metav1.LabelSelector{
	MatchLabels: map[string]string{
		v1alpha1.InstallerSetType: PACInstallerSet,
	},
}

func (r *Reconciler) EnsurePipelinesAsCode(ctx context.Context, ta *v1alpha1.TektonAddon) error {

	pacLabelSelector, err := common.LabelSelector(pacLS)
	if err != nil {
		return err
	}

	if *ta.Spec.EnablePAC {

		exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.operatorVersion, pacLabelSelector)
		if err != nil {
			return err
		}
		if !exist {
			return r.ensurePAC(ctx, ta)
		}

	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, pacLabelSelector); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) ensurePAC(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	pacManifest := mf.Manifest{}

	koDataDir := os.Getenv(common.KoEnvKey)
	pacLocation := filepath.Join(koDataDir, "tekton-addon", "pipelines-as-code")
	if err := common.AppendManifest(&pacManifest, pacLocation); err != nil {
		return err
	}

	// installerSet adds it's owner as namespace's owner
	// so deleting tekton addon deletes target namespace too
	// to skip it we filter out namespace
	pacManifest = pacManifest.Filter(mf.Not(mf.ByKind("Namespace")))

	// Run transformers
	if err := r.addonTransform(ctx, &pacManifest, ta); err != nil {
		return err
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, pacManifest, r.operatorVersion,
		PACInstallerSet, "addon-pac"); err != nil {
		return err
	}

	return nil
}
