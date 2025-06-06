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

package tektondashboard

import (
	"context"
	"fmt"

	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektondashboardreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektondashboard"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonDashboard resources.
type Reconciler struct {
	// installer Set client to do CRUD operations for components
	installerSetClient *client.InstallerSetClient
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// readOnlyManifest has the source manifest of Tekton Dashboard for
	// a particular version with readonly value as true
	readonlyManifest mf.Manifest
	// fullaccessManifest has the source manifest of Tekton Dashboard for
	// a particular version with readonly value as false
	fullaccessManifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension        common.Extension
	pipelineInformer pipelineinformer.TektonPipelineInformer
	operatorVersion  string
	dashboardVersion string
}

// Check that our Reconciler implements controller.Reconciler
var _ tektondashboardreconciler.Interface = (*Reconciler)(nil)
var _ tektondashboardreconciler.Finalizer = (*Reconciler)(nil)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, td *v1alpha1.TektonDashboard) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With("tektondashboard", td.GetName())
	td.Status.InitializeConditions()
	td.Status.ObservedGeneration = td.Generation
	td.Status.SetVersion(r.dashboardVersion)

	logger.Debugw("Starting TektonDashboard reconciliation",
		"version", r.dashboardVersion,
		"generation", td.Generation,
		"status", td.Status.GetCondition(apis.ConditionReady))

	if td.GetName() != v1alpha1.DashboardResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.DashboardResourceName,
			td.GetName(),
		)
		logger.Errorw("Invalid resource name", "expectedName", v1alpha1.DashboardResourceName, "actualName", td.GetName())
		td.Status.MarkNotReady(msg)
		return nil
	}

	// find the valid tekton-pipeline installation
	logger.Debug("Checking Tekton Pipeline dependency")
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			logger.Infow("Tekton Pipeline dependency not ready yet", "error", err)
			td.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return v1alpha1.REQUEUE_EVENT_AFTER

		}
		// (tektonpipeline.opeator.tekton.dev instance not available yet)
		logger.Errorw("Tekton Pipeline dependency missing", "error", err)
		td.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}
	logger.Info("All dependencies installed successfully")
	td.Status.MarkDependenciesInstalled()

	logger.Debug("Removing obsolete installer sets")
	if err := r.installerSetClient.RemoveObsoleteSets(ctx); err != nil {
		logger.Errorw("Failed to remove obsolete installer sets", "error", err)
		return err
	}
	logger.Debug("Obsolete installer sets removed")

	logger.Debug("Executing pre-reconciliation")
	if err := r.extension.PreReconcile(ctx, td); err != nil {
		logger.Errorw("Pre-reconciliation failed", "error", err)
		td.Status.MarkPreReconcilerFailed(fmt.Sprintf("PreReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PreReconcile Complete
	logger.Info("Pre-reconciliation completed successfully")
	td.Status.MarkPreReconcilerComplete()

	var manifest mf.Manifest
	if td.Spec.Readonly {
		logger.Debugw("Using readonly manifest", "readonly", true)
		manifest = r.readonlyManifest
	} else {
		logger.Debugw("Using full access manifest", "readonly", false)
		manifest = r.fullaccessManifest
	}

	// When Tekton Dashboard is insalled targetNamespace is getting updated with the OwnerRef as TektonDashboard
	// and hence deleting the component in the integration tests, targetNamespace was getting deleted. Hence
	// filtering out the namespace here
	logger.Debug("Filtering out namespace from manifest")
	manifest = manifest.Filter(mf.Not(mf.ByKind("Namespace")))

	logger.Debug("Applying main manifest")
	if err := r.installerSetClient.MainSet(ctx, td, &manifest, filterAndTransform(r.extension)); err != nil {
		msg := fmt.Sprintf("Main Reconcilation failed: %s", err.Error())
		logger.Errorw("Failed to apply main installer set", "error", err)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			logger.Infow("Main reconciliation requested requeue")
			return err
		}
		td.Status.MarkInstallerSetNotReady(msg)
		return nil
	}
	logger.Info("Main manifest applied successfully")

	logger.Debug("Executing post-reconciliation")
	if err := r.extension.PostReconcile(ctx, td); err != nil {
		msg := fmt.Sprintf("PostReconciliation failed: %s", err.Error())
		logger.Errorw("Post-reconciliation failed", "error", err)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			logger.Infow("Post-reconciliation requested requeue")
			return err
		}
		logger.Infow("Post-reconciliation completed successfully")
		td.Status.MarkPostReconcilerFailed(msg)
		return nil
	}

	logger.Info("Post-reconciliation completed successfully")
	td.Status.MarkPostReconcilerComplete()

	logger.Info("TektonDashboard reconciliation completed successfully")
	return nil
}
