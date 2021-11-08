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
	"k8s.io/client-go/kubernetes"
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
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonresultconciler.Interface = (*Reconciler)(nil)
var _ tektonresultconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a TektonResult.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonResult) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// List all TektonResults to determine if cluster-scoped resources should be deleted.
	tps, err := r.operatorClientSet.OperatorV1alpha1().TektonResults().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list all TektonResults: %w", err)
	}

	for _, tp := range tps.Items {
		if tp.GetDeletionTimestamp().IsZero() {
			// Not deleting all TektonResults. Nothing to do here.
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
func (r *Reconciler) ReconcileKind(ctx context.Context, tr *v1alpha1.TektonResult) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tr.Status.InitializeConditions()
	tr.Status.ObservedGeneration = tr.Generation

	logger.Infow("Reconciling TektonResults", "status", tr.Status)

	if tr.GetName() != common.ResultResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			common.ResultResourceName,
			tr.GetName(),
		)
		logger.Error(msg)
		tr.GetStatus().MarkInstallFailed(msg)
		return nil
	}

	// find the valid tekton-pipeline installation
	tp, err := common.PipelineReady(r.pipelineInformer)
	if err != nil {
		if err.Error() == common.PipelineNotReady {
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

	stages := common.Stages{
		common.AppendTarget,
		r.transform,
		common.Install,
		common.CheckDeployments,
	}
	manifest := r.manifest.Append()
	return stages.Execute(ctx, &manifest, tr)
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

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	instance := comp.(*v1alpha1.TektonResult)
	targetNs := comp.GetSpec().GetTargetNamespace()
	extra := []mf.Transformer{
		common.ApplyProxySettings,
		common.ReplaceNamespaceInDeploymentArgs(targetNs),
		common.ReplaceNamespaceInDeploymentEnv(targetNs),
	}
	extra = append(extra, r.extension.Transformers(instance)...)
	return common.Transform(ctx, manifest, instance, extra...)
}

func (r *Reconciler) installed(ctx context.Context, instance v1alpha1.TektonComponent) (*mf.Manifest, error) {
	// Create new, empty manifest with valid client and logger
	installed := r.manifest.Append()
	stages := common.Stages{common.AppendInstalled, r.transform}
	err := stages.Execute(ctx, &installed, instance)
	return &installed, err
}
