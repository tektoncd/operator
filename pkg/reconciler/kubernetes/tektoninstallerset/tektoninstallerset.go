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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonInstallerreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektoninstallerset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonInstallerSet resources.
type Reconciler struct {
	operatorClientSet clientset.Interface
	mfClient          mf.Client
	kubeClientSet     kubernetes.Interface
}

// Reconciler implements controller.Reconciler
var _ tektonInstallerreconciler.Interface = (*Reconciler)(nil)
var _ tektonInstallerreconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a TektonInstallerSet.
func (r *Reconciler) FinalizeKind(ctx context.Context, installerSet *v1alpha1.TektonInstallerSet) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	deleteManifests, err := mf.ManifestFrom(installerSet.Spec.Manifests, mf.UseClient(r.mfClient))
	if err != nil {
		logger.Error("Error creating initial manifest: ", err)
		installerSet.Status.MarkNotReady(fmt.Sprintf("Internal Error: failed to create manifest: %s", err.Error()))
		return err
	}

	installer := NewInstaller(&deleteManifests, r.mfClient, r.kubeClientSet, logger)
	err = installer.DeleteResources()
	if err != nil {
		logger.Error("failed to delete resources: ", err)
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
	logger := logging.FromContext(ctx).With("installerSet", fmt.Sprintf("%s/%s", installerSet.Namespace, installerSet.Name))

	logger.Debugw("Starting TektonInstallerSet reconciliation",
		"resourceVersion", installerSet.ResourceVersion,
		"status", installerSet.Status.GetCondition(apis.ConditionReady))

	installManifests, err := mf.ManifestFrom(installerSet.Spec.Manifests, mf.UseClient(r.mfClient))
	if err != nil {
		msg := fmt.Sprintf("Internal Error: failed to create manifest: %s", err.Error())
		logger.Errorw("Failed to create initial manifest", "error", err)
		installerSet.Status.MarkNotReady(msg)
		return err
	}
	logger.Debug("Successfully created initial manifest")

	// Set owner of InstallerSet as owner of CRDs so that
	// deleting the installer will not delete the CRDs and Namespace
	// If installerSet has not set any owner then CRDs will
	// not have any owner
	installerSetOwner := installerSet.GetOwnerReferences()
	logger.Debug("Transforming manifest with ownership information")
	installManifests, err = installManifests.Transform(
		injectOwner(getReference(installerSet)),
		injectOwnerForCRDsAndNamespace(installerSetOwner),
	)
	if err != nil {
		logger.Errorw("Failed to transform manifest with ownership information", "error", err)
		return err
	}

	installer := NewInstaller(&installManifests, r.mfClient, r.kubeClientSet, logger)

	// Install CRDs
	logger.Debug("Installing CRDs")
	err = installer.EnsureCRDs()
	if err != nil {
		logger.Errorw("CRD installation failed", "error", err)
		installerSet.Status.MarkCRDsInstallationFailed(err.Error())
		return r.handleError(err, installerSet)
	}

	// Update Status for CRD condition
	installerSet.Status.MarkCRDsInstalled()
	logger.Debug("CRDs installed successfully")

	// Install ClusterScoped Resources
	logger.Debug("Installing cluster-scoped resources")
	err = installer.EnsureClusterScopedResources()
	if err != nil {
		logger.Errorw("Cluster-scoped resources installation failed", "error", err)
		installerSet.Status.MarkClustersScopedInstallationFailed(err.Error())
		return r.handleError(err, installerSet)
	}

	// Update Status for ClustersScope Condition
	installerSet.Status.MarkClustersScopedResourcesInstalled()
	logger.Debug("Cluster-scoped resources installed successfully")

	// Install NamespaceScoped Resources
	logger.Debug("Installing namespace-scoped resources")
	err = installer.EnsureNamespaceScopedResources()
	if err != nil {
		logger.Errorw("Namespace-scoped resources installation failed", "error", err)
		installerSet.Status.MarkNamespaceScopedInstallationFailed(err.Error())
		return r.handleError(err, installerSet)
	}

	// Update Status for NamespaceScope Condition
	installerSet.Status.MarkNamespaceScopedResourcesInstalled()
	logger.Debug("Namespace-scoped resources installed successfully")

	// Install Job Resources
	logger.Debug("Installing job resources")
	err = installer.EnsureJobResources()
	if err != nil {
		logger.Errorw("Job resources installation failed", "error", err)
		installerSet.Status.MarkJobsInstallationFailed(err.Error())
		return r.handleError(err, installerSet)
	}

	// Update Status for Job Resources
	installerSet.Status.MarkJobsInstalled()
	logger.Debug("Job resources installed successfully")

	// Install Deployment Resources
	logger.Debug("Installing deployment resources")
	err = installer.EnsureDeploymentResources(ctx)
	if err != nil {
		logger.Errorw("Deployment resources installation failed", "error", err)
		installerSet.Status.MarkDeploymentsAvailableFailed(err.Error())
		return r.handleError(err, installerSet)
	}

	// Update Status for Deployment Resources
	installerSet.Status.MarkDeploymentsAvailable()
	logger.Debug("Deployment resources installed successfully")

	// Install StatefulSet Resources
	logger.Debug("Installing statefulset resources")
	err = installer.EnsureStatefulSetResources(ctx)
	if err != nil {
		logger.Errorw("StatefulSet resources installation failed", "error", err)
		installerSet.Status.MarkStatefulSetNotReady(err.Error())
		return r.handleError(err, installerSet)
	}

	// Update Status for StatefulSet Resources
	installerSet.Status.MarkStatefulSetReady()
	logger.Debug("StatefulSet resources installed successfully")

	// Check if webhook is ready
	logger.Debugw("Checking webhook readiness")
	err = installer.IsWebhookReady()
	if err != nil {
		logger.Warnw("Webhook not ready", "error", err)
		installerSet.Status.MarkWebhookNotReady(err.Error())
		return nil
	}

	// Update Status for Webhook
	installerSet.Status.MarkWebhookReady()
	logger.Debug("Webhook is ready")

	// Check if controller is ready
	logger.Debug("Checking controller readiness")
	err = installer.IsControllerReady()
	if err != nil {
		logger.Warnw("Controller not ready", "error", err)
		installerSet.Status.MarkControllerNotReady(err.Error())
		return nil
	}

	// Update Ready status of Controller
	installerSet.Status.MarkControllerReady()
	logger.Debug("Controller is ready")

	// job
	labels := installerSet.GetLabels()
	installSetname := installerSet.GetName()
	logger.Debug("Checking job completion status")
	err = installer.IsJobCompleted(ctx, labels, installSetname)
	if err != nil {
		logger.Warnw("Jobs not completed", "error", err)
		return err
	}
	logger.Debug("All jobs completed successfully")

	// Check if any other deployment exists other than controller
	// and webhook and is ready
	logger.Debug("Checking all deployments readiness")
	err = installer.AllDeploymentsReady()
	if err != nil {
		logger.Warnw("Not all deployments are ready", "error", err)
		installerSet.Status.MarkAllDeploymentsNotReady(err.Error())
		return nil
	}

	// Mark all deployments ready
	installerSet.Status.MarkAllDeploymentsReady()
	logger.Debug("All deployments are ready")

	logger.Debugw("TektonInstallerSet reconciliation completed successfully",
		"ready", installerSet.Status.GetCondition(apis.ConditionReady))

	return nil
}

func (r *Reconciler) handleError(err error, installerSet *v1alpha1.TektonInstallerSet) error {
	if err == v1alpha1.RECONCILE_AGAIN_ERR {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	return err
}
