/*
Copyright 2022 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    hpacp://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openshiftpipelinesascode

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	pacreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/openshiftpipelinesascode"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for OpenShiftPipelinesAsCode resources.
type Reconciler struct {
	// installer Set client to do CRUD operations for components
	installerSetClient *client.InstallerSetClient
	// pipelineInformer to query for TektonPipeline
	pipelineInformer pipelineinformer.TektonPipelineInformer
	// manifest has the source manifest of Openshift Pipelines As Code for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// version of PipelinesAsCode which we are installing
	pacVersion string
}

// Check that our Reconciler implements controller.Reconciler
var _ pacreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind compares the actual state with the desired, and apacempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, pac *v1alpha1.OpenShiftPipelinesAsCode) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With("name", pac.GetName())
	pac.Status.InitializeConditions()
	pac.Status.SetVersion(r.pacVersion)

	if pac.GetName() != v1alpha1.OpenShiftPipelinesAsCodeName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.OpenShiftPipelinesAsCodeName,
			pac.GetName(),
		)
		logger.Error(msg)
		pac.Status.MarkNotReady(msg)
		return nil
	}

	//Make sure TektonPipeline is installed before proceeding with OpenShiftPipelinesAsCode
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			pac.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		pac.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}
	pac.Status.MarkDependenciesInstalled()

	if err := r.extension.PreReconcile(ctx, pac); err != nil {
		return err
	}

	//Mark PreReconcile Complete
	pac.Status.MarkPreReconcilerComplete()

	if err := r.installerSetClient.MainSet(ctx, pac, &r.manifest, filterAndTransform(r.extension)); err != nil {
		msg := fmt.Sprintf("Main Reconcilation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		pac.Status.MarkInstallerSetNotReady(msg)
		return nil
	}

	if err := r.extension.PostReconcile(ctx, pac); err != nil {
		msg := fmt.Sprintf("PostReconciliation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		pac.Status.MarkPostReconcilerFailed(msg)
		return nil
	}

	// Mark PostReconcile Complete
	pac.Status.MarkPostReconcilerComplete()
	return nil
}
