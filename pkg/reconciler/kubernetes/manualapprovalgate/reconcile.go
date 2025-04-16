/*
Copyright 2024 The Tekton Authors

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

package manualapprovalgate

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	manualapprovalgatereconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/manualapprovalgate"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

type Reconciler struct {
	// kube client to interact with core k8s resources
	kubeClientSet kubernetes.Interface
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// installer Set client to do CRUD operations for components
	installerSetClient *client.InstallerSetClient
	// manifest has the source manifest of ManualApprovalGate for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// manualApprovalGateVersion describes the current manualapprovalgate version
	manualApprovalGateVersion string
	operatorVersion           string
	// pipelineInformer provides access to a shared informer and lister for
	// TektonPipelines
	pipelineInformer pipelineinformer.TektonPipelineInformer
}

// Check that our Reconciler implements controller.Reconciler
var _ manualapprovalgatereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, mag *v1alpha1.ManualApprovalGate) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With("manualapprovalgate", mag.GetName())

	logger.Debugw("Starting ManualApprovalGate reconciliation",
		"version", r.manualApprovalGateVersion,
		"status", mag.Status.GetCondition(apis.ConditionReady))

	mag.Status.InitializeConditions()
	mag.Status.SetVersion(r.manualApprovalGateVersion)

	if mag.GetName() != v1alpha1.ManualApprovalGates {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.ManualApprovalGates,
			mag.GetName(),
		)
		logger.Errorw("Invalid resource name", "expectedName", v1alpha1.ManualApprovalGates, "actualName", mag.GetName())
		mag.Status.MarkNotReady(msg)
		return nil
	}

	// reconcile target namespace
	logger.Debug("Reconciling target namespace")
	if err := common.ReconcileTargetNamespace(ctx, nil, nil, mag, r.kubeClientSet); err != nil {
		logger.Errorw("Failed to reconcile target namespace", "error", err)
		return err
	}
	logger.Info("Target namespace reconciled successfully")

	//Make sure TektonPipeline is installed before proceeding with
	//ManualApprovalGate
	logger.Debug("Checking Tekton Pipeline dependency")
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			logger.Infow("Tekton Pipeline dependency not ready yet", "error", err)
			mag.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		logger.Errorw("Tekton Pipeline dependency missing", "error", err)
		mag.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}
	logger.Info("All dependencies installed successfully")
	mag.Status.MarkDependenciesInstalled()

	logger.Debug("Removing obsolete installer sets")
	if err := r.installerSetClient.RemoveObsoleteSets(ctx); err != nil {
		logger.Errorw("Failed to remove obsolete installer sets", "error", err)
		return err
	}
	logger.Debug("Obsolete installer sets removed")

	logger.Debug("Executing pre-reconciliation")
	if err := r.extension.PreReconcile(ctx, mag); err != nil {
		msg := fmt.Sprintf("PreReconciliation failed: %s", err.Error())
		logger.Errorw("Pre-reconciliation failed", "error", err)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			logger.Info("Pre-reconciliation requested requeue")
			return err
		}
		mag.Status.MarkPreReconcilerFailed(msg)
		return nil
	}

	logger.Info("Pre-reconciliation completed successfully")
	mag.Status.MarkPreReconcilerComplete()

	if err := r.installerSetClient.MainSet(ctx, mag, &r.manifest, filterAndTransform(r.extension)); err != nil {
		msg := fmt.Sprintf("Main Reconcilation failed: %s", err.Error())
		logger.Errorw("Failed to apply main installer set", "error", err)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			logger.Info("Main reconciliation requested requeue")
			return err
		}
		mag.Status.MarkInstallerSetNotReady(msg)
		return nil
	}
	logger.Info("Main manifest applied successfully")

	logger.Debug("Executing post-reconciliation")
	if err := r.extension.PostReconcile(ctx, mag); err != nil {
		msg := fmt.Sprintf("PostReconciliation failed: %s", err.Error())
		logger.Errorw("Post-reconciliation failed", "error", err)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			logger.Info("Post-reconciliation requested requeue")
			return err
		}
		mag.Status.MarkPostReconcilerFailed(msg)
		return nil
	}

	// Mark PostReconcile Complete
	logger.Info("Post-reconciliation completed successfully")
	mag.Status.MarkPostReconcilerComplete()

	logger.Info("ManualApprovalGate reconciliation completed successfully")
	return nil
}
