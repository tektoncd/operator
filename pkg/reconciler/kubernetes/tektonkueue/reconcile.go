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

package tektonkueue

import (
	"context"
	"errors"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektonkueuereconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonkueue"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonKueue resources.
type Reconciler struct {
	// operator client to interact with operator resources
	operatorClientSet operatorclient.Interface
	// kube client to interact with core k8s resources
	kubeClientSet kubernetes.Interface
	// installer Set client to do CRUD operations for components
	installerSetClient *client.InstallerSetClient
	// pipelineInformer to query for TektonPipeline
	pipelineInformer pipelineinformer.TektonPipelineInformer
	// manifest has the source manifest of Tekton Kueues for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// version of kueue which we are installing
	tektonKueueVersion string
	operatorVersion    string

	// Namespace for upstream Kueue installation. Default it kueue-system
	kueueNameSpace string
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonkueuereconciler.Interface = (*Reconciler)(nil)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tektonKueue *v1alpha1.TektonKueue) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With("name", tektonKueue.GetName())
	tektonKueue.Status.InitializeConditions()
	tektonKueue.Status.SetVersion(r.tektonKueueVersion)

	if tektonKueue.GetName() != v1alpha1.TektonKueueResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.TektonKueueResourceName,
			tektonKueue.GetName(),
		)
		logger.Error(msg)
		tektonKueue.Status.MarkNotReady(msg)
		return nil
	}

	// reconcile target namespace
	if err := common.ReconcileTargetNamespace(ctx, nil, nil, tektonKueue, r.kubeClientSet); err != nil {
		return err
	}
	// Make sure TektonPipeline is installed before proceeding with
	err := r.ensureDependenciesInstalled(tektonKueue)
	if err != nil {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	tektonKueue.Status.MarkDependenciesInstalled()

	if err := r.installerSetClient.RemoveObsoleteSets(ctx); err != nil {
		logger.Error("failed to remove obsolete installer sets: %v", err)
		return err
	}

	if err := r.extension.PreReconcile(ctx, tektonKueue); err != nil {
		msg := fmt.Sprintf("PreReconciliation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		tektonKueue.Status.MarkPreReconcilerFailed(msg)
		return nil
	}

	// Mark PreReconcile Complete
	tektonKueue.Status.MarkPreReconcilerComplete()

	//  Create/Update Required TektonInstallerSets
	if err := r.ensureInstallerSets(ctx, tektonKueue); err != nil {
		return err
	}

	if err := r.extension.PostReconcile(ctx, tektonKueue); err != nil {
		msg := fmt.Sprintf("PostReconciliation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		tektonKueue.Status.MarkPostReconcilerFailed(msg)
		return nil
	}

	// Mark PostReconcile Complete
	tektonKueue.Status.MarkPostReconcilerComplete()
	return nil
}

func (r *Reconciler) ensureDependenciesInstalled(tektonKueue *v1alpha1.TektonKueue) error {
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady || errors.Is(err, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR) {
			tektonKueue.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		tektonKueue.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}

	discoveryClient := r.kubeClientSet.Discovery()
	_, err := discoveryClient.ServerResourcesForGroupVersion("kueue.x-k8s.io/v1beta1")
	if err != nil {
		tektonKueue.Status.MarkDependencyMissing("API kueue.x-k8s.io/v1beta1 does not exist. Please install kueue.x-k8s.io/v1beta1")
		return err
	}

	return nil

}
