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
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonpipelinereconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonpipeline"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	installer "github.com/tektoncd/operator/pkg/reconciler/common/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	tisBuilder "github.com/tektoncd/operator/pkg/reconciler/shared/tektoninstallerset"
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

const (
	// Pipelines ConfigMap
	FeatureFlag    = "feature-flags"
	ConfigDefaults = "config-defaults"
	ConfigMetrics  = "config-observability"

	proxyLabel     = "operator.tekton.dev/disable-proxy=true"
	createdByValue = "TektonPipeline"
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
	// metrics handles metrics for pipeline install
	metrics         *Recorder
	kubeClientSet   kubernetes.Interface
	operatorVersion string
	pipelineVersion string
}

var (
	ls = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: v1alpha1.PipelineResourceName,
		},
	}
)

// Check that our Reconciler implements controller.Reconciler
var _ tektonpipelinereconciler.Interface = (*Reconciler)(nil)
var _ tektonpipelinereconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a TektonPipeline.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonPipeline) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// Delete CRDs before deleting rest of resources so that any instance
	// of CRDs which has finalizer set will get deleted before we remove
	// the controller;s deployment for it
	if err := r.manifest.Filter(mf.CRDs).Delete(); err != nil {
		logger.Error("Failed to deleted CRDs for TektonPipeline")
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

	if tp.GetName() != v1alpha1.PipelineResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.PipelineResourceName,
			tp.GetName(),
		)
		logger.Error(msg)
		tp.Status.MarkNotReady(msg)
		return nil
	}

	// Pass the object through defaulting
	tp.SetDefaults(ctx)

	// Mark TektonPipeline Instance as Not Ready if an upgrade is needed
	if err := r.markUpgrade(ctx, tp); err != nil {
		return err
	}

	if err := tektoninstallerset.CleanUpObsoleteResources(ctx, r.operatorClientSet, createdByValue); err != nil {
		return err
	}

	if err := r.targetNamespaceCheck(ctx, tp); err != nil {
		return err
	}

	if err := r.extension.PreReconcile(ctx, tp); err != nil {
		tp.Status.MarkPreReconcilerFailed(fmt.Sprintf("PreReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PreReconcile Complete
	tp.Status.MarkPreReconcilerComplete()

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
		createdIs, err := r.createInstallerSet(ctx, tp)
		if err != nil {
			return err
		}
		// If there was no existing installer set, that means its a new install
		r.metrics.logMetrics(metricsNew, r.pipelineVersion, logger)

		return r.updateTektonPipelineStatus(ctx, tp, createdIs)
	}

	// If exists, then fetch the TektonInstallerSet
	installedTIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Get(ctx, existingInstallerSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			createdIs, err := r.createInstallerSet(ctx, tp)
			if err != nil {
				return err
			}
			// if there is version diff then its a call for upgrade
			if tp.Status.Version != r.pipelineVersion {
				r.metrics.logMetrics(metricsUpgrade, r.pipelineVersion, logger)
			}
			return r.updateTektonPipelineStatus(ctx, tp, createdIs)
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

	if installerSetTargetNamespace != tp.Spec.TargetNamespace || installerSetReleaseVersion != r.operatorVersion {
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
			tp.Status.MarkNotReady("Waiting for previous installer set to get deleted")
			return v1alpha1.REQUEUE_EVENT_AFTER
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

		// Hash of TektonInstallerSet Spec
		newIs, err := r.generateInstallerSet(ctx, tp)
		if err != nil {
			return err
		}
		expectedSpecHash := newIs.Annotations[v1alpha1.LastAppliedHashKey]

		// spec hash stored on installerSet
		lastAppliedHash := installedTIS.GetAnnotations()[v1alpha1.LastAppliedHashKey]

		if lastAppliedHash != expectedSpecHash {
			manifest := r.manifest
			if err := r.transform(ctx, &manifest, tp); err != nil {
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
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}

	// Mark InstallerSet Available
	tp.Status.MarkInstallerSetAvailable()

	ready := installedTIS.Status.GetCondition(apis.ConditionReady)
	if ready == nil {
		tp.Status.MarkInstallerSetNotReady("Waiting for installation")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	if ready.Status == corev1.ConditionUnknown {
		tp.Status.MarkInstallerSetNotReady("Waiting for installation")
		return v1alpha1.REQUEUE_EVENT_AFTER
	} else if ready.Status == corev1.ConditionFalse {
		tp.Status.MarkInstallerSetNotReady(ready.Message)
		manifest := r.manifest
		if err := r.transform(ctx, &manifest, tp); err != nil {
			logger.Error("manifest transformation failed:  ", err)
			return err
		}
		err = common.PreemptDeadlock(ctx, &manifest, r.kubeClientSet, v1alpha1.PipelineResourceName)
		if err != nil {
			logger.Error("preempt deadlock error: %v", err)
		}
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	// Mark InstallerSet Ready
	tp.Status.MarkInstallerSetReady()

	if err := r.extension.PostReconcile(ctx, tp); err != nil {
		tp.Status.MarkPostReconcilerFailed(fmt.Sprintf("PostReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PostReconcile Complete
	tp.Status.MarkPostReconcilerComplete()

	// Update the object for any spec changes
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonPipelines().Update(ctx, tp, v1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) updateTektonPipelineStatus(ctx context.Context, tp *v1alpha1.TektonPipeline, createdIs *v1alpha1.TektonInstallerSet) error {
	// update the tp with TektonInstallerSet and releaseVersion
	tp.Status.SetTektonInstallerSet(createdIs.Name)
	tp.Status.SetVersion(r.pipelineVersion)
	return nil
}

func (r *Reconciler) createInstallerSet(ctx context.Context, tp *v1alpha1.TektonPipeline) (*v1alpha1.TektonInstallerSet, error) {

	generatedIs, err := r.generateInstallerSet(ctx, tp)
	if err != nil {
		return nil, err
	}

	createdIs, err := tisBuilder.Create(ctx, generatedIs)
	if err != nil {
		return nil, err
	}

	return createdIs, nil
}

func (r *Reconciler) generateInstallerSet(ctx context.Context, tp *v1alpha1.TektonPipeline) (*v1alpha1.TektonInstallerSet, error) {
	// Creates a new default installer
	dis := installer.NewDefaultInstaller()

	manifest := r.manifest
	if err := r.transform(ctx, &manifest, tp); err != nil {
		tp.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// Adds the manifest to the installer
	dis.AddManifest(manifest)

	// Adds the labels to the installer
	dis.AddCreatedByLabel(createdByValue)
	dis.AddTypeLabel(v1alpha1.PipelineResourceName)
	dis.AddReleaseVersionLabel(r.operatorVersion)

	// Adds the ownerRef to the installer
	// ownerRef := *metav1.NewControllerRef(tp, tp.GetGroupVersionKind())
	// dis.AddOwnerReferences(ownerRef)

	// Adds the annotations to the installer
	dis.AddAnnotationsKeyVal(v1alpha1.TargetNamespaceKey, tp.Spec.TargetNamespace)

	generatedIs, err := tisBuilder.GenerateInstallerSetWithPrefixName(ctx, dis, v1alpha1.PipelineResourceName)
	if err != nil {
		return nil, err
	}

	return generatedIs, nil
}

func (r *Reconciler) targetNamespaceCheck(ctx context.Context, tp *v1alpha1.TektonPipeline) error {
	labels := r.manifest.Filter(mf.ByKind("Namespace")).Resources()[0].GetLabels()

	ns, err := r.kubeClientSet.CoreV1().Namespaces().Get(ctx, tp.GetSpec().GetTargetNamespace(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := common.CreateTargetNamespace(ctx, labels, tp, r.kubeClientSet); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	for key, value := range labels {
		ns.Labels[key] = value
	}
	_, err = r.kubeClientSet.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	return err
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
		common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdPipeline),
		common.AddConfigMapValues(FeatureFlag, pipeline.Spec.PipelineProperties),
		common.AddConfigMapValues(ConfigDefaults, pipeline.Spec.OptionalPipelineProperties),
		common.AddConfigMapValues(ConfigMetrics, pipeline.Spec.PipelineMetricsProperties),
		common.ApplyProxySettings,
		common.DeploymentImages(images),
		common.InjectLabelOnNamespace(proxyLabel),
		common.AddConfiguration(pipeline.Spec.Config),
	}
	trns = append(trns, extra...)
	return common.Transform(ctx, manifest, instance, trns...)
}

func (m *Recorder) logMetrics(status, version string, logger *zap.SugaredLogger) {
	err := m.Count(status, version)
	if err != nil {
		logger.Warnf("Failed to log the metrics : %v", err)
	}
}

func (r *Reconciler) markUpgrade(ctx context.Context, tp *v1alpha1.TektonPipeline) error {
	labels := tp.GetLabels()
	ver, ok := labels[v1alpha1.ReleaseVersionKey]
	if ok && ver == r.operatorVersion {
		return nil
	}
	if ok && ver != r.operatorVersion {
		tp.Status.MarkInstallerSetNotReady(v1alpha1.UpgradePending)
		tp.Status.MarkPreReconcilerFailed(v1alpha1.UpgradePending)
		tp.Status.MarkPostReconcilerFailed(v1alpha1.UpgradePending)
		tp.Status.MarkNotReady(v1alpha1.UpgradePending)
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels[v1alpha1.ReleaseVersionKey] = r.operatorVersion
	tp.SetLabels(labels)

	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonPipelines().Update(ctx,
		tp, v1.UpdateOptions{}); err != nil {
		return err
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}
