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
	"errors"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorv1alpha1 "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	syncerservicereconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/syncerservice"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for SyncerService resources.
type Reconciler struct {
	kubeClientSet      kubernetes.Interface
	operatorClientSet  clientset.Interface
	installerSetClient *client.InstallerSetClient
	manifest           mf.Manifest
	extension          common.Extension
	pipelineInformer   operatorv1alpha1.TektonPipelineInformer
	operatorVersion    string
	syncerVersion      string
}

// Check that our Reconciler implements controller.Reconciler
var _ syncerservicereconciler.Interface = (*Reconciler)(nil)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, ss *v1alpha1.SyncerService) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With("syncerservice", ss.Name)

	ss.Status.InitializeConditions()
	ss.Status.ObservedGeneration = ss.Generation

	logger.Infow("Starting SyncerService reconciliation",
		"version", r.syncerVersion,
		"generation", ss.Generation)

	if ss.GetName() != v1alpha1.SyncerServiceResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.SyncerServiceResourceName, ss.GetName())
		ss.Status.MarkNotReady(msg)
		return nil
	}

	// Check for TektonPipeline dependency
	tp, err := common.PipelineReady(r.pipelineInformer)
	if err != nil {
		if err.Error() == common.PipelineNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			ss.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			return fmt.Errorf(common.PipelineNotReady)
		}
		ss.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}

	if tp.GetSpec().GetTargetNamespace() != ss.GetSpec().GetTargetNamespace() {
		errMsg := fmt.Sprintf("tekton-pipelines is missing in %s namespace", ss.GetSpec().GetTargetNamespace())
		ss.Status.MarkDependencyMissing(errMsg)
		return errors.New(errMsg)
	}

	ss.Status.MarkDependenciesInstalled()

	// reconcile target namespace
	if err := common.ReconcileTargetNamespace(ctx, nil, nil, ss, r.kubeClientSet); err != nil {
		return err
	}

	if err := r.installerSetClient.RemoveObsoleteSets(ctx); err != nil {
		logger.Error("failed to remove obsolete installer sets: %v", err)
		return err
	}

	if err := r.extension.PreReconcile(ctx, ss); err != nil {
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		msg := fmt.Sprintf("PreReconciliation failed: %s", err.Error())
		ss.Status.MarkPreReconcilerFailed(msg)
		return nil
	}

	ss.Status.MarkPreReconcilerComplete()

	// Create/Update InstallerSet
	if err := r.ensureInstallerSet(ctx, ss); err != nil {
		return err
	}

	if err := r.extension.PostReconcile(ctx, ss); err != nil {
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		msg := fmt.Sprintf("PostReconciliation failed: %s", err.Error())
		ss.Status.MarkPostReconcilerFailed(msg)
		return nil
	}

	ss.Status.MarkPostReconcilerComplete()

	logger.Infow("SyncerService reconciliation completed successfully",
		"ready", ss.Status.IsReady())

	return nil
}
