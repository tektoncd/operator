/*
Copyright 2025 The Tekton Authors

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

package tektonpruner

import (
	"context"
	"errors"
	"fmt"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/pruner/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

const (
	PrunerConfigInstallerSet = "pruner-config"
	CreatedByValue           = "TektonConfig"
	PrunerConfigMapName      = config.PrunerConfigMapName
)

func (r *Reconciler) ensureInstallerSets(ctx context.Context, tp *v1alpha1.TektonPruner) error {
	logger := logging.FromContext(ctx)
	filteredManifest := r.manifest.Filter(mf.Not(mf.All(mf.ByKind("ConfigMap"), mf.ByName(config.PrunerConfigMapName))))
	if err := r.installerSetClient.MainSet(ctx, tp, &filteredManifest, filterAndTransform(r.extension)); err != nil {
		msg := fmt.Sprintf("Main Reconcilation failed: %s", err.Error())
		logger.Error(msg)
		if errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
			return err
		}
		tp.Status.MarkInstallerSetNotReady(msg)
	}

	if err := r.ensureConfigInstallerSet(ctx, tp); err != nil {
		msg := fmt.Sprintf("Config InstallerSet Reconcilation failed: %s", err.Error())
		logger.Error(msg)
		if errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
			return err
		}
		tp.Status.MarkInstallerSetNotReady(msg)

	}
	return nil
}

func (r *Reconciler) ensureConfigInstallerSet(ctx context.Context, tp *v1alpha1.TektonPruner) error {
	logger := logging.FromContext(ctx)
	labelSelector := metav1.LabelSelector{
		MatchLabels: getLabels(),
	}
	configLabelSector, err := common.LabelSelector(labelSelector)
	if err != nil {
		logger.Errorw("Invalid Pruner config label selector", "error", err)
		return err
	}
	existingConfigInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, configLabelSector)
	if err != nil {
		logger.Errorw("Failed to get config installer set name", "error", err, "selector", configLabelSector)
		return err
	}
	if existingConfigInstallerSet == "" {
		tp.Status.MarkInstallerSetNotAvailable(PrunerConfigInstallerSet + " InstallerSet not available")
		logger.Infow("Creating new InstallerSet", PrunerConfigInstallerSet, "targetNamespace", tp.Spec.TargetNamespace)

		_, err := r.createConfigInstallerSet(ctx, tp)
		if err != nil {
			logger.Errorw("Failed to create Config InstallerSet", "error", err)
			return err
		}

	} else {
		// If exists, then fetch the Tekton Pruner Config InstallerSet
		installedConfigTIS, _ := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, existingConfigInstallerSet, metav1.GetOptions{})

		configInstallerSetTargetNamespace := installedConfigTIS.Annotations[v1alpha1.TargetNamespaceKey]
		configInstallerSetReleaseVersion := installedConfigTIS.Labels[v1alpha1.ReleaseVersionKey]

		if configInstallerSetTargetNamespace != tp.Spec.TargetNamespace || configInstallerSetReleaseVersion != r.operatorVersion {
			logger.Infow("Config InstallerSet needs update",
				"name", existingConfigInstallerSet,
				"currentNamespace", configInstallerSetTargetNamespace,
				"expectedNamespace", tp.Spec.TargetNamespace,
				"currentVersion", configInstallerSetReleaseVersion,
				"expectedVersion", r.operatorVersion)

			// Delete the existing Tekton Pruner InstallerSet
			err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
				Delete(ctx, existingConfigInstallerSet, metav1.DeleteOptions{})
			if err != nil {
				logger.Errorw("Failed to delete Config InstallerSet", "name", existingConfigInstallerSet, "error", err)
				return err
			}

			// Make sure the Tekton Pruner Config InstallerSet is deleted
			_, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
				Get(ctx, existingConfigInstallerSet, metav1.GetOptions{})
			if err == nil {
				tp.Status.MarkNotReady("Waiting for previous installer set to get deleted")
				logger.Debugw("Config InstallerSet deletion pending", "name", existingConfigInstallerSet)
				return v1alpha1.REQUEUE_EVENT_AFTER
			}
			if !apierrors.IsNotFound(err) {
				logger.Errorw("Failed to confirm Config InstallerSet deletion", "name", existingConfigInstallerSet, "error", err)
				return err
			}
			return nil

		} else {
			// If target namespace and version are not changed then check if Pruner
			// spec is changed by checking hash stored as annotation on
			// Tekton Pruner InstallerSet with computing new hash of TektonPruner Spec

			// Hash of TektonPruner Spec
			expectedSpecHash, err := hash.Compute(tp.Spec)
			if err != nil {
				logger.Errorw("Failed to compute spec hash", "error", err)
				return err
			}

			// spec hash stored on installerSet
			lastAppliedHash := installedConfigTIS.GetAnnotations()[v1alpha1.LastAppliedHashKey]
			if lastAppliedHash != expectedSpecHash {
				logger.Infow("Config spec changed, updating InstallerSet",
					"name", installedConfigTIS.Name,
					"oldHash", lastAppliedHash,
					"newHash", expectedSpecHash)
				if err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
					Delete(ctx, installedConfigTIS.Name, metav1.DeleteOptions{}); err != nil {
					logger.Errorw("Failed to delete outdated Config InstallerSet", "name", installedConfigTIS.Name, "error", err)
					return err
				}

				// after updating installer set enqueue after a duration
				// to allow changes to get deployed
				logger.Infow("Config InstallerSet deleted to apply spec changes", "name", installedConfigTIS.Name)
				return v1alpha1.REQUEUE_EVENT_AFTER
			}
			logger.Debugw("Config InstallerSet up to date", "name", installedConfigTIS.Name)
		}
	}
	return nil
}

func (r *Reconciler) createConfigInstallerSet(ctx context.Context, tektonPruner *v1alpha1.TektonPruner) (*v1alpha1.TektonInstallerSet, error) {
	logger := logging.FromContext(ctx)
	manifest := r.manifest
	manifest = manifest.Filter(mf.ByKind("ConfigMap"), mf.ByName(config.PrunerConfigMapName))

	logger.Infow("Creating a new ConfigInstallerSet", "manifest", manifest.Resources())

	transformer := filterAndTransform(r.extension)
	if _, err := transformer(ctx, &manifest, tektonPruner); err != nil {
		tektonPruner.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// generate installer set
	tis := r.makeInstallerSet(tektonPruner, manifest, PrunerConfigInstallerSet)

	// compute the hash of  spec and store as an annotation
	// in further reconciliation we compute hash of tektonPruner spec and check with
	// annotation, if they are same then we skip updating the object
	// otherwise we update the manifest
	specHash, err := hash.Compute(tektonPruner.Spec)
	if err != nil {
		return nil, err
	}
	tis.Annotations[v1alpha1.LastAppliedHashKey] = specHash

	// create installer set
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, tis, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdIs, nil
}

func (r *Reconciler) makeInstallerSet(tc *v1alpha1.TektonPruner, manifest mf.Manifest, installerSetType string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(tc, tc.GetGroupVersionKind())
	labels := getLabels()

	labels[v1alpha1.ReleaseVersionKey] = r.operatorVersion

	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", installerSetType),
			Labels:       labels,
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: tc.Spec.TargetNamespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}

func getLabels() map[string]string {
	labels := map[string]string{
		v1alpha1.CreatedByKey:     CreatedByValue,
		v1alpha1.InstallerSetType: PrunerConfigInstallerSet,
	}
	return labels
}
