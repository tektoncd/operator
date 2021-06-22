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
	"fmt"

	mf "github.com/manifestival/manifestival"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektontriggerreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektontrigger"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Triggers ConfigMap
const (
	configDefaults = "config-defaults-triggers"
)

// Reconciler implements controller.Reconciler for TektonTrigger resources.
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
	// metrics handles metrics for trigger install
	metrics *Recorder

	pipelineInformer pipelineinformer.TektonPipelineInformer
}

// Check that our Reconciler implements controller.Reconciler
var _ tektontriggerreconciler.Interface = (*Reconciler)(nil)
var _ tektontriggerreconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a TektonTriggers.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonTrigger) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// List all TektonTriggers to determine if cluster-scoped resources should be deleted.
	tps, err := r.operatorClientSet.OperatorV1alpha1().TektonTriggers().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list all TektonTriggers: %w", err)
	}

	for _, tp := range tps.Items {
		if tp.GetDeletionTimestamp().IsZero() {
			// Not deleting all TektonTriggers. Nothing to do here.
			return nil
		}
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}
	logger.Info("Deleting cluster-scoped resources")
	manifest, err := r.installed(ctx, original)
	if err != nil {
		logger.Error("Unable to fetch installed manifest; no cluster-scoped resources will be finalized", err)
		return nil
	}
	if err := common.Uninstall(ctx, manifest, nil); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}
	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tt *v1alpha1.TektonTrigger) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tt.Status.InitializeConditions()
	tt.Status.ObservedGeneration = tt.Generation

	logger.Infow("Reconciling TektonTriggers", "status", tt.Status)

	if tt.GetName() != common.TriggerResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			common.TriggerResourceName,
			tt.GetName(),
		)
		logger.Error(msg)
		tt.GetStatus().MarkInstallFailed(msg)
		return nil
	}

	//find the valid tekton-pipeline installation
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady {
			tt.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return fmt.Errorf(common.PipelineNotReady)
		}
		// (tektonpipeline.opeator.tekton.dev instance not available yet)
		tt.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}
	tt.Status.MarkDependenciesInstalled()

	if err := r.extension.PreReconcile(ctx, tt); err != nil {
		r.metrics.logMetrics(metricsFail, logger)
		return err
	}
	stages := common.Stages{
		common.AppendTarget,
		r.transform,
		common.Install,
		common.CheckDeployments,
	}
	manifest := r.manifest.Append()
	err := stages.Execute(ctx, &manifest, tt)
	if err != nil {
		r.metrics.logMetrics(metricsFail, logger)
		return err
	}
	r.metrics.logMetrics(metricsSuccess, logger)
	return err
}

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	trigger := comp.(*v1alpha1.TektonTrigger)
	triggerImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.TriggersImagePrefix))
	// adding extension's transformers first to run them before `extra` transformers
	trns := r.extension.Transformers(trigger)
	extra := []mf.Transformer{
		common.AddConfigMapValues(configDefaults, trigger.Spec.TriggersProperties),
		common.ApplyProxySettings,
		common.DeploymentImages(triggerImages),
		common.AddConfiguration(trigger.Spec.Config),
	}
	trns = append(trns, extra...)
	return common.Transform(ctx, manifest, trigger, trns...)
}

func (r *Reconciler) installed(ctx context.Context, instance v1alpha1.TektonComponent) (*mf.Manifest, error) {
	// Create new, empty manifest with valid client and logger
	installed := r.manifest.Append()
	// TODO: add ingress, etc
	stages := common.Stages{common.AppendInstalled, r.transform}
	err := stages.Execute(ctx, &installed, instance)
	return &installed, err
}

func (m *Recorder) logMetrics(status string, logger *zap.SugaredLogger) {
	err := m.Count(status)
	if err != nil {
		logger.Warnf("Failed to log the metrics : %v", err)
	}

}
