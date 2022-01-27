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

package tektonchains

import (
	"context"
	"fmt"
	"time"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektonchainsreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonchains"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonChains resources.
type Reconciler struct {
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// manifest has the source manifest of Tekton Triggers for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	// enqueueAfter enqueues a obj after a duration
	enqueueAfter func(obj interface{}, after time.Duration)
	extension    common.Extension
	// releaseVersion describes the current chains version
	releaseVersion string
	// pipelineInformer provides access to a shared informer and lister for
	// TektonPipelines
	pipelineInformer pipelineinformer.TektonPipelineInformer
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonchainsreconciler.Interface = (*Reconciler)(nil)
var _ tektonchainsreconciler.Finalizer = (*Reconciler)(nil)

const createdByValue = "TektonChains"

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tc *v1alpha1.TektonChains) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tc.Status.InitializeConditions()
	tc.Status.ObservedGeneration = tc.Generation

	logger.Infow("Reconciling TektonChains", "status", tc.Status)

	if tc.GetName() != v1alpha1.ChainsResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.ChainsResourceName,
			tc.GetName(),
		)
		logger.Error(msg)
		tc.GetStatus().MarkInstallFailed(msg)
		return nil
	}

	// find a valid TektonPipeline installation
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady {
			tc.Status.MarkDependencyInstalling("TektonPipeline is still installing")
			// wait for TektonPipeline status to change
			return fmt.Errorf(common.PipelineNotReady)
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		tc.Status.MarkDependencyMissing("TektonPipeline does not exist")
		return err
	}
	tc.Status.MarkDependenciesInstalled()

	// Pass the object through defaulting
	tc.SetDefaults(ctx)

	if err := r.extension.PreReconcile(ctx, tc); err != nil {
		tc.Status.MarkPreReconcilerFailed(fmt.Sprintf("PreReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PreReconcile Complete
	tc.Status.MarkPreReconcilerComplete()

	// Check if a Tekton InstallerSet already exists, if not then create one
	existingInstallerSet := tc.Status.GetTektonInstallerSet()
	if existingInstallerSet == "" {
		tc.Status.MarkInstallerSetNotAvailable("Chains InstallerSet not available")

		createdIs, err := r.createInstallerSet(ctx, tc)
		if err != nil {
			return err
		}

		return r.updateTektonChainsStatus(ctx, tc, createdIs)
	}

	// If exists, then fetch the InstallerSet
	installedTIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Get(ctx, existingInstallerSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			createdIs, err := r.createInstallerSet(ctx, tc)
			if err != nil {
				return err
			}
			return r.updateTektonChainsStatus(ctx, tc, createdIs)
		}
		logger.Error("failed to get InstallerSet: %s", err)
		return err
	}

	installerSetTargetNamespace := installedTIS.Annotations[tektoninstallerset.TargetNamespaceKey]
	installerSetReleaseVersion := installedTIS.Annotations[tektoninstallerset.ReleaseVersionKey]

	// Check if TargetNamespace of existing TektonInstallerSet is same as expected
	// Check if Release Version in TektonInstallerSet is same as expected
	// If any of the above things is not same then delete the existing TektonInstallerSet
	// and create a new with expected properties

	if installerSetTargetNamespace != tc.Spec.TargetNamespace || installerSetReleaseVersion != r.releaseVersion {
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
			tc.Status.MarkNotReady("Waiting for previous installer set to get deleted")
			r.enqueueAfter(tc, 10*time.Second)
			return nil
		}
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get InstallerSet: %s", err)
			return err
		}
		return nil

	} else {
		// If target namespace and version are not changed then check if Chains
		// spec is changed by checking hash stored as annotation on
		// TektonInstallerSet with computing new hash of TektonChains Spec

		// Hash of TektonChains Spec
		expectedSpecHash, err := hash.Compute(tc.Spec)
		if err != nil {
			return err
		}

		// spec hash stored on installerSet
		lastAppliedHash := installedTIS.GetAnnotations()[tektoninstallerset.LastAppliedHashKey]

		if lastAppliedHash != expectedSpecHash {

			manifest := r.manifest
			if err := r.transform(ctx, &manifest, tc); err != nil {
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
			r.enqueueAfter(tc, 20*time.Second)
			return nil
		}
	}

	// Mark InstallerSetAvailable
	tc.Status.MarkInstallerSetAvailable()

	ready := installedTIS.Status.GetCondition(apis.ConditionReady)
	if ready == nil {
		tc.Status.MarkInstallerSetNotReady("Waiting for installation")
		r.enqueueAfter(tc, 10*time.Second)
		return nil
	}

	if ready.Status == corev1.ConditionUnknown {
		tc.Status.MarkInstallerSetNotReady("Waiting for installation")
		r.enqueueAfter(tc, 10*time.Second)
		return nil
	} else if ready.Status == corev1.ConditionFalse {
		tc.Status.MarkInstallerSetNotReady(ready.Message)
		r.enqueueAfter(tc, 10*time.Second)
		return nil
	}

	// Mark InstallerSet Ready
	tc.Status.MarkInstallerSetReady()

	if err := r.extension.PostReconcile(ctx, tc); err != nil {
		tc.Status.MarkPostReconcilerFailed(fmt.Sprintf("PostReconciliation failed: %s", err.Error()))
		return err
	}

	tc.Status.MarkPostReconcilerComplete()
	return nil
}

// FinalizeKind removes all resources after deletion of a TektonChains.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonChains) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// Delete CRDs before deleting rest of resources so that any instance
	// of CRDs which has finalizer set will get deleted before we remove
	// the controller's deployment for it
	if err := r.manifest.Filter(mf.CRDs).Delete(); err != nil {
		logger.Error("Failed to deleted CRDs for TektonChains")
		return err
	}

	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", tektoninstallerset.CreatedByKey, createdByValue),
		}); err != nil {
		logger.Error("Failed to delete installer set created by TektonChains", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	return nil
}

func (r *Reconciler) updateTektonChainsStatus(ctx context.Context, tc *v1alpha1.TektonChains, createdIs *v1alpha1.TektonInstallerSet) error {
	// update the tc with TektonInstallerSet and releaseVersion
	tc.Status.SetTektonInstallerSet(createdIs.Name)
	tc.Status.SetVersion(r.releaseVersion)

	// Update the status with TektonInstallerSet so that any new thread
	// reconciling knows that TektonInstallerSet is created otherwise
	// there will be 2 instances created if we don't update status here
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonChainses().
		UpdateStatus(ctx, tc, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return v1alpha1.RECONCILE_AGAIN_ERR
}

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	instance := comp.(*v1alpha1.TektonChains)
	extra := []mf.Transformer{
		common.ApplyProxySettings,
		common.AddConfiguration(instance.Spec.Config),
	}
	extra = append(extra, r.extension.Transformers(instance)...)
	return common.Transform(ctx, manifest, instance, extra...)
}

func (r *Reconciler) createInstallerSet(ctx context.Context, tc *v1alpha1.TektonChains) (*v1alpha1.TektonInstallerSet, error) {

	manifest := r.manifest
	if err := r.transform(ctx, &manifest, tc); err != nil {
		tc.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// compute the hash of tektonchains spec and store as an annotation
	// in further reconciliation we compute hash of tc spec and check with
	// annotation, if they are same then we skip updating the object
	// otherwise we update the manifest
	specHash, err := hash.Compute(tc.Spec)
	if err != nil {
		return nil, err
	}

	// create installer set
	tis := makeInstallerSet(tc, manifest, specHash, r.releaseVersion)
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, tis, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdIs, nil
}

func makeInstallerSet(tc *v1alpha1.TektonChains, manifest mf.Manifest, tdSpecHash, releaseVersion string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(tc, tc.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", v1alpha1.ChainsResourceName),
			Labels: map[string]string{
				tektoninstallerset.CreatedByKey: createdByValue,
			},
			Annotations: map[string]string{
				tektoninstallerset.ReleaseVersionKey:  releaseVersion,
				tektoninstallerset.TargetNamespaceKey: tc.Spec.TargetNamespace,
				tektoninstallerset.LastAppliedHashKey: tdSpecHash,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}
