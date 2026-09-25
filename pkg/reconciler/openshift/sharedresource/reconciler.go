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

package sharedresource

import (
	"context"
	"errors"
	"fmt"

	"github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/sharedresource"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	tektoninstallersetclient "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for SharedResource resources. It
// installs the Shared Resource component through TektonInstallerSets owned by the
// SharedResource instance.
type Reconciler struct {
	extension              common.Extension
	installerSetClient     *tektoninstallersetclient.InstallerSetClient
	operatorClientset      clientset.Interface
	operatorVersion        string
	platformParams         networkpolicy.PlatformParams
	releaseManifest        *manifestival.Manifest
	componentVersion       string
	tektonPipelineInformer operatorinformer.TektonPipelineInformer
}

// Check that our Reconciler implements controller.Reconciler
var _ sharedresource.Interface = (*Reconciler)(nil)
var _ sharedresource.Finalizer = (*Reconciler)(nil)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, sr *v1alpha1.SharedResource) reconciler.Event {
	logger := logging.FromContext(ctx).With(sr.Kind, sr.Name)

	sr.Status.InitializeConditions()
	sr.Status.SetVersion(r.componentVersion)
	sr.Status.ObservedGeneration = sr.Generation

	logger.Infow("Starting Shared Resource reconciliation",
		"version", r.componentVersion,
		"generation", sr.Generation,
		"status", sr.Status.GetCondition(apis.ConditionReady))

	if sr.GetName() != v1alpha1.SharedResourceResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.SharedResourceResourceName, sr.GetName())
		logger.Error(msg)
		sr.Status.MarkNotReady(msg)
		return nil
	}

	// Make sure TektonPipeline is installed before proceeding with installation
	if _, err := common.PipelineReady(r.tektonPipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady || errors.Is(err, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR) {
			sr.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		sr.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}

	sr.Status.MarkDependenciesInstalled()

	// Platform specific pre reconcile tasks
	if err := r.extension.PreReconcile(ctx, sr); err != nil {
		if errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
			return err
		}
		message := fmt.Sprintf("PreReconciliation failed: %s", err.Error())
		sr.Status.MarkPreReconcilerFailed(message)
		return err
	}
	sr.Status.MarkPreReconcilerComplete()

	// Create Main InstallerSet
	if err := r.installerSetClient.MainSet(ctx, sr, r.releaseManifest, FilterAndTransform(r)); err != nil {
		msg := fmt.Sprintf("Failed to install main set: %s", err.Error())
		logger.Error(msg)
		if errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
			return err
		}
		sr.Status.MarkInstallerSetNotReady(msg)
		return nil
	}

	// Platform specific post reconcile tasks
	if err := r.extension.PostReconcile(ctx, sr); err != nil {
		if errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
			return err
		}
		msg := fmt.Sprintf("Post reconciliation failed: %s", err.Error())
		sr.Status.MarkPostReconcilerFailed(msg)
		return err
	}

	sr.Status.MarkPostReconcilerComplete()

	logger.Infow("Reconciliation completed successfully",
		"ready", sr.Status.IsReady())
	return nil
}

// FinalizeKind removes all resources after deletion.
func (r *Reconciler) FinalizeKind(ctx context.Context, sr *v1alpha1.SharedResource) reconciler.Event {
	logger := logging.FromContext(ctx).With(sr.Kind, sr.Name)

	// Delete CRDs before deleting rest of resources so that any instance
	// of CRDs which has finalizer set will get deleted before we remove
	// the controller's deployment for it
	if err := r.releaseManifest.Filter(manifestival.CRDs).Delete(); err != nil {
		logger.Error("Failed to delete CRDs: ", err)
		return err
	}

	if err := r.installerSetClient.CleanupMainSet(ctx); err != nil {
		logger.Error("Failed to cleanup main InstallerSet: ", err)
		return err
	}

	if err := r.extension.Finalize(ctx, sr); err != nil {
		logger.Error("Failed to finalize platform resources: ", err)
	}

	return nil
}
