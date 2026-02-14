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
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/multiclusterproxyaae"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/pipeline"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/pruner"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/result"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/scheduler"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/syncerservice"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/trigger"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/upgrade"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
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
var (
	_ tektonConfigreconciler.Interface = (*Reconciler)(nil)
	_ tektonConfigreconciler.Finalizer = (*Reconciler)(nil)
)

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
		if err := result.EnsureTektonResultCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonResults()); err != nil {
			return err
		}
		if err := syncerservice.EnsureSyncerServiceCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().SyncerServices()); err != nil {
			return err
		}
		if err := pipeline.EnsureTektonPipelineCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonPipelines()); err != nil {
			return err
		}
		if err := multiclusterproxyaae.EnsureTektonMulticlusterProxyAAECRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonMulticlusterProxyAAEs()); err != nil {
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
	logger := logging.FromContext(ctx).With("tektonconfig", tc.Name)
	tc.Status.InitializeConditions()
	tc.Status.SetVersion(r.operatorVersion)

	logger.Debugw("Starting TektonConfig reconciliation",
		"version", r.operatorVersion,
		"profile", tc.Spec.Profile,
		"status", tc.Status.GetCondition(apis.ConditionReady))

	if tc.GetName() != v1alpha1.ConfigResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.ConfigResourceName,
			tc.GetName(),
		)
		logger.Errorw("Invalid resource name", "expectedName", v1alpha1.ConfigResourceName, "actualName", tc.GetName())
		tc.Status.MarkNotReady(msg)
		return nil
	}

	// run pre upgrade
	if err := r.upgrade.RunPreUpgrade(ctx); err != nil {
		logger.Errorw("Pre-upgrade failed", "error", err)
		return err
	}
	logger.Debug("Pre-upgrade completed successfully")

	// Mark TektonConfig Instance as Not Ready if an upgrade is needed
	if err := r.markUpgrade(ctx, tc); err != nil {
		logger.Errorw("Failed to mark upgrade status", "error", err)
		return err
	}

	// reconcile target namespace
	nsMetaLabels := map[string]string{}
	nsMetaAnnotations := map[string]string{}
	if tc.Spec.TargetNamespaceMetadata != nil {
		nsMetaLabels = tc.Spec.TargetNamespaceMetadata.Labels
		nsMetaAnnotations = tc.Spec.TargetNamespaceMetadata.Annotations
	}
	logger.Debugw("Reconciling target namespace",
		"labelCount", len(nsMetaLabels),
		"annotationCount", len(nsMetaAnnotations))

	if err := common.ReconcileTargetNamespace(ctx, nsMetaLabels, nsMetaAnnotations, tc, r.kubeClientSet); err != nil {
		logger.Errorw("Failed to reconcile target namespace", "error", err)
		return err
	}
	logger.Debug("Target namespace reconciled successfully")

	// Pre-reconcile extension hooks
	if err := r.extension.PreReconcile(ctx, tc); err != nil {
		if err == v1alpha1.RECONCILE_AGAIN_ERR {
			logger.Infow("Extensions requested requeue")
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Errorw("Pre-install hook failed", "error", err)
		tc.Status.MarkPreInstallFailed(err.Error())
		return err
	}

	tc.Status.MarkPreInstallComplete()
	logger.Debug("Pre-install completed successfully")

	// Ensure Pipeline CR
	tektonpipeline := pipeline.GetTektonPipelineCR(tc, r.operatorVersion)
	logger.Debug("Ensuring TektonPipeline CR exists")
	if _, err := pipeline.EnsureTektonPipelineExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonPipelines(), tektonpipeline); err != nil {
		errMsg := fmt.Sprintf("TektonPipeline: %s", err.Error())
		logger.Errorw("Failed to ensure TektonPipeline exists", "error", err)
		tc.Status.MarkComponentNotReady(errMsg)
		if err == v1alpha1.RECONCILE_AGAIN_ERR {
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		return nil
	}
	logger.Debug("TektonPipeline CR reconciled successfully")

	// Start Event based Pruner only if old Job based Pruner is Disabled.
	if tc.Spec.TektonPruner.IsDisabled() {
		logger.Debugw("TektonPruner is disabled. Shutting down event based pruner")
		if err := pruner.EnsureTektonPrunerCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonPruners()); err != nil {
			tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonPruner: %s", err.Error()))
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	} else if !tc.Spec.Pruner.Disabled {
		msg := "Invalid Pruner Configuration!! Both pruners, tektonpruner(event based) and pruner(job based) cannot be enabled simultaneously. Please disable one of them."
		logger.Error(msg)
		tc.Status.MarkComponentNotReady(msg)
		return v1alpha1.REQUEUE_EVENT_AFTER
	} else {
		logger.Infof("TektonPruner is enabled.Creating TektonPipeline CR")
		tektonPruner := pruner.GetTektonPrunerCR(tc, r.operatorVersion)
		if _, err := pruner.EnsureTektonPrunerExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonPruners(), tektonPruner); err != nil {
			tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonPruner %s", err.Error()))
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}

	// Ensure TektonMulticlusterProxyAAE CR (conditional based on scheduler multi-cluster config).
	// Run before EnsureSchedulerComponent so the CR is created even when scheduler component
	// is blocked (e.g. cert-manager or Kueue not installed). Multicluster-proxy-aae is deployed only when:
	// - Scheduler is enabled (not disabled)
	// - multi-cluster-disabled: false
	// - multi-cluster-role: Hub
	proxyAAEEnabled := multiclusterproxyaae.IsMulticlusterProxyAAEEnabled(tc)
	logger.Infow("TektonMulticlusterProxyAAE enablement",
		"enabled", proxyAAEEnabled,
		"schedulerDisabled", tc.Spec.Scheduler.IsDisabled(),
		"multiClusterDisabled", tc.Spec.Scheduler.MultiClusterDisabled,
		"multiClusterRole", tc.Spec.Scheduler.MultiClusterRole)
	if proxyAAEEnabled {
		proxyCR := multiclusterproxyaae.GetTektonMulticlusterProxyAAECR(tc, r.operatorVersion)
		logger.Debug("Ensuring TektonMulticlusterProxyAAE CR exists (multi-cluster enabled with Hub role)")
		if _, err := multiclusterproxyaae.EnsureTektonMulticlusterProxyAAEExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonMulticlusterProxyAAEs(), proxyCR); err != nil {
			if err == v1alpha1.RECONCILE_AGAIN_ERR {
				return v1alpha1.REQUEUE_EVENT_AFTER
			}
			errMsg := fmt.Sprintf("TektonMulticlusterProxyAAE: %s", err.Error())
			logger.Errorw("Failed to ensure TektonMulticlusterProxyAAE exists", "error", err)
			tc.Status.MarkComponentNotReady(errMsg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Debug("TektonMulticlusterProxyAAE CR reconciled successfully")
	} else {
		logger.Debugw("Ensuring TektonMulticlusterProxyAAE CR doesn't exist",
			"schedulerDisabled", tc.Spec.Scheduler.IsDisabled(),
			"multiClusterDisabled", tc.Spec.Scheduler.MultiClusterDisabled,
			"multiClusterRole", tc.Spec.Scheduler.MultiClusterRole)
		if err := multiclusterproxyaae.EnsureTektonMulticlusterProxyAAECRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonMulticlusterProxyAAEs()); err != nil {
			if err == v1alpha1.RECONCILE_AGAIN_ERR {
				return v1alpha1.REQUEUE_EVENT_AFTER
			}
			errMsg := fmt.Sprintf("TektonMulticlusterProxyAAE: %s", err.Error())
			logger.Errorw("Failed to ensure TektonMulticlusterProxyAAE has been deleted", "error", err)
			tc.Status.MarkComponentNotReady(errMsg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Debug("TektonMulticlusterProxyAAE CR removal reconciled successfully")
	}

	if err := r.EnsureSchedulerComponent(ctx, tc); err != nil {
		return err
	}

	// Ensure Pipeline Trigger
	if !tc.Spec.Trigger.Disabled && (tc.Spec.Profile == v1alpha1.ProfileAll || tc.Spec.Profile == v1alpha1.ProfileBasic) {
		tektontrigger := trigger.GetTektonTriggerCR(tc, r.operatorVersion)
		logger.Debug("Ensuring TektonTrigger CR exists")
		if _, err := trigger.EnsureTektonTriggerExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonTriggers(), tektontrigger); err != nil {
			errMsg := fmt.Sprintf("TektonTrigger: %s", err.Error())
			logger.Errorw("Failed to ensure TektonTrigger exists", "error", err)
			tc.Status.MarkComponentNotReady(errMsg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Debug("TektonTrigger CR reconciled successfully")
	} else {
		logger.Debugw("Ensuring TektonTrigger CR doesn't exist", "profile", tc.Spec.Profile, "triggerDisabled", tc.Spec.Trigger.Disabled)
		if err := trigger.EnsureTektonTriggerCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonTriggers()); err != nil {
			errMsg := fmt.Sprintf("TektonTrigger: %s", err.Error())
			logger.Errorw("Failed to ensure TektonTrigger has been deleted", "error", err)
			tc.Status.MarkComponentNotReady(errMsg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Debug("TektonTrigger CR removal reconciled successfully")
	}

	// Ensure Chain CR
	if !tc.Spec.Chain.Disabled {
		tektonchain := chain.GetTektonChainCR(tc, r.operatorVersion)
		logger.Debug("Ensuring TektonChain CR exists")
		if _, err := chain.EnsureTektonChainExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonChains(), tektonchain); err != nil {
			errMsg := fmt.Sprintf("TektonChain: %s", err.Error())
			logger.Errorw("Failed to ensure TektonChain exists", "error", err)
			tc.Status.MarkComponentNotReady(errMsg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Debug("TektonChain CR reconciled successfully")
	} else {
		logger.Debugw("Ensuring TektonChain CR doesn't exist", "chainDisabled", tc.Spec.Chain.Disabled)
		if err := chain.EnsureTektonChainCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonChains()); err != nil {
			errMsg := fmt.Sprintf("TektonChain: %s", err.Error())
			logger.Errorw("Failed to ensure TektonChain has been deleted", "error", err)
			tc.Status.MarkComponentNotReady(errMsg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Debug("TektonChain CR removal reconciled successfully")
	}

	// Ensure Result CR
	if !tc.Spec.Result.Disabled {
		tektonresult := result.GetTektonResultCR(tc, r.operatorVersion)
		logger.Debug("Ensuring TektonResult CR exists")
		if _, err := result.EnsureTektonResultExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonResults(), tektonresult); err != nil {
			errMsg := fmt.Sprintf("TektonResult %s", err.Error())
			logger.Errorw("Failed to ensure TektonResult exists", "error", err)
			tc.Status.MarkComponentNotReady(errMsg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Debug("TektonResult CR reconciled successfully")
	} else {
		logger.Debugw("Ensuring TektonResult CR doesn't exist", "resultDisabled", tc.Spec.Result.Disabled)
		if err := result.EnsureTektonResultCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().TektonResults()); err != nil {
			errMsg := fmt.Sprintf("TektonResult: %s", err.Error())
			logger.Errorw("Failed to ensure TektonResult has been deleted", "error", err)
			tc.Status.MarkComponentNotReady(errMsg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Debug("TektonResult CR removal reconciled successfully")
	}

	// Ensure SyncerService CR (conditional based on scheduler multi-cluster config)
	// Syncer-service is deployed only when:
	// - Scheduler is enabled (not disabled)
	// - multi-cluster-disabled: false
	// - multi-cluster-role: Hub
	if syncerservice.IsSyncerServiceEnabled(&tc.Spec.Scheduler) {
		syncerServiceCR := syncerservice.GetSyncerServiceCR(tc, r.operatorVersion)
		logger.Debug("Ensuring SyncerService CR exists (multi-cluster enabled with Hub role)")
		if _, err := syncerservice.EnsureSyncerServiceExists(ctx, r.operatorClientSet.OperatorV1alpha1().SyncerServices(), syncerServiceCR); err != nil {
			errMsg := fmt.Sprintf("SyncerService: %s", err.Error())
			logger.Errorw("Failed to ensure SyncerService exists", "error", err)
			tc.Status.MarkComponentNotReady(errMsg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Debug("SyncerService CR reconciled successfully")
	} else {
		logger.Debugw("Ensuring SyncerService CR doesn't exist",
			"schedulerDisabled", tc.Spec.Scheduler.IsDisabled(),
			"multiClusterDisabled", tc.Spec.Scheduler.MultiClusterDisabled,
			"multiClusterRole", tc.Spec.Scheduler.MultiClusterRole)
		if err := syncerservice.EnsureSyncerServiceCRNotExists(ctx, r.operatorClientSet.OperatorV1alpha1().SyncerServices()); err != nil {
			errMsg := fmt.Sprintf("SyncerService: %s", err.Error())
			logger.Errorw("Failed to ensure SyncerService has been deleted", "error", err)
			tc.Status.MarkComponentNotReady(errMsg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Debug("SyncerService CR removal reconciled successfully")
	}

	// Ensure Pruner
	if !tc.Spec.Pruner.Disabled {
		logger.Debugw("Reconciling pruner installer set", "prunerDisabled", tc.Spec.Pruner.Disabled)
		err := r.reconcilePrunerInstallerSet(ctx, tc)
		if err != nil {
			logger.Errorw("Failed to reconcile pruner installer set", "error", err)
			return err
		}
		logger.Debug("Pruner installer set reconciled successfully")
	}

	// Run resource pruning
	if err := common.Prune(ctx, r.kubeClientSet, tc); err != nil {
		errMsg := fmt.Sprintf("tekton-resource-pruner: %s", err.Error())
		logger.Errorw("Resource pruning failed", "error", err)
		tc.Status.MarkComponentNotReady(errMsg)
	} else {
		logger.Debug("Resource pruning completed successfully")
	}

	tc.Status.MarkComponentsReady()
	logger.Debug("All components marked ready")

	// Post-reconcile extension hooks
	if err := r.extension.PostReconcile(ctx, tc); err != nil {
		logger.Errorw("Post-reconcile hook failed", "error", err)
		return err
	}

	tc.Status.MarkPostInstallComplete()
	logger.Debug("Post-install completed successfully")

	// Update the object for any spec changes
	logger.Debug("Updating TektonConfig status")
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonConfigs().UpdateStatus(ctx, tc, metav1.UpdateOptions{}); err != nil {
		logger.Errorw("Failed to update TektonConfig status", "error", err)
		return err
	}
	logger.Debug("TektonConfig status updated successfully")

	// run post upgrade
	if err := r.upgrade.RunPostUpgrade(ctx); err != nil {
		logger.Errorw("Post-upgrade failed", "error", err)
		return err
	}
	logger.Debug("Post-upgrade completed successfully")

	logger.Debugw("TektonConfig reconciliation completed successfully",
		"status", tc.Status.GetCondition(apis.ConditionReady))
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

func (r *Reconciler) EnsureSchedulerComponent(ctx context.Context, tc *v1alpha1.TektonConfig) error {
	return scheduler.EnsureTektonComponent(ctx, tc, r.operatorClientSet, r.operatorVersion)
}
