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
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektontriggerreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektontrigger"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const resourceKind = v1alpha1.KindTektonTrigger

// Reconciler implements controller.Reconciler for TektonTrigger resources.
type Reconciler struct {
	// kube client to interact with core k8s resources
	kubeClientSet kubernetes.Interface
	// installer Set client to do CRUD operations for components
	installerSetClient *client.InstallerSetClient
	// pipelineInformer to query for TektonPipeline
	pipelineInformer pipelineinformer.TektonPipelineInformer
	// manifest has the source manifest of Tekton Triggers for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// version of triggers which we are installing
	triggersVersion string
}

// Check that our Reconciler implements controller.Reconciler
var _ tektontriggerreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tt *v1alpha1.TektonTrigger) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With("name", tt.GetName())
	tt.Status.InitializeConditions()
	tt.Status.SetVersion(r.triggersVersion)

	if tt.GetName() != v1alpha1.TriggerResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.TriggerResourceName,
			tt.GetName(),
		)
		logger.Error(msg)
		tt.Status.MarkNotReady(msg)
		return nil
	}

	if err := r.targetNamespaceCheck(ctx, tt); err != nil {
		return err
	}

	//Make sure TektonPipeline is installed before proceeding with
	//TektonTrigger
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			tt.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		tt.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}
	tt.Status.MarkDependenciesInstalled()

	// Pass the object through defaulting
	tt.SetDefaults(ctx)

	if err := r.installerSetClient.RemoveObsoleteSets(ctx); err != nil {
		logger.Error("failed to remove obsolete installer sets: %v", err)
		return err
	}

	if err := r.extension.PreReconcile(ctx, tt); err != nil {
		msg := fmt.Sprintf("PreReconciliation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		tt.Status.MarkPreReconcilerFailed(msg)
		return nil
	}

	//Mark PreReconcile Complete
	tt.Status.MarkPreReconcilerComplete()

	if err := r.installerSetClient.MainSet(ctx, tt, &r.manifest, filterAndTransform(r.extension)); err != nil {
		msg := fmt.Sprintf("Main Reconcilation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		tt.Status.MarkInstallerSetNotReady(msg)
		return nil
	}

	if err := r.extension.PostReconcile(ctx, tt); err != nil {
		msg := fmt.Sprintf("PostReconciliation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		tt.Status.MarkPostReconcilerFailed(msg)
		return nil
	}

	// Mark PostReconcile Complete
	tt.Status.MarkPostReconcilerComplete()
	return nil
}

func (r *Reconciler) targetNamespaceCheck(ctx context.Context, comp v1alpha1.TektonComponent) error {
	ns, err := r.kubeClientSet.CoreV1().Namespaces().Get(ctx, comp.GetSpec().GetTargetNamespace(), metav1.GetOptions{})
	if err != nil {
		// if namespace is not there then return wait for it
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	// if the namespace is in deletion state then delete the installerset
	// and create later otherwise it will keep doing api calls to create resources
	// and keep failing
	if ns.DeletionTimestamp != nil {
		if err := r.installerSetClient.CleanupMainSet(ctx); err != nil {
			return err
		}
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	return err
}
