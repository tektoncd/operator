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

package tektonresult

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineInformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektonresultconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonresult"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	DbSecretName  = "tekton-results-postgres"
	TlsSecretName = "tekton-results-tls"
)

// Reconciler implements controller.Reconciler for TektonResult resources.
type Reconciler struct {
	// kubeClientSet allows us to talk to the k8s for core APIs
	kubeClientSet kubernetes.Interface
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// manifest is empty, but with a valid client and logger. all
	// manifests are immutable, and any created during reconcile are
	// expected to be appended to this one, obviating the passing of
	// client & logger
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension

	pipelineInformer pipelineInformer.TektonPipelineInformer

	operatorVersion string
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonresultconciler.Interface = (*Reconciler)(nil)
var _ tektonresultconciler.Finalizer = (*Reconciler)(nil)

var (
	ls = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: v1alpha1.ResultResourceName,
		},
	}
)

const createdByValue = "TektonResult"

// FinalizeKind removes all resources after deletion of a TektonResult.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonResult) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}
	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err != nil {
		logger.Error("Failed to delete installer set created by TektonResult", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}
	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tr *v1alpha1.TektonResult) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tr.Status.InitializeConditions()
	tr.Status.ObservedGeneration = tr.Generation

	logger.Infow("Reconciling TektonResults", "status", tr.Status)

	if tr.GetName() != v1alpha1.ResultResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.ResultResourceName,
			tr.GetName(),
		)
		logger.Error(msg)
		tr.Status.MarkNotReady(msg)
		return nil
	}

	// find the valid tekton-pipeline installation
	tp, err := common.PipelineReady(r.pipelineInformer)
	if err != nil {
		if err.Error() == common.PipelineNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			tr.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return fmt.Errorf(common.PipelineNotReady)
		}
		// tektonpipeline.operator.tekton.dev instance not available yet
		tr.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}

	if tp.GetSpec().GetTargetNamespace() != tr.GetSpec().GetTargetNamespace() {
		errMsg := fmt.Sprintf("tekton-pipelines is missing in %s namespace", tr.GetSpec().GetTargetNamespace())
		tr.Status.MarkDependencyMissing(errMsg)
		return fmt.Errorf(errMsg)
	}

	// check if the secrets are created
	if err := r.validateSecretsAreCreated(ctx, tr); err != nil {
		return err
	}
	tr.Status.MarkDependenciesInstalled()

	// Check if an tektoninstallerset already exists, if not then create
	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}
	existingInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
	if err != nil {
		return err
	}
	if existingInstallerSet == "" {
		createdIs, err := r.createInstallerSet(ctx, tr)
		if err != nil {
			return err
		}

		return r.updateTektonResultsStatus(ctx, tr, createdIs)
	}

	// If exists, then fetch the TektonInstallerSet
	installedTIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Get(ctx, existingInstallerSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			createdIs, err := r.createInstallerSet(ctx, tr)
			if err != nil {
				return err
			}
			return r.updateTektonResultsStatus(ctx, tr, createdIs)
		}
		logger.Error("failed to get InstallerSet: %s", err)
		return err
	}

	installerSetTargetNamespace := installedTIS.Annotations[v1alpha1.TargetNamespaceKey]
	installerSetReleaseVersion := installedTIS.Labels[v1alpha1.ReleaseVersionKey]

	// Check if TargetNamespace of existing TektonInstallerSet is same as expected
	// Check if Release Version in TektonInstallerSet is same as expected
	// If any of the thing above is not same then delete the existing TektonInstallerSet
	// and create a new with expected properties
	if installerSetTargetNamespace != tr.Spec.TargetNamespace || installerSetReleaseVersion != r.operatorVersion {
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
			tr.Status.MarkNotReady("Waiting for previous installer set to get deleted")
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get InstallerSet: %s", err)
			return err
		}
		return nil

	} else {
		// If target namespace and version are not changed then check if spec
		// of TektonResult is changed by checking hash stored as annotation on
		// TektonInstallerSet with computing new hash of TektonResult Spec

		// Hash of TektonResult Spec
		expectedSpecHash, err := hash.Compute(tr.Spec)
		if err != nil {
			return err
		}

		// spec hash stored on installerSet
		lastAppliedHash := installedTIS.GetAnnotations()[v1alpha1.LastAppliedHashKey]

		if lastAppliedHash != expectedSpecHash {

			if err := r.transform(ctx, &r.manifest, tr); err != nil {
				logger.Error("manifest transformation failed:  ", err)
				return err
			}

			// Update the spec hash
			current := installedTIS.GetAnnotations()
			current[v1alpha1.LastAppliedHashKey] = expectedSpecHash
			installedTIS.SetAnnotations(current)

			// Update the manifests
			installedTIS.Spec.Manifests = r.manifest.Resources()

			if _, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
				Update(ctx, installedTIS, metav1.UpdateOptions{}); err != nil {
				return err
			}

			// after updating installer set enqueue after a duration
			// to allow changes to get deployed
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}

	// Mark InstallerSetAvailable
	tr.Status.MarkInstallerSetAvailable()

	ready := installedTIS.Status.GetCondition(apis.ConditionReady)
	if ready == nil {
		tr.Status.MarkInstallerSetNotReady("Waiting for installation")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	if ready.Status == corev1.ConditionUnknown {
		tr.Status.MarkInstallerSetNotReady("Waiting for installation")
		return v1alpha1.REQUEUE_EVENT_AFTER
	} else if ready.Status == corev1.ConditionFalse {
		tr.Status.MarkInstallerSetNotReady(ready.Message)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	// Mark InstallerSet Ready
	tr.Status.MarkInstallerSetReady()

	return nil
}

func (r *Reconciler) updateTektonResultsStatus(ctx context.Context, tr *v1alpha1.TektonResult, createdIs *v1alpha1.TektonInstallerSet) error {
	// update the tr with TektonInstallerSet
	tr.Status.SetTektonInstallerSet(createdIs.Name)

	return nil
}

// TektonResults expects secrets to be created before installing
func (r *Reconciler) validateSecretsAreCreated(ctx context.Context, tr *v1alpha1.TektonResult) error {
	logger := logging.FromContext(ctx)
	_, err := r.kubeClientSet.CoreV1().Secrets(tr.Spec.TargetNamespace).Get(ctx, DbSecretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err)
			tr.Status.MarkDependencyMissing(fmt.Sprintf("%s secret is missing", DbSecretName))
			return err
		}
		logger.Error(err)
		return err
	}
	_, err = r.kubeClientSet.CoreV1().Secrets(tr.Spec.TargetNamespace).Get(ctx, TlsSecretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err)
			tr.Status.MarkDependencyMissing(fmt.Sprintf("%s secret is missing", TlsSecretName))
			return err
		}
		logger.Error(err)
		return err
	}
	return nil
}
