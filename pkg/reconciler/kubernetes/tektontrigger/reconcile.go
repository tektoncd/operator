/*
Copyright 2020 The Tekton Authors

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

package tektontrigger

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektontriggerreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektontrigger"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonTrigger resources.
type Reconciler struct {
	// installer Set client to do CRUD operations for components
	installerSetClient *client.InstallerSetClient
	// pipelineInformer to query for TektonPipeline
	pipelineInformer pipelineinformer.TektonPipelineInformer
	// manifest has the source manifest of Tekton Triggers for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// metrics handles metrics for trigger install
	metrics *Recorder
	// version of triggers which we are installing
	triggersVersion string
}

// Check that our Reconciler implements controller.Reconciler
var _ tektontriggerreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tt *v1alpha1.TektonTrigger) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With("name", tt.GetName())
	tt.Status.InitializeConditions()
	tt.Status.SetVersion(r.triggersVersion)

	if tt.GetName() != v1alpha1.TriggerResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.TriggerResourceName,
			tt.GetName(),
		)
		logger.Error(msg)
		tt.Status.MarkNotReady(msg)
		return nil
	}

	//Make sure TektonPipeline is installed before proceeding with
	//TektonTrigger
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady {
			tt.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		tt.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}
	tt.Status.MarkDependenciesInstalled()

	// Pass the object through defaulting
	tt.SetDefaults(ctx)

	if err := r.extension.PreReconcile(ctx, tt); err != nil {
		tt.Status.MarkPreReconcilerFailed(fmt.Sprintf("PreReconciliation failed: %s", err.Error()))
		return err
	}

	//Mark PreReconcile Complete
	tt.Status.MarkPreReconcilerComplete()

	sets, err := r.installerSetClient.CheckMainSet(ctx, tt)
	if err == nil {
		logger.Infof("found %v installer sets", len(sets))
	}

	switch err {
	case client.ErrNotFound:
		logger.Info("installer set not found, creating")
		sets, err = r.installerSetClient.CreateMainSet(ctx, tt, &r.manifest)
		if err != nil {
			return nil
		}
		if tt.Status.IsNewInstallation() {
			r.metrics.logMetrics(metricsNew, r.triggersVersion, logger)
		}

	case client.ErrInvalidState, client.ErrNsDifferent, client.ErrVersionDifferent:
		logger.Infof("installer set not in valid state : %v, cleaning up!", err)
		if err := r.installerSetClient.CleanupMainSet(ctx); err != nil {
			logger.Errorf("failed to cleanup main installer set: %v", err)
			return nil
		}
		if err == client.ErrVersionDifferent {
			r.metrics.logMetrics(metricsUpgrade, r.triggersVersion, logger)
			markUpgrade(tt)
		} else {
			markReinstalling(tt)
		}
		logger.Infof("returning, will create main installer sets in further reconcile")
		return v1alpha1.REQUEUE_EVENT_AFTER

	case client.ErrUpdateRequired:
		logger.Info("updating installer set")
		sets, err = r.installerSetClient.UpdateMainSet(ctx, tt, sets, &r.manifest)
		if err != nil {
			return nil
		}
	case client.ErrSetsInDeletionState:
		logger.Info(err)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	//Mark InstallerSet Available
	tt.Status.MarkInstallerSetAvailable()

	for _, set := range sets {
		if !set.Status.IsReady() {
			logger.Infof("installer set %v no yet ready, wait !", set.GetName())
			return nil
		}
	}

	//Mark InstallerSet Ready
	tt.Status.MarkInstallerSetReady()

	if err := r.extension.PostReconcile(ctx, tt); err != nil {
		tt.Status.MarkPostReconcilerFailed(fmt.Sprintf("PostReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PostReconcile Complete
	tt.Status.MarkPostReconcilerComplete()
	return nil
}

func (m *Recorder) logMetrics(status, version string, logger *zap.SugaredLogger) {
	err := m.Count(status, version)
	if err != nil {
		logger.Warnf("Failed to log the metrics : %v", err)
	}
}

func markUpgrade(tt *v1alpha1.TektonTrigger) {
	tt.Status.MarkInstallerSetNotReady(v1alpha1.UpgradePending)
	tt.Status.MarkPreReconcilerFailed(v1alpha1.UpgradePending)
	tt.Status.MarkPostReconcilerFailed(v1alpha1.UpgradePending)
	tt.Status.MarkNotReady(v1alpha1.UpgradePending)
}

func markReinstalling(tt *v1alpha1.TektonTrigger) {
	tt.Status.MarkInstallerSetNotReady(v1alpha1.Reinstalling)
	tt.Status.MarkPreReconcilerFailed(v1alpha1.Reinstalling)
	tt.Status.MarkPostReconcilerFailed(v1alpha1.Reinstalling)
	tt.Status.MarkNotReady(v1alpha1.Reinstalling)
}
