/*
Copyright 2019 The Tekton Authors

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

package tektonpipeline

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonpipelinereconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonpipeline"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// Pipelines ConfigMap
	featureFlag    = "feature-flags"
	configDefaults = "config-defaults"

	proxyLabel = "operator.tekton.dev/disable-proxy=true"

	// TektonInstallerSet keys
	lastAppliedHashKey = "operator.tekton.dev/last-applied-hash"
	createdByKey       = "operator.tekton.dev/created-by"
	createdByValue     = "TektonPipeline"
	releaseVersionKey  = "operator.tekton.dev/release-version"
	targetNamespaceKey = "operator.tekton.dev/target-namespace"
)

// Reconciler implements controller.Reconciler for TektonPipeline resources.
type Reconciler struct {
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	//manifest has the source manifest of Tekton Pipeline for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// enqueueAfter enqueues a obj after a duration
	enqueueAfter func(obj interface{}, after time.Duration)
	// releaseVersion describes the current pipelines version
	releaseVersion string
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonpipelinereconciler.Interface = (*Reconciler)(nil)
var _ tektonpipelinereconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a TektonPipeline.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonPipeline) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", createdByKey, createdByValue),
		}); err != nil {
		logger.Error("Failed to delete installer set created by TektonPipeline", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tp *v1alpha1.TektonPipeline) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tp.Status.InitializeConditions()

	if tp.GetName() != common.PipelineResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			common.PipelineResourceName,
			tp.GetName(),
		)
		logger.Error(msg)
		tp.Status.MarkNotReady(msg)
		return nil
	}

	// Pass the object through defaulting
	tp.SetDefaults(ctx)

	if err := r.extension.PreReconcile(ctx, tp); err != nil {
		tp.Status.MarkPreReconcilerFailed(fmt.Sprintf("PreReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PreReconcile Complete
	tp.Status.MarkPreReconcilerComplete()

	// Check if an tekton installer set already exists, if not then create
	existingInstallerSet := tp.Status.GetTektonInstallerSet()
	if existingInstallerSet == "" {
		return r.createInstallerSet(ctx, tp)
	}

	// If exists, then fetch the TektonInstallerSet
	installedTIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Get(context.TODO(), existingInstallerSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.createInstallerSet(ctx, tp)
		}
		logger.Error("failed to get InstallerSet: %s", err)
		return err
	}

	installerSetTargetNamespace := installedTIS.Annotations[targetNamespaceKey]
	installerSetReleaseVersion := installedTIS.Annotations[releaseVersionKey]

	// Check if TargetNamespace of existing TektonInstallerSet is same as expected
	// Check if Release Version in TektonInstallerSet is same as expected
	// If any of the thing above is not same the delete the existing TektonInstallerSet
	// and create a new with expected properties

	if installerSetTargetNamespace != tp.Spec.TargetNamespace || installerSetReleaseVersion != r.releaseVersion {

		// Delete the existing TektonInstallerSet
		err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Delete(context.TODO(), existingInstallerSet, metav1.DeleteOptions{})
		if err != nil {
			logger.Error("failed to delete InstallerSet: %s", err)
			return err
		}

		// Make sure the TektonInstallerSet is deleted
		_, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Get(context.TODO(), existingInstallerSet, metav1.GetOptions{})
		if err == nil {
			tp.Status.MarkNotReady("Waiting for previous installer set to get deleted")
			r.enqueueAfter(tp, 10*time.Second)
			return nil
		}
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get InstallerSet: %s", err)
			return err
		}

		return nil

	} else {
		// If target namespace and version are not changed then check if spec
		// of TektonPipeline is changed by checking hash stored as annotation on
		// TektonInstallerSet with computing new hash of TektonPipeline Spec

		// Hash of TektonPipeline Spec
		expectedSpecHash, err := computeHashOf(tp.Spec)
		if err != nil {
			return err
		}

		// spec hash stored on installerSet
		lastAppliedHash := installedTIS.GetAnnotations()[lastAppliedHashKey]

		if lastAppliedHash != expectedSpecHash {

			manifest := r.manifest
			if err := r.transform(ctx, &manifest, tp); err != nil {
				logger.Error("manifest transformation failed:  ", err)
				return err
			}

			// Update the spec hash
			current := installedTIS.GetAnnotations()
			current[lastAppliedHash] = expectedSpecHash
			installedTIS.SetAnnotations(current)

			// Update the manifests
			installedTIS.Spec.Manifests = manifest.Resources()

			_, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
				Update(context.TODO(), installedTIS, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			// after updating installer set enqueue after a duration
			// to allow changes to get deployed
			r.enqueueAfter(tp, 20*time.Second)
			return nil
		}
	}

	// Mark InstallerSet Available
	tp.Status.MarkInstallerSetAvailable()

	ready := installedTIS.Status.GetCondition(apis.ConditionReady)
	if ready == nil {
		tp.Status.MarkInstallerSetNotReady("Waiting for installation")
		r.enqueueAfter(tp, 10*time.Second)
		return nil
	}

	if ready.Status == corev1.ConditionUnknown {
		tp.Status.MarkInstallerSetNotReady("Waiting for installation")
		r.enqueueAfter(tp, 10*time.Second)
		return nil
	} else if ready.Status == corev1.ConditionFalse {
		tp.Status.MarkInstallerSetNotReady(ready.Message)
		r.enqueueAfter(tp, 10*time.Second)
		return nil
	}

	// Mark InstallerSet Ready
	tp.Status.MarkInstallerSetReady()

	if err := r.extension.PostReconcile(ctx, tp); err != nil {
		tp.Status.MarkPostReconcilerFailed(fmt.Sprintf("PostReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PostReconcile Complete
	tp.Status.MarkPostReconcilerComplete()

	return nil
}

func (r *Reconciler) createInstallerSet(ctx context.Context, tp *v1alpha1.TektonPipeline) error {

	manifest := r.manifest
	if err := r.transform(ctx, &manifest, tp); err != nil {
		tp.Status.MarkNotReady("transformation failed: " + err.Error())
		return err
	}

	// compute the hash of tektonpipeline spec and store as an annotation
	// in further reconciliation we compute hash of tp spec and check with
	// annotation, if they are same then we skip updating the object
	// otherwise we update the manifest
	specHash, err := computeHashOf(tp.Spec)
	if err != nil {
		return err
	}

	// create installer set
	tis := makeInstallerSet(tp, manifest, specHash, r.releaseVersion)
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(context.TODO(), tis, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// update the tp with TektonInstallerSet and releaseVersion
	tp.Status.SetTektonInstallerSet(createdIs.Name)
	tp.Status.SetVersion(r.releaseVersion)

	// Update the status with TektonInstallerSet so that any new thread
	// reconciling with know that TektonInstallerSet is created otherwise
	// there will be 2 instance created if we don't update status here
	_, err = r.operatorClientSet.OperatorV1alpha1().TektonPipelines().
		UpdateStatus(context.TODO(), tp, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func makeInstallerSet(tp *v1alpha1.TektonPipeline, manifest mf.Manifest, tpSpecHash, releaseVersion string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(tp, tp.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", common.PipelineResourceName),
			Labels: map[string]string{
				createdByKey: createdByValue,
			},
			Annotations: map[string]string{
				releaseVersionKey:  releaseVersion,
				targetNamespaceKey: tp.Spec.TargetNamespace,
				lastAppliedHashKey: tpSpecHash,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	pipeline := comp.(*v1alpha1.TektonPipeline)
	images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.PipelinesImagePrefix))
	instance := comp.(*v1alpha1.TektonPipeline)
	// adding extension's transformers first to run them before `extra` transformers
	trns := r.extension.Transformers(instance)
	extra := []mf.Transformer{
		common.AddConfigMapValues(featureFlag, pipeline.Spec.PipelineProperties),
		common.AddConfigMapValues(configDefaults, pipeline.Spec.OptionalPipelineProperties),
		common.ApplyProxySettings,
		common.DeploymentImages(images),
		common.InjectLabelOnNamespace(proxyLabel),
		common.AddConfiguration(pipeline.Spec.Config),
	}
	trns = append(trns, extra...)
	return common.Transform(ctx, manifest, instance, trns...)
}

func computeHashOf(obj interface{}) (string, error) {
	h := sha256.New()
	d, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	h.Write(d)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
