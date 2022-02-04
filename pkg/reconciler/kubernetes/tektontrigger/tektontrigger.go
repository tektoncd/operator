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

package tektontrigger

import (
	"context"
	stdError "errors"
	"fmt"
	"time"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektontriggerreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektontrigger"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Triggers ConfigMap
const (
	ConfigDefaults = "config-defaults-triggers"
	FeatureFlag    = "feature-flags-triggers"

	createdByValue = "TektonTrigger"
)

// Reconciler implements controller.Reconciler for TektonTrigger resources.
type Reconciler struct {
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// manifest has the source manifest of Tekton Triggers for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// pipelineInformer to query for TektonPipeline
	// metrics handles metrics for trigger install
	metrics *Recorder

	pipelineInformer pipelineinformer.TektonPipelineInformer
	// enqueueAfter enqueues a obj after a duration
	enqueueAfter    func(obj interface{}, after time.Duration)
	triggersVersion string
	operatorVersion string
	kubeClientSet   kubernetes.Interface
}

var (
	ls = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: v1alpha1.TriggerResourceName,
		},
	}
)

// Check that our Reconciler implements controller.Reconciler
var _ tektontriggerreconciler.Interface = (*Reconciler)(nil)
var _ tektontriggerreconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a TektonTriggers.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonTrigger) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// Delete CRDs before deleting rest of resources so that any instance
	// of CRDs which has finalizer set will get deleted before we remove
	// the controller;s deployment for it
	if err := r.manifest.Filter(mf.CRDs).Delete(); err != nil {
		logger.Error("Failed to deleted CRDs for TektonTrigger")
		return err
	}

	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}
	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err != nil {
		logger.Error("Failed to delete installer set created by TektonTrigger", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tt *v1alpha1.TektonTrigger) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tt.Status.InitializeConditions()

	if tt.GetName() != v1alpha1.TriggerResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.TriggerResourceName,
			tt.GetName(),
		)
		logger.Error(msg)
		tt.Status.MarkNotReady(msg)
		return nil
	}

	// Make sure TektonPipeline is installed before proceeding with
	// TektonTrigger
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady {
			tt.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			r.enqueueAfter(tt, 10*time.Second)
			return nil
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		tt.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}
	tt.Status.MarkDependenciesInstalled()

	// Pass the object through defaulting
	tt.SetDefaults(ctx)
	// Mark TektonTrigger Instance as Not Ready if an upgrade is needed
	if err := r.markUpgrade(ctx, tt); err != nil {
		return err
	}

	if err := tektoninstallerset.CleanUpObsoleteResources(ctx, r.operatorClientSet, createdByValue); err != nil {
		return err
	}

	if err := r.extension.PreReconcile(ctx, tt); err != nil {
		tt.Status.MarkPreReconcilerFailed(fmt.Sprintf("PreReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PreReconcile Complete
	tt.Status.MarkPreReconcilerComplete()

	// Check if an tekton installer set already exists, if not then create
	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}
	existingInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
	if err != nil {
		return err
	}
	if existingInstallerSet == "" {
		createdIs, err := r.createInstallerSet(ctx, tt)
		if err != nil {
			return err
		}

		// If there was no existing installer set, that means its a new install
		r.metrics.logMetrics(metricsNew, r.triggersVersion, logger)

		return r.updateTektonTriggerStatus(ctx, tt, createdIs)
	}

	// If exists, then fetch the TektonInstallerSet
	installedTIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Get(ctx, existingInstallerSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			createdIs, err := r.createInstallerSet(ctx, tt)
			if err != nil {
				return err
			}
			// if there is version diff then its a call for upgrade
			if tt.Status.Version != r.triggersVersion {
				r.metrics.logMetrics(metricsUpgrade, r.triggersVersion, logger)
			}
			return r.updateTektonTriggerStatus(ctx, tt, createdIs)
		}
		logger.Error("failed to get InstallerSet: %s", err)
		return err
	}

	installerSetTargetNamespace := installedTIS.Annotations[v1alpha1.TargetNamespaceKey]
	installerSetReleaseVersion := installedTIS.Labels[v1alpha1.ReleaseVersionKey]

	// Check if TargetNamespace of existing TektonInstallerSet is same as expected
	// Check if Release Version in TektonInstallerSet is same as expected
	// If any of the thing above is not same the delete the existing TektonInstallerSet
	// and create a new with expected properties

	if installerSetTargetNamespace != tt.Spec.TargetNamespace || installerSetReleaseVersion != r.operatorVersion {

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
			tt.Status.MarkNotReady("Waiting for previous installer set to get deleted")
			r.enqueueAfter(tt, 10*time.Second)
			return nil
		}
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get InstallerSet: %s", err)
			return err
		}

		return nil

	} else {
		// If target namespace and version are not changed then check if spec
		// of TektonTrigger is changed by checking hash stored as annotation on
		// TektonInstallerSet with computing new hash of TektonTrigger Spec

		// Hash of TektonPipeline Spec
		expectedSpecHash, err := hash.Compute(tt.Spec)
		if err != nil {
			return err
		}

		// spec hash stored on installerSet
		lastAppliedHash := installedTIS.GetAnnotations()[v1alpha1.LastAppliedHashKey]

		if lastAppliedHash != expectedSpecHash {

			manifest := r.manifest
			if err := r.transform(ctx, &manifest, tt); err != nil {
				logger.Error("manifest transformation failed:  ", err)
				return err
			}

			// Update the spec hash
			current := installedTIS.GetAnnotations()
			current[v1alpha1.LastAppliedHashKey] = expectedSpecHash
			installedTIS.SetAnnotations(current)

			// Update the manifests
			installedTIS.Spec.Manifests = manifest.Resources()

			if _, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
				Update(ctx, installedTIS, metav1.UpdateOptions{}); err != nil {
				return err
			}

			// after updating installer set enqueue after a duration
			// to allow changes to get deployed
			r.enqueueAfter(tt, 20*time.Second)
			return nil
		}
	}

	// Mark InstallerSet Available
	tt.Status.MarkInstallerSetAvailable()

	ready := installedTIS.Status.GetCondition(apis.ConditionReady)
	if ready == nil {
		tt.Status.MarkInstallerSetNotReady("Waiting for installation")
		r.enqueueAfter(tt, 10*time.Second)
		return nil
	}

	if ready.Status == corev1.ConditionUnknown {
		tt.Status.MarkInstallerSetNotReady("Waiting for installation")
		r.enqueueAfter(tt, 10*time.Second)
		return nil
	} else if ready.Status == corev1.ConditionFalse {
		tt.Status.MarkInstallerSetNotReady(ready.Message)
		manifest := r.manifest
		if err := r.transform(ctx, &manifest, tt); err != nil {
			logger.Error("manifest transformation failed:  ", err)
			return err
		}
		err = common.PreemptDeadlock(ctx, &manifest, r.kubeClientSet, v1alpha1.TriggerResourceName)
		r.enqueueAfter(tt, 10*time.Second)
		return err
	}

	// Mark InstallerSet Ready
	tt.Status.MarkInstallerSetReady()

	if err := r.extension.PostReconcile(ctx, tt); err != nil {
		tt.Status.MarkPostReconcilerFailed(fmt.Sprintf("PostReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PostReconcile Complete
	tt.Status.MarkPostReconcilerComplete()

	// Update the object for any spec changes
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonTriggers().Update(ctx, tt, v1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	trigger := comp.(*v1alpha1.TektonTrigger)
	triggerImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.TriggersImagePrefix))
	// adding extension's transformers first to run them before `extra` transformers
	trns := r.extension.Transformers(trigger)
	extra := []mf.Transformer{
		common.AddConfigMapValues(ConfigDefaults, trigger.Spec.OptionalTriggersProperties),
		common.AddConfigMapValues(FeatureFlag, trigger.Spec.TriggersProperties),
		common.ApplyProxySettings,
		common.DeploymentImages(triggerImages),
		common.AddConfiguration(trigger.Spec.Config),
	}
	trns = append(trns, extra...)
	return common.Transform(ctx, manifest, trigger, trns...)
}

func (r *Reconciler) updateTektonTriggerStatus(ctx context.Context, tt *v1alpha1.TektonTrigger, createdIs *v1alpha1.TektonInstallerSet) error {
	// update the tt with TektonInstallerSet and releaseVersion
	tt.Status.SetTektonInstallerSet(createdIs.Name)
	tt.Status.SetVersion(r.triggersVersion)

	// Update the status with TektonInstallerSet so that any new thread
	// reconciling with know that TektonInstallerSet is created otherwise
	// there will be 2 instance created if we don't update status here
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonTriggers().
		UpdateStatus(ctx, tt, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return stdError.New("ensuring Reconcile TektonTrigger status update")
}

func (r *Reconciler) createInstallerSet(ctx context.Context, tt *v1alpha1.TektonTrigger) (*v1alpha1.TektonInstallerSet, error) {

	manifest := r.manifest
	if err := r.transform(ctx, &manifest, tt); err != nil {
		tt.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// compute the hash of tektontrigger spec and store as an annotation
	// in further reconciliation we compute hash of tt spec and check with
	// annotation, if they are same then we skip updating the object
	// otherwise we update the manifest
	specHash, err := hash.Compute(tt.Spec)
	if err != nil {
		return nil, err
	}

	// create installer set
	tis := makeInstallerSet(tt, manifest, specHash, r.operatorVersion)
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, tis, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdIs, nil
}

func makeInstallerSet(tt *v1alpha1.TektonTrigger, manifest mf.Manifest, ttSpecHash, releaseVersion string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(tt, tt.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", v1alpha1.TriggerResourceName),
			Labels: map[string]string{
				v1alpha1.CreatedByKey:      createdByValue,
				v1alpha1.ReleaseVersionKey: releaseVersion,
				v1alpha1.InstallerSetType:  v1alpha1.TriggerResourceName,
			},
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: tt.Spec.TargetNamespace,
				v1alpha1.LastAppliedHashKey: ttSpecHash,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}

func (m *Recorder) logMetrics(status, version string, logger *zap.SugaredLogger) {
	err := m.Count(status, version)
	if err != nil {
		logger.Warnf("Failed to log the metrics : %v", err)
	}
}

func (r *Reconciler) markUpgrade(ctx context.Context, tt *v1alpha1.TektonTrigger) error {
	labels := tt.GetLabels()
	ver, ok := labels[v1alpha1.ReleaseVersionKey]
	if ok && ver == r.operatorVersion {
		return nil
	}
	if ok && ver != r.operatorVersion {
		tt.Status.MarkInstallerSetNotReady(v1alpha1.UpgradePending)
		tt.Status.MarkPreReconcilerFailed(v1alpha1.UpgradePending)
		tt.Status.MarkPostReconcilerFailed(v1alpha1.UpgradePending)
		tt.Status.MarkNotReady(v1alpha1.UpgradePending)
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels[v1alpha1.ReleaseVersionKey] = r.operatorVersion
	tt.SetLabels(labels)

	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonTriggers().Update(ctx,
		tt, v1.UpdateOptions{}); err != nil {
		return err
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}
