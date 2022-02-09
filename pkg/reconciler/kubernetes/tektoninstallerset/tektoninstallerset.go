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

package tektoninstallerset

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonInstallerreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektoninstallerset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonInstallerSet resources.
type Reconciler struct {
	operatorClientSet clientset.Interface
	mfClient          mf.Client
	mfLogger          logr.Logger
	enqueueAfter      func(obj interface{}, after time.Duration)
}

// Reconciler implements controller.Reconciler
var _ tektonInstallerreconciler.Interface = (*Reconciler)(nil)
var _ tektonInstallerreconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a TektonInstallerSet.
func (r *Reconciler) FinalizeKind(ctx context.Context, installerSet *v1alpha1.TektonInstallerSet) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	deleteManifests, err := mf.ManifestFrom(installerSet.Spec.Manifests, mf.UseClient(r.mfClient), mf.UseLogger(r.mfLogger))
	if err != nil {
		logger.Error("Error creating initial manifest: ", err)
		installerSet.Status.MarkNotReady(fmt.Sprintf("Internal Error: failed to create manifest: %s", err.Error()))
		return err
	}

	// Delete all resources except CRDs and Namespace as they are own by owner of
	// TektonInstallerSet
	// They will be deleted when the component CR is deleted
	deleteManifests = deleteManifests.Filter(mf.Not(mf.Any(namespacePred, mf.CRDs)))
	err = deleteManifests.Delete(mf.PropagationPolicy(v1.DeletePropagationForeground))
	if err != nil {
		logger.Error("failed to delete resources")
		return err
	}

	return nil
}

// Returns ownerReference to add in resource while installing
func getReference(tis *v1alpha1.TektonInstallerSet) []v1.OwnerReference {
	return []v1.OwnerReference{*v1.NewControllerRef(tis, tis.GetGroupVersionKind())}
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, installerSet *v1alpha1.TektonInstallerSet) pkgreconciler.Event {
	installerSet.Status.InitializeConditions()
	logger := logging.FromContext(ctx)

	installManifests, err := mf.ManifestFrom(installerSet.Spec.Manifests, mf.UseClient(r.mfClient), mf.UseLogger(r.mfLogger))
	if err != nil {
		logger.Error("Error creating initial manifest: ", err)
		installerSet.Status.MarkNotReady(fmt.Sprintf("Internal Error: failed to create manifest: %s", err.Error()))
		return err
	}

	// Set owner of InstallerSet as owner of CRDs so that
	// deleting the installer will not delete the CRDs and Namespace
	// If installerSet has not set any owner then CRDs will
	// not have any owner
	installerSetOwner := installerSet.GetOwnerReferences()

	installManifests, err = installManifests.Transform(
		injectOwner(getReference(installerSet)),
		injectOwnerForCRDsAndNamespace(installerSetOwner),
	)
	if err != nil {
		logger.Error("failed to transform manifest")
		return err
	}

	installer := installer{
		Manifest: installManifests,
	}

	// Install CRDs
	err = installer.EnsureCRDs()
	if err != nil {
		installerSet.Status.MarkCRDsInstallationFailed(err.Error())
		return r.handleError(err, installerSet)
	}

	// Update Status for CRD condition
	installerSet.Status.MarkCRDsInstalled()

	// Install ClusterScoped Resources
	err = installer.EnsureClusterScopedResources()
	if err != nil {
		installerSet.Status.MarkClustersScopedInstallationFailed(err.Error())
		return r.handleError(err, installerSet)
	}

	// Update Status for ClustersScope Condition
	installerSet.Status.MarkClustersScopedResourcesInstalled()

	// Install NamespaceScoped Resources
	err = installer.EnsureNamespaceScopedResources()
	if err != nil {
		installerSet.Status.MarkNamespaceScopedInstallationFailed(err.Error())
		return r.handleError(err, installerSet)
	}

	// Update Status for NamespaceScope Condition
	installerSet.Status.MarkNamespaceScopedResourcesInstalled()

	// Install Deployment Resources
	err = installer.EnsureDeploymentResources()
	if err != nil {
		installerSet.Status.MarkDeploymentsAvailableFailed(err.Error())
		return r.handleError(err, installerSet)
	}

	// Update Status for Deployment Resources
	installerSet.Status.MarkDeploymentsAvailable()

	// Check if webhook is ready
	err = installer.IsWebhookReady()
	if err != nil {
		installerSet.Status.MarkWebhookNotReady(err.Error())
		r.enqueueAfter(installerSet, time.Second*10)
		return nil
	}

	// Update Status for Webhook
	installerSet.Status.MarkWebhookReady()

	// Check if controller is ready
	err = installer.IsControllerReady()
	if err != nil {
		installerSet.Status.MarkControllerNotReady(err.Error())
		r.enqueueAfter(installerSet, time.Second*10)
		return nil
	}

	// Update Ready status of Controller
	installerSet.Status.MarkControllerReady()

	// Check if any other deployment exists other than controller
	// and webhook and is ready
	err = installer.AllDeploymentsReady()
	if err != nil {
		installerSet.Status.MarkAllDeploymentsNotReady(err.Error())
		r.enqueueAfter(installerSet, time.Second*10)
		return nil
	}

	// Mark all deployments ready
	installerSet.Status.MarkAllDeploymentsReady()

	return nil
}

func (r *Reconciler) handleError(err error, installerSet *v1alpha1.TektonInstallerSet) error {
	if err == v1alpha1.RECONCILE_AGAIN_ERR {
		r.enqueueAfter(installerSet, 10*time.Second)
		return nil
	}
	return err
}
