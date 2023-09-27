/*
Copyright 2021 The Tekton Authors

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
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonConfigreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/chain"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/pipeline"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/trigger"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/upgrade"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonConfig resources.
type Reconciler struct {
	// kubeClientSet allows us to talk to the k8s for core APIs
	kubeClientSet kubernetes.Interface
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// Platform-specific behavior to affect the transform
	extension       common.Extension
	manifest        mf.Manifest
	operatorVersion string
	// performs pre and post upgrade operations
	upgrade *upgrade.Upgrade
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonConfigreconciler.Interface = (*Reconciler)(nil)
var _ tektonConfigreconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a TektonConfig.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonConfig) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	if original.Spec.Profile == v1alpha1.ProfileLite {
		return pipeline.EnsureTektonPipelineCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonPipelines())
	} else {
		// TektonPipeline and TektonTrigger is common for profile type basic and all
		if err := trigger.EnsureTektonTriggerCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonTriggers()); err != nil {
			return err
		}
		if err := chain.EnsureTektonChainCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonChains()); err != nil {
			return err
		}
		if err := pipeline.EnsureTektonPipelineCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonPipelines()); err != nil {
			return err
		}
	}

	// remove pruner tektonInstallerSet
	labelSelector, err := common.LabelSelector(prunerInstallerSetLabel)
	if err != nil {
		return err
	}
	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: labelSelector},
	); err != nil {
		logger.Error("failed to delete pruner installerSet", err)
		return err
	}

	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tc *v1alpha1.TektonConfig) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tc.Status.InitializeConditions()
	tc.Status.SetVersion(r.operatorVersion)

	logger.Infow("Reconciling TektonConfig", "status", tc.Status)
	if tc.GetName() != v1alpha1.ConfigResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.ConfigResourceName,
			tc.GetName(),
		)
		logger.Error(msg)
		tc.Status.MarkNotReady(msg)
		return nil
	}

	// run pre upgrade
	err := r.upgrade.RunPreUpgrade(ctx)
	if err != nil {
		return err
	}

	tc.SetDefaults(ctx)
	// Mark TektonConfig Instance as Not Ready if an upgrade is needed
	if err := r.markUpgrade(ctx, tc); err != nil {
		return err
	}

	// reconcile target namespace
	if err := common.ReconcileTargetNamespace(ctx, nil, tc, r.kubeClientSet); err != nil {
		return err
	}

	if err := r.extension.PreReconcile(ctx, tc); err != nil {
		if err == v1alpha1.RECONCILE_AGAIN_ERR {
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		tc.Status.MarkPreInstallFailed(err.Error())
		return err
	}

	tc.Status.MarkPreInstallComplete()

	// Ensure if the pipeline CR already exists, if not create Pipeline CR
	tektonpipeline := pipeline.GetTektonPipelineCR(tc)
	// Ensure it exists
	if _, err := pipeline.EnsureTektonPipelineExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonPipelines(), tektonpipeline); err != nil {
		tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonPipeline: %s", err.Error()))
		if err == v1alpha1.RECONCILE_AGAIN_ERR {
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		return nil
	}

	// Create TektonTrigger CR if the profile is all or basic
	if tc.Spec.Profile == v1alpha1.ProfileAll || tc.Spec.Profile == v1alpha1.ProfileBasic {
		tektontrigger := trigger.GetTektonTriggerCR(tc)
		if _, err := trigger.EnsureTektonTriggerExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonTriggers(), tektontrigger); err != nil {
			tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonTrigger: %s", err.Error()))
			return v1alpha1.REQUEUE_EVENT_AFTER

		}
	} else {
		if err := trigger.EnsureTektonTriggerCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonTriggers()); err != nil {
			tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonTrigger: %s", err.Error()))
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}

	if !tc.Spec.Chain.Disabled {
		tektonchain := chain.GetTektonChainCR(tc)
		if _, err := chain.EnsureTektonChainExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonChains(), tektonchain); err != nil {
			tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonChain: %s", err.Error()))
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	} else {
		if err := chain.EnsureTektonChainCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonChains()); err != nil {
			tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonChain: %s", err.Error()))
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}

	// reconcile pruner installerSet
	if !tc.Spec.Pruner.Disabled {
		err := r.reconcilePrunerInstallerSet(ctx, tc)
		if err != nil {
			return err
		}
	}

	if err := common.Prune(ctx, r.kubeClientSet, tc); err != nil {
		tc.Status.MarkComponentNotReady(fmt.Sprintf("tekton-resource-pruner: %s", err.Error()))
		logger.Error(err)
	}

	tc.Status.MarkComponentsReady()

	if err := r.extension.PostReconcile(ctx, tc); err != nil {
		return err
	}

	tc.Status.MarkPostInstallComplete()

	// Update the object for any spec changes
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonConfigs().Update(ctx, tc, metav1.UpdateOptions{}); err != nil {
		return err
	}

	// run post upgrade
	err = r.upgrade.RunPostUpgrade(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) markUpgrade(ctx context.Context, tc *v1alpha1.TektonConfig) error {
	labels := tc.GetLabels()
	ver, ok := labels[v1alpha1.ReleaseVersionKey]
	if ok && ver == r.operatorVersion {
		return nil
	}
	if ok && ver != r.operatorVersion {
		tc.Status.MarkComponentNotReady("Upgrade Pending")
		tc.Status.MarkPreInstallFailed(v1alpha1.UpgradePending)
		tc.Status.MarkPostInstallFailed(v1alpha1.UpgradePending)
		tc.Status.MarkNotReady("Upgrade Pending")
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels[v1alpha1.ReleaseVersionKey] = r.operatorVersion
	tc.SetLabels(labels)

	// Update the object for any spec changes
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonConfigs().Update(ctx, tc, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}
