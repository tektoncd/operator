/*
Copyright 2026 The Tekton Authors

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

package syncerservice

import (
	"context"
	"fmt"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

const createdByValue = "SyncerService"

var (
	ls = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: v1alpha1.SyncerServiceResourceName,
		},
	}
)

func (r *Reconciler) ensureInstallerSet(ctx context.Context, ss *v1alpha1.SyncerService) error {
	logger := logging.FromContext(ctx)

	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}

	existingInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
	if err != nil {
		return err
	}

	if existingInstallerSet == "" {
		logger.Info("No existing installer set found, creating new one")
		createdIs, err := r.createInstallerSet(ctx, ss)
		if err != nil {
			return err
		}
		r.updateStatus(ss, createdIs)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	installedTIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Get(ctx, existingInstallerSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			createdIs, err := r.createInstallerSet(ctx, ss)
			if err != nil {
				return err
			}
			r.updateStatus(ss, createdIs)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		return err
	}

	installerSetTargetNamespace := installedTIS.Annotations[v1alpha1.TargetNamespaceKey]
	installerSetReleaseVersion := installedTIS.Labels[v1alpha1.ReleaseVersionKey]

	if installerSetTargetNamespace != ss.Spec.TargetNamespace || installerSetReleaseVersion != r.operatorVersion {
		logger.Infow("Configuration changed, deleting existing installer set",
			"existingNamespace", installerSetTargetNamespace,
			"newNamespace", ss.Spec.TargetNamespace)
		if err := r.deleteInstallerSet(ctx, existingInstallerSet); err != nil {
			return err
		}
		ss.Status.MarkNotReady("Waiting for previous installer set to get deleted")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	// Check spec hash
	expectedSpecHash, err := hash.Compute(ss.Spec)
	if err != nil {
		return err
	}

	lastAppliedHash := installedTIS.GetAnnotations()[v1alpha1.LastAppliedHashKey]
	if lastAppliedHash != expectedSpecHash {
		logger.Infow("SyncerService spec changed, updating installer set")
		manifest := r.manifest
		if err := r.transform(ctx, &manifest, ss); err != nil {
			return err
		}

		current := installedTIS.GetAnnotations()
		current[v1alpha1.LastAppliedHashKey] = expectedSpecHash
		installedTIS.SetAnnotations(current)
		installedTIS.Spec.Manifests = manifest.Resources()

		_, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Update(ctx, installedTIS, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	r.updateStatus(ss, installedTIS)
	ss.Status.MarkInstallerSetAvailable()

	ready := installedTIS.Status.GetCondition(apis.ConditionReady)
	if ready == nil || ready.Status == "Unknown" {
		ss.Status.MarkInstallerSetNotReady("Waiting for installation")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	if ready.Status == "False" {
		ss.Status.MarkInstallerSetNotReady(ready.Message)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	ss.Status.MarkInstallerSetReady()
	return nil
}

func (r *Reconciler) createInstallerSet(ctx context.Context, ss *v1alpha1.SyncerService) (*v1alpha1.TektonInstallerSet, error) {
	manifest := r.manifest

	if err := r.transform(ctx, &manifest, ss); err != nil {
		return nil, err
	}

	specHash, err := hash.Compute(ss.Spec)
	if err != nil {
		return nil, err
	}

	tis := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", v1alpha1.SyncerServiceResourceName),
			Labels: map[string]string{
				v1alpha1.CreatedByKey:      createdByValue,
				v1alpha1.InstallerSetType:  v1alpha1.SyncerServiceResourceName,
				v1alpha1.ReleaseVersionKey: r.operatorVersion,
			},
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: ss.Spec.TargetNamespace,
				v1alpha1.LastAppliedHashKey: specHash,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ss, ss.GetGroupVersionKind()),
			},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}

	return r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Create(ctx, tis, metav1.CreateOptions{})
}

func (r *Reconciler) deleteInstallerSet(ctx context.Context, name string) error {
	return r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Delete(ctx, name, metav1.DeleteOptions{})
}

func (r *Reconciler) updateStatus(ss *v1alpha1.SyncerService, installerSet *v1alpha1.TektonInstallerSet) {
	ss.Status.SetSyncerServiceInstallerSet(installerSet.Name)
	ss.Status.SetVersion(r.syncerVersion)
}
