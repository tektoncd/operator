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

package tektonscheduler

import (
	"context"
	"errors"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	TektonSchedulerreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonscheduler"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonScheduler resources.
type Reconciler struct {
	// operator client to interact with operator resources
	operatorClientSet operatorclient.Interface
	// kube client to interact with core k8s resources
	kubeClientSet kubernetes.Interface
	// installer Set client to do CRUD operations for components
	installerSetClient *client.InstallerSetClient
	// pipelineInformer to query for TektonPipeline
	pipelineInformer pipelineinformer.TektonPipelineInformer
	// manifest has the source manifest of Tekton schedulers for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// version of scheduler which we are installing
	tektonSchedulerVersion string
	operatorVersion        string
}

// Check that our Reconciler implements controller.Reconciler
var _ TektonSchedulerreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, TektonScheduler *v1alpha1.TektonScheduler) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With("name", TektonScheduler.GetName())
	TektonScheduler.Status.InitializeConditions()
	TektonScheduler.Status.SetVersion(r.tektonSchedulerVersion)

	if TektonScheduler.GetName() != v1alpha1.TektonSchedulerResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.TektonSchedulerResourceName,
			TektonScheduler.GetName(),
		)
		logger.Error(msg)
		TektonScheduler.Status.MarkNotReady(msg)
		return nil
	}

	// reconcile target namespace
	if err := common.ReconcileTargetNamespace(ctx, nil, nil, TektonScheduler, r.kubeClientSet); err != nil {
		return err
	}
	// Make sure TektonPipeline is installed before proceeding with
	err := r.ensureDependenciesInstalled(TektonScheduler)
	if err != nil {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	TektonScheduler.Status.MarkDependenciesInstalled()

	if err := r.installerSetClient.RemoveObsoleteSets(ctx); err != nil {
		logger.Error("failed to remove obsolete installer sets: %v", err)
		return err
	}

	if err := r.extension.PreReconcile(ctx, TektonScheduler); err != nil {
		msg := fmt.Sprintf("PreReconciliation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		TektonScheduler.Status.MarkPreReconcilerFailed(msg)
		return nil
	}

	// Mark PreReconcile Complete
	TektonScheduler.Status.MarkPreReconcilerComplete()

	//  Create/Update Required TektonInstallerSets
	if err := r.ensureInstallerSets(ctx, TektonScheduler); err != nil {
		return err
	}

	if err := r.extension.PostReconcile(ctx, TektonScheduler); err != nil {
		msg := fmt.Sprintf("PostReconciliation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		TektonScheduler.Status.MarkPostReconcilerFailed(msg)
		return nil
	}

	// Mark PostReconcile Complete
	TektonScheduler.Status.MarkPostReconcilerComplete()
	return nil
}

func (r *Reconciler) ensureDependenciesInstalled(TektonScheduler *v1alpha1.TektonScheduler) error {
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady || errors.Is(err, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR) {
			TektonScheduler.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		TektonScheduler.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}

	discoveryClient := r.kubeClientSet.Discovery()
	_, err := discoveryClient.ServerResourcesForGroupVersion("kueue.x-k8s.io/v1beta1")
	if err != nil {
		TektonScheduler.Status.MarkDependencyMissing("API kueue.x-k8s.io/v1beta1 does not exist. Please install kueue.x-k8s.io/v1beta1")
		return err
	}

	return nil

}
