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
	"time"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektondashboardreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektondashboard"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonDashboard resources.
type Reconciler struct {
	// kubeClientSet allows us to talk to the k8s for core APIs
	kubeClientSet kubernetes.Interface
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// readOnlyManifest has the source manifest of Tekton Dashboard for
	// a particular version with readonly value as true
	readonlyManifest mf.Manifest
	// fullaccessManifest has the source manifest of Tekton Dashboard for
	// a particular version with readonly value as false
	fullaccessManifest mf.Manifest
	// Platform-specific behavior to affect the transform
	// enqueueAfter enqueues a obj after a duration
	enqueueAfter func(obj interface{}, after time.Duration)
	extension    common.Extension

	releaseVersion string

	pipelineInformer pipelineinformer.TektonPipelineInformer
}

// Check that our Reconciler implements controller.Reconciler
var _ tektondashboardreconciler.Interface = (*Reconciler)(nil)
var _ tektondashboardreconciler.Finalizer = (*Reconciler)(nil)

var watchedResourceName = "dashboard"

const createdByValue = "TektonDashboard"

// FinalizeKind removes all resources after deletion of a TektonDashboards.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonDashboard) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// Delete CRDs before deleting rest of resources so that any instance
	// of CRDs which has finalizer set will get deleted before we remove
	// the controller;s deployment for it

	var manifest mf.Manifest
	if original.Spec.Readonly {
		manifest = r.readonlyManifest
	} else {
		manifest = r.fullaccessManifest
	}

	if err := manifest.Filter(mf.CRDs).Delete(); err != nil {
		logger.Error("Failed to deleted CRDs for TektonDashboard")
		return err
	}

	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", tektoninstallerset.CreatedByKey, createdByValue),
		}); err != nil {
		logger.Error("Failed to delete installer set created by TektonDashboard", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}
	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, td *v1alpha1.TektonDashboard) pkgreconciler.Event {

	logger := logging.FromContext(ctx)
	td.Status.InitializeConditions()
	td.Status.ObservedGeneration = td.Generation

	logger.Infow("Reconciling TektonDashboards", "status", td.Status)

	r.releaseVersion = common.TargetVersion(td)

	if td.GetName() != watchedResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			watchedResourceName,
			td.GetName(),
		)
		logger.Error(msg)
		td.GetStatus().MarkInstallFailed(msg)
		return nil
	}

	// find the valid tekton-pipeline installation
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady {
			td.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return fmt.Errorf(common.PipelineNotReady)
		}
		// (tektonpipeline.opeator.tekton.dev instance not available yet)
		td.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}
	td.Status.MarkDependenciesInstalled()

	if err := r.extension.PreReconcile(ctx, td); err != nil {
		td.Status.MarkPreReconcilerFailed(fmt.Sprintf("PreReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PreReconcile Complete
	td.Status.MarkPreReconcilerComplete()

	// Check if an tekton installer set already exists, if not then create
	existingInstallerSet := td.Status.GetTektonInstallerSet()
	if existingInstallerSet == "" {
		td.Status.MarkInstallerSetNotAvailable("Dashboard Installer Set Not Available")

		createdIs, err := r.createInstallerSet(ctx, td)
		if err != nil {
			return err
		}

		return r.updateTektonDashboardStatus(ctx, td, createdIs)
	}

	// If exists, then fetch the TektonInstallerSet
	installedTIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Get(ctx, existingInstallerSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			createdIs, err := r.createInstallerSet(ctx, td)
			if err != nil {
				return err
			}
			return r.updateTektonDashboardStatus(ctx, td, createdIs)
		}
		logger.Error("failed to get InstallerSet: %s", err)
		return err
	}

	installerSetTargetNamespace := installedTIS.Annotations[tektoninstallerset.TargetNamespaceKey]
	installerSetReleaseVersion := installedTIS.Annotations[tektoninstallerset.ReleaseVersionKey]

	// Check if TargetNamespace of existing TektonInstallerSet is same as expected
	// Check if Release Version in TektonInstallerSet is same as expected
	// If any of the thing above is not same then delete the existing TektonInstallerSet
	// and create a new with expected properties

	if installerSetTargetNamespace != td.Spec.TargetNamespace || installerSetReleaseVersion != r.releaseVersion {
		// Delete the existing TektonInstallerSet
		err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Delete(ctx, existingInstallerSet, metav1.DeleteOptions{})
		if err != nil {
			logger.Error("failed to delete InstallerSet: %s", err)
			return err
		}

		// Make sure the TektonInstallerSet is deleted
		_, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, existingInstallerSet, metav1.GetOptions{})
		if err == nil {
			td.Status.MarkNotReady("Waiting for previous installer set to get deleted")
			r.enqueueAfter(td, 10*time.Second)
			return nil
		}
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get InstallerSet: %s", err)
			return err
		}
		return nil

	} else {
		// If target namespace and version are not changed then check if spec
		// of TektonDashboard is changed by checking hash stored as annotation on
		// TektonInstallerSet with computing new hash of TektonDashboard Spec

		// Hash of TektonDashboard Spec

		expectedSpecHash, err := hash.Compute(td.Spec)
		if err != nil {
			return err
		}

		// spec hash stored on installerSet
		lastAppliedHash := installedTIS.GetAnnotations()[tektoninstallerset.LastAppliedHashKey]

		if lastAppliedHash != expectedSpecHash {

			var manifest mf.Manifest
			if td.Spec.Readonly {
				manifest = r.readonlyManifest
			} else {
				manifest = r.fullaccessManifest
			}

			if err := r.transform(ctx, &manifest, td); err != nil {
				logger.Error("manifest transformation failed:  ", err)
				return err
			}

			// Update the spec hash
			current := installedTIS.GetAnnotations()
			current[tektoninstallerset.LastAppliedHashKey] = expectedSpecHash
			installedTIS.SetAnnotations(current)

			// Update the manifests
			installedTIS.Spec.Manifests = manifest.Resources()

			if _, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
				Update(ctx, installedTIS, metav1.UpdateOptions{}); err != nil {
				return err
			}

			// after updating installer set enqueue after a duration
			// to allow changes to get deployed
			r.enqueueAfter(td, 20*time.Second)
			return nil
		}
	}

	// Mark InstallerSetAvailable
	td.Status.MarkInstallerSetAvailable()

	ready := installedTIS.Status.GetCondition(apis.ConditionReady)
	if ready == nil {
		td.Status.MarkInstallerSetNotReady("Waiting for installation")
		r.enqueueAfter(td, 10*time.Second)
		return nil
	}

	if ready.Status == corev1.ConditionUnknown {
		td.Status.MarkInstallerSetNotReady("Waiting for installation")
		r.enqueueAfter(td, 10*time.Second)
		return nil
	} else if ready.Status == corev1.ConditionFalse {
		td.Status.MarkInstallerSetNotReady(ready.Message)
		r.enqueueAfter(td, 10*time.Second)
		return nil
	}

	// Mark InstallerSet Ready
	td.Status.MarkInstallerSetReady()

	if err := r.extension.PostReconcile(ctx, td); err != nil {
		td.Status.MarkPostReconcilerFailed(fmt.Sprintf("PostReconciliation failed: %s", err.Error()))
		return err
	}

	td.Status.MarkPostReconcilerComplete()
	return nil
}

func (r *Reconciler) updateTektonDashboardStatus(ctx context.Context, td *v1alpha1.TektonDashboard, createdIs *v1alpha1.TektonInstallerSet) error {
	// update the td with TektonInstallerSet and releaseVersion
	td.Status.SetTektonInstallerSet(createdIs.Name)
	td.Status.SetVersion(r.releaseVersion)

	// Update the status with TektonInstallerSet so that any new thread
	// reconciling with know that TektonInstallerSet is created otherwise
	// there will be 2 instance created if we don't update status here
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonDashboards().
		UpdateStatus(ctx, td, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return v1alpha1.RECONCILE_AGAIN_ERR
}

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	instance := comp.(*v1alpha1.TektonDashboard)
	extra := []mf.Transformer{
		common.ApplyProxySettings,
		common.AddConfiguration(instance.Spec.Config),
	}
	extra = append(extra, r.extension.Transformers(instance)...)
	return common.Transform(ctx, manifest, instance, extra...)
}

func (r *Reconciler) createInstallerSet(ctx context.Context, td *v1alpha1.TektonDashboard) (*v1alpha1.TektonInstallerSet, error) {

	var manifest mf.Manifest
	if td.Spec.Readonly {
		manifest = r.readonlyManifest
	} else {
		manifest = r.fullaccessManifest
	}

	if err := r.transform(ctx, &manifest, td); err != nil {
		td.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// create installer set
	tis := makeInstallerSet(td, manifest, r.releaseVersion)
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, tis, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdIs, nil
}

func makeInstallerSet(td *v1alpha1.TektonDashboard, manifest mf.Manifest, releaseVersion string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(td, td.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", v1alpha1.DashboardResourceName),
			Labels: map[string]string{
				tektoninstallerset.CreatedByKey: createdByValue,
			},
			Annotations: map[string]string{
				tektoninstallerset.ReleaseVersionKey:  releaseVersion,
				tektoninstallerset.TargetNamespaceKey: td.Spec.TargetNamespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}
