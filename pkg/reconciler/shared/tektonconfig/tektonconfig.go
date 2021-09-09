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

package tektonconfig

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonConfigreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/pipeline"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/trigger"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonConfig resources.
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
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonConfigreconciler.Interface = (*Reconciler)(nil)
var _ tektonConfigreconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a TektonConfig.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonConfig) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// List all TektonConfigs to determine if cluster-scoped resources should be deleted.
	tps, err := r.operatorClientSet.OperatorV1alpha1().TektonConfigs().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list all TektonConfigs: %w", err)
	}

	for _, tp := range tps.Items {
		if tp.GetDeletionTimestamp().IsZero() {
			// Not deleting all TektonPipelines. Nothing to do here.
			return nil
		}
	}

	if original.Spec.Profile == common.ProfileLite {
		return pipeline.TektonPipelineCRDelete(r.operatorClientSet.OperatorV1alpha1().TektonPipelines(), common.PipelineResourceName)
	} else {
		// TektonPipeline and TektonTrigger is common for profile type basic and all
		if err := pipeline.TektonPipelineCRDelete(r.operatorClientSet.OperatorV1alpha1().TektonPipelines(), common.PipelineResourceName); err != nil {
			return err
		}
		if err := trigger.TektonTriggerCRDelete(r.operatorClientSet.OperatorV1alpha1().TektonTriggers(), common.TriggerResourceName); err != nil {
			return err
		}
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tc *v1alpha1.TektonConfig) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tc.Status.InitializeConditions()
	tc.Status.ObservedGeneration = tc.Generation

	logger.Infow("Reconciling TektonConfig", "status", tc.Status)
	if tc.GetName() != common.ConfigResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			common.ConfigResourceName,
			tc.GetName(),
		)
		logger.Error(msg)
		tc.GetStatus().MarkInstallFailed(msg)
		return nil
	}

	if err := r.extension.PreReconcile(ctx, tc); err != nil {
		// If prereconcile updates the TektonConfig CR, it returns an error
		// to reconcile
		if err.Error() == "reconcile" {
			return err
		}
		tc.GetStatus().MarkInstallFailed(err.Error())
		return err
	}

	var stages common.Stages
	if tc.Spec.Profile == common.ProfileLite {
		err := trigger.TektonTriggerCRDelete(r.operatorClientSet.OperatorV1alpha1().TektonTriggers(), common.TriggerResourceName)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		stages = common.Stages{
			r.createPipelineCR,
		}
	} else {
		// TektonPipeline and TektonTrigger is common for profile type basic and all
		stages = common.Stages{
			r.createPipelineCR,
			r.createTriggerCR,
		}
	}

	manifest := r.manifest.Append()
	if err := stages.Execute(ctx, &manifest, tc); err != nil {
		tc.GetStatus().MarkInstallFailed(err.Error())
		return err
	}
	if err := r.extension.PostReconcile(ctx, tc); err != nil {
		tc.GetStatus().MarkInstallFailed(err.Error())
		return err
	}

	if err := common.Prune(r.kubeClientSet, ctx, tc); err != nil {
		logger.Error(err)
	}

	tc.Status.MarkInstallSucceeded()
	tc.Status.MarkDeploymentsAvailable()
	return nil
}

func (r *Reconciler) createPipelineCR(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	return pipeline.CreatePipelineCR(comp, r.operatorClientSet.OperatorV1alpha1())
}

func (r *Reconciler) createTriggerCR(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	return trigger.CreateTriggerCR(comp, r.operatorClientSet.OperatorV1alpha1())
}
