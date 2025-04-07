/*
Copyright 2019 The Tekton Authors

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

package tektonpipeline

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	tektonpipelinereconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonpipeline"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	resourceKind = v1alpha1.KindTektonPipeline

	proxyLabel = "operator.tekton.dev/disable-proxy=true"
)

// Reconciler implements controller.Reconciler for TektonPipeline resources.
type Reconciler struct {
	// installer Set client to do CRUD operations for components
	installerSetClient *client.InstallerSetClient
	// manifest has the source manifest of Tekton Pipeline for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// kube client to interact with core k8s resources
	kubeClientSet kubernetes.Interface
	// version of pipelines which we are installing
	pipelineVersion string
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonpipelinereconciler.Interface = (*Reconciler)(nil)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tp *v1alpha1.TektonPipeline) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With(
		"name", tp.GetName(),
		"namespace", tp.GetNamespace(),
		"resourceVersion", tp.GetResourceVersion(),
	)
	logger.Debugw("Starting TektonPipeline reconciliation",
		"version", r.pipelineVersion,
		"status", tp.Status.GetCondition(apis.ConditionReady))

	tp.Status.InitializeConditions()
	tp.Status.SetVersion(r.pipelineVersion)

	if tp.GetName() != v1alpha1.PipelineResourceName {
		msg := fmt.Sprintf("Resource ignored: expected name '%s', got '%s'",
			v1alpha1.PipelineResourceName, tp.GetName())
		logger.Errorw("Invalid resource name", "expectedName", v1alpha1.PipelineResourceName, "actualName", tp.GetName())
		tp.Status.MarkNotReady(msg)
		return nil
	}

	// Pass the object through defaulting
	tp.SetDefaults(ctx)

	// reconcile target namespace
	logger.Debug("Reconciling target namespace")
	if err := common.ReconcileTargetNamespace(ctx, nil, nil, tp, r.kubeClientSet); err != nil {
		logger.Errorw("Failed to reconcile target namespace", "error", err)
		return err
	}

	if err := r.installerSetClient.RemoveObsoleteSets(ctx); err != nil {
		logger.Errorw("Failed to remove obsolete installer sets", "error", err)
		return err
	}
	logger.Debug("Obsolete installer sets removed")

	// Pipeline controller is deployed as statefulset, ensure deployment installerset is deleted
	if tp.Spec.Performance.StatefulsetOrdinals != nil && *tp.Spec.Performance.StatefulsetOrdinals {
		logger.Debugw("Cleaning up deployment installer set", "usingStatefulset", true)
		if err := r.installerSetClient.CleanupSubTypeDeployment(ctx); err != nil {
			logger.Errorw("Failed to delete main deployment installer set", "error", err)
			return err
		}
		logger.Debug("Deployment installer set deleted")
	} else {
		// Pipeline controller is deployed as deployment, ensure statefulset installerset is deleted
		if err := r.installerSetClient.CleanupSubTypeStatefulset(ctx); err != nil {
			logger.Debugw("Cleaning up statefulset installer set", "usingDeployment", true)
			return err
		}
		logger.Debug("Statefulset installer set deleted")
	}

	logger.Debug("Executing pre-reconciliation")
	if err := r.extension.PreReconcile(ctx, tp); err != nil {
		msg := fmt.Sprintf("PreReconciliation failed: %s", err.Error())
		logger.Errorw("Pre-reconciliation failed", "error", err)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			logger.Infow("Pre-reconciliation requested requeue")
			return err
		}
		tp.Status.MarkPreReconcilerFailed(msg)
		return nil
	}

	// Mark PreReconcile Complete
	logger.Info("Pre-reconciliation completed successfully")
	tp.Status.MarkPreReconcilerComplete()

	// When TektonPipeline component is deleted targetNamespace was getting deleted,
	// because in pipeline reconciler targetNamespace was updated by adding few labels which
	// in turn also updated the ownerRef of targetNamespace from TektonConfig to TektonPipeline.
	// Since namespace is created in TektonConfig reconciler hence deleting TektonPipeline
	// component should not delete the targetNamespace hence filtering out the namespace here
	logger.Debug("Filtering out namespace from manifest")
	manifest := r.manifest.Filter(mf.Not(mf.ByKind("Namespace")))

	// Ensure webhook deadlock prevention before applying the manifest
	logger.Debug("Preempting webhook deadlock")
	if err := common.PreemptDeadlock(ctx, &manifest, r.kubeClientSet, v1alpha1.PipelineResourceName); err != nil {
		logger.Errorw("Failed to preempt webhook deadlock", "error", err)
		return err
	}

	//Apply manifest
	logger.Debug("Applying main manifest")
	if err := r.installerSetClient.MainSet(ctx, tp, &manifest, filterAndTransform(r.extension)); err != nil {
		msg := fmt.Sprintf("Main Reconcilation failed: %s", err.Error())
		logger.Errorw("Failed to apply main installer set", "error", err)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			logger.Infow("Main reconciliation requested requeue")
			return err
		}
		tp.Status.MarkInstallerSetNotReady(msg)
		return nil
	}
	logger.Info("Main manifest applied successfully")

	logger.Debug("Executing post-reconciliation")
	if err := r.extension.PostReconcile(ctx, tp); err != nil {
		msg := fmt.Sprintf("PostReconciliation failed: %s", err.Error())
		logger.Errorw("Post-reconciliation failed", "error", err)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			logger.Info("Post-reconciliation requested requeue")
			return err
		}
		tp.Status.MarkPostReconcilerFailed(msg)
		return nil
	}

	// Mark PostReconcile Complete
	logger.Info("Post-reconciliation completed successfully")
	tp.Status.MarkPostReconcilerComplete()

	logger.Info("TektonPipeline reconciliation completed successfully")
	return nil
}
