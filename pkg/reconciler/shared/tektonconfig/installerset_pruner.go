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

package tektonconfig

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

const (
	yamlDirNamePrunerManifest = "tekton-pruner"
	labelCreatedByValue       = "TektonConfig"
)

var (
	prunerInstallerSetLabel = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     labelCreatedByValue,
			v1alpha1.InstallerSetType: v1alpha1.PrunerResourceName,
		},
	}

	prunerYamlDirHashFunc sync.Once
	prunerManifestHash    prunerManifestSpec
)

type prunerManifestSpec struct {
	YamlLocation string
	ComputedHash string
}

// reconciles pruner InstallerSets
// for pruner we manage RBAC and ServiceAccount via installer sets
// RBAC and ServiceAccount details are in yaml file and it is located in "config/pruner/00-pruner.yaml"
// in the runtime container the directory will be as "$KO_DATA_PATH/tekton-pruner/"
func (r *Reconciler) reconcilePrunerInstallerSet(ctx context.Context, tc *v1alpha1.TektonConfig) error {
	// we have to calculate hash for the entire pruner yaml directory to confirm the changes.
	// reads all yaml files from the directory and computes hash, it is expensive process to access disk on each call.
	// hence calculate only once at startup, it helps not to degrade the performance of the reconcile loop
	// also it not necessary to read the files frequently, as the files are shipped along the container and never change
	prunerYamlDirHashFunc.Do(func() {
		yamlDirLocation := filepath.Join(common.ComponentBaseDir(), yamlDirNamePrunerManifest)
		computedHash, err := hash.ComputeHashDir(yamlDirLocation, "/tekton-pruner-")
		if err != nil {
			logger := logging.FromContext(ctx)
			logger.Errorw("error on calculating hash for pruner manifest yaml directory",
				"directory", yamlDirLocation,
				err,
			)
		}
		prunerManifestHash = prunerManifestSpec{
			YamlLocation: yamlDirLocation,
			ComputedHash: computedHash,
		}
	})

	// report error if the hash not calculated
	// actual error will be printer on the log from above func on the first call of this method
	if prunerManifestHash.ComputedHash == "" {
		return errors.New("error on calculation hash for pruner manifest yaml directory")
	}

	// verify availability of pruner InstallerSet
	labelSelector, err := common.LabelSelector(prunerInstallerSetLabel)
	if err != nil {
		return err
	}
	actualInstallerSetName, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
	if err != nil {
		return err
	}

	createInstallerSet := false
	if actualInstallerSetName == "" {
		// set create installerSet flag
		createInstallerSet = true
	}

	if !createInstallerSet {
		// get the existing installerSet and compare the hash value
		// if it is mismatch, have to delete the old one and create new one with the supplied yaml file
		actualInstallerSet, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Get(ctx, actualInstallerSetName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		appliedHash, found := actualInstallerSet.GetAnnotations()[v1alpha1.LastAppliedHashKey]
		if !found || prunerManifestHash.ComputedHash != appliedHash {
			// delete the existing installerSet
			if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Delete(ctx, actualInstallerSetName, metav1.DeleteOptions{}); err != nil {
				return err
			}
			// set create installerSet flag
			createInstallerSet = true
		}
	}

	if createInstallerSet {
		// create installerSet with changes
		return r.createPrunerInstallerSet(ctx, tc)
	}

	return nil
}

func (r *Reconciler) createPrunerInstallerSet(ctx context.Context, tc *v1alpha1.TektonConfig) error {
	// get new manifest
	manifest := r.manifest.Append()

	// add resources to manifest from yaml files
	if err := common.AppendManifest(&manifest, prunerManifestHash.YamlLocation); err != nil {
		return err
	}

	// apply transformers
	if err := r.transformPruner(ctx, &manifest, tc); err != nil {
		tc.Status.MarkNotReady("transformation failed: " + err.Error())
		return err
	}

	// setup installerSet
	ownerRef := *metav1.NewControllerRef(tc, tc.GetGroupVersionKind())
	prunerInstallerSet := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", v1alpha1.PrunerResourceName),
			Labels: map[string]string{
				v1alpha1.CreatedByKey:      labelCreatedByValue,
				v1alpha1.InstallerSetType:  v1alpha1.PrunerResourceName,
				v1alpha1.ReleaseVersionKey: r.operatorVersion,
			},
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: tc.Spec.TargetNamespace,
				v1alpha1.LastAppliedHashKey: prunerManifestHash.ComputedHash,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			// include resources from manifest
			Manifests: manifest.Resources(),
		},
	}

	// creates installerSet in the cluster
	_, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Create(ctx, prunerInstallerSet, metav1.CreateOptions{})
	return err
}

// mutates the passed manifest with list of transformers
func (r *Reconciler) transformPruner(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	extra := []mf.Transformer{
		common.InjectOperandNameLabelOverwriteExisting(v1alpha1.PrunerResourceName),
	}
	extra = append(extra, r.extension.Transformers(comp)...)
	return common.Transform(ctx, manifest, comp, extra...)
}
