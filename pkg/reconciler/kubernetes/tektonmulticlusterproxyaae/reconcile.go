/*
Copyright 2026 The Tekton Authors

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

package tektonmulticlusterproxyaae

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	proxyAAEreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonmulticlusterproxyaae"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonMulticlusterProxyAAE resources.
type Reconciler struct {
	operatorClientSet           operatorclient.Interface
	kubeClientSet               kubernetes.Interface
	installerSetClient          *client.InstallerSetClient
	pipelineInformer            pipelineinformer.TektonPipelineInformer
	manifest                    mf.Manifest
	extension                   common.Extension
	multiclusterProxyAAEVersion string
	operatorVersion             string
}

// Check that our Reconciler implements controller.Reconciler
var _ proxyAAEreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind compares the actual state with the desired, and attempts to converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, proxy *v1alpha1.TektonMulticlusterProxyAAE) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With("name", proxy.GetName())
	proxy.Status.InitializeConditions()
	proxy.Status.SetVersion(r.multiclusterProxyAAEVersion)

	if proxy.GetName() != v1alpha1.MultiClusterProxyAAEResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.MultiClusterProxyAAEResourceName,
			proxy.GetName(),
		)
		logger.Error(msg)
		proxy.Status.MarkNotReady(msg)
		return nil
	}

	if err := common.ReconcileTargetNamespace(ctx, nil, nil, proxy, r.kubeClientSet); err != nil {
		return err
	}

	// Make sure TektonPipeline is installed before proceeding with TektonMulticlusterProxyAAE.
	if err := r.ensureDependenciesInstalled(proxy); err != nil {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	proxy.Status.MarkDependenciesInstalled()

	if err := r.installerSetClient.RemoveObsoleteSets(ctx); err != nil {
		logger.Error("failed to remove obsolete installer sets: %v", err)
		return err
	}

	if err := r.extension.PreReconcile(ctx, proxy); err != nil {
		msg := fmt.Sprintf("PreReconciliation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		proxy.Status.MarkPreReconcilerFailed(msg)
		return nil
	}
	proxy.Status.MarkPreReconcilerComplete()

	if err := r.installerSetClient.MainSet(ctx, proxy, &r.manifest, filterAndTransform(r.extension)); err != nil {
		msg := fmt.Sprintf("Main reconciliation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		proxy.Status.MarkInstallerSetNotReady(msg)
		return err
	}

	if err := r.extension.PostReconcile(ctx, proxy); err != nil {
		msg := fmt.Sprintf("PostReconciliation failed: %s", err.Error())
		logger.Error(msg)
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			return err
		}
		proxy.Status.MarkPostReconcilerFailed(msg)
		return nil
	}
	proxy.Status.MarkPostReconcilerComplete()
	return nil
}

// ensureDependenciesInstalled ensures TektonPipeline is installed and ready before proceeding.
func (r *Reconciler) ensureDependenciesInstalled(proxy *v1alpha1.TektonMulticlusterProxyAAE) error {
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			proxy.Status.MarkDependencyInstalling("Waiting for TektonPipeline 'pipeline' to become ready")
			// wait for pipeline status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		proxy.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}
	return nil
}
