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
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonConfigreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/pipeline"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/trigger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonConfig resources.
type Reconciler struct {
	// kubeClientSet allows us to talk to the k8s for core APIs
	kubeClientSet kubernetes.Interface
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// enqueueAfter enqueues a obj after a duration
	enqueueAfter    func(obj interface{}, after time.Duration)
	manifest        mf.Manifest
	operatorVersion string
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonConfigreconciler.Interface = (*Reconciler)(nil)
var _ tektonConfigreconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a TektonConfig.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonConfig) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	if original.Spec.Profile == v1alpha1.ProfileLite {
		return pipeline.TektonPipelineCRDelete(ctx, r.operatorClientSet.OperatorV1alpha1().TektonPipelines(), v1alpha1.PipelineResourceName)
	} else {
		// TektonPipeline and TektonTrigger is common for profile type basic and all
		if err := trigger.TektonTriggerCRDelete(ctx, r.operatorClientSet.OperatorV1alpha1().TektonTriggers(), v1alpha1.TriggerResourceName); err != nil {
			return err
		}
		if err := pipeline.TektonPipelineCRDelete(ctx, r.operatorClientSet.OperatorV1alpha1().TektonPipelines(), v1alpha1.PipelineResourceName); err != nil {
			return err
		}
	}

	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tc *v1alpha1.TektonConfig) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tc.Status.InitializeConditions()

	logger.Infow("Reconciling TektonConfig", "status", tc.Status)
	if tc.GetName() != v1alpha1.ConfigResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.ConfigResourceName,
			tc.GetName(),
		)
		logger.Error(msg)
		tc.Status.MarkNotReady(msg)
		return nil
	}

	tc.SetDefaults(ctx)
	// Mark TektonConfig Instance as Not Ready if an upgrade is needed
	if err := r.markUpgrade(ctx, tc); err != nil {
		return err
	}

	if err := r.ensureTargetNamespaceExists(ctx, tc); err != nil {
		return err
	}

	if err := r.createOperatorVersionConfigMap(tc); err != nil {
		return err
	}

	if err := r.extension.PreReconcile(ctx, tc); err != nil {
		if err == v1alpha1.RECONCILE_AGAIN_ERR {
			r.enqueueAfter(tc, 10*time.Second)
			return nil
		}
		tc.Status.MarkPreInstallFailed(err.Error())
		return err
	}

	tc.Status.MarkPreInstallComplete()

	// Create TektonPipeline CR
	if err := pipeline.CreatePipelineCR(ctx, tc, r.operatorClientSet.OperatorV1alpha1()); err != nil {
		tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonPipeline: %s", err.Error()))
		r.enqueueAfter(tc, 10*time.Second)
		return nil
	}

	// Create TektonTrigger CR if the profile is all or basic
	if tc.Spec.Profile == v1alpha1.ProfileAll || tc.Spec.Profile == v1alpha1.ProfileBasic {
		if err := trigger.CreateTriggerCR(ctx, tc, r.operatorClientSet.OperatorV1alpha1()); err != nil {
			tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonTrigger: %s", err.Error()))
			r.enqueueAfter(tc, 10*time.Second)
			return nil
		}
	} else {
		if err := trigger.TektonTriggerCRDelete(ctx, r.operatorClientSet.OperatorV1alpha1().TektonTriggers(), v1alpha1.TriggerResourceName); err != nil {
			tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonTrigger: %s", err.Error()))
			r.enqueueAfter(tc, 10*time.Second)
			return nil
		}
	}

	if err := common.Prune(ctx, r.kubeClientSet, tc); err != nil {
		tc.Status.MarkComponentNotReady(fmt.Sprintf("tekton-resource-pruner: %s", err.Error()))
		logger.Error(err)
	}

	tc.Status.MarkComponentsReady()

	if err := r.extension.PostReconcile(ctx, tc); err != nil {
		tc.Status.MarkPostInstallFailed(err.Error())
		r.enqueueAfter(tc, 10*time.Second)
		return nil
	}

	tc.Status.MarkPostInstallComplete()

	if err := r.deleteObsoleteTargetNamespaces(ctx, tc); err != nil {
		logger.Error(err)
	}

	// Update the object for any spec changes
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonConfigs().Update(ctx, tc, v1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) createOperatorVersionConfigMap(tc *v1alpha1.TektonConfig) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	operatorDir := filepath.Join(koDataDir, "info")

	if err := common.AppendManifest(&r.manifest, operatorDir); err != nil {
		return err
	}

	manifest, err := r.manifest.Transform(
		mf.InjectNamespace(tc.GetSpec().GetTargetNamespace()),
		mf.InjectOwner(tc),
	)
	if err != nil {
		return err
	}

	if err = manifest.Apply(); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) ensureTargetNamespaceExists(ctx context.Context, tc *v1alpha1.TektonConfig) error {

	ns, err := r.kubeClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("operator.tekton.dev/targetNamespace=%s", "true"),
	})

	if err != nil {
		return err
	}

	if len(ns.Items) > 0 {
		for _, namespace := range ns.Items {
			if namespace.Name != tc.GetSpec().GetTargetNamespace() {
				namespace.Labels["operator.tekton.dev/targetNamespace/mark-for-deletion"] = "true"
				_, err = r.kubeClientSet.CoreV1().Namespaces().Update(ctx, &namespace, metav1.UpdateOptions{})
				if err != nil {
					return err
				}

			} else {
				return nil
			}
		}
	} else {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: tc.GetSpec().GetTargetNamespace(),
				Labels: map[string]string{
					"operator.tekton.dev/targetNamespace": "true",
				},
			},
		}
		if _, err = r.kubeClientSet.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
			if errors.IsAlreadyExists(err) {
				return r.addTargetNamespaceLabel(ctx, tc.GetSpec().GetTargetNamespace())
			}
			return err
		}
	}
	return nil
}

func (r *Reconciler) deleteObsoleteTargetNamespaces(ctx context.Context, tc *v1alpha1.TektonConfig) error {

	ns, err := r.kubeClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("operator.tekton.dev/targetNamespace/mark-for-deletion=%s", "true"),
	})

	if err != nil {
		return err
	}

	for _, namespace := range ns.Items {
		if namespace.Name != tc.GetSpec().GetTargetNamespace() {
			if err := r.kubeClientSet.CoreV1().Namespaces().Delete(ctx, tc.GetSpec().GetTargetNamespace(), metav1.DeleteOptions{}); err != nil {
				return err
			}
		} else {
			return nil
		}
	}

	return nil
}

func (r *Reconciler) markUpgrade(ctx context.Context, tc *v1alpha1.TektonConfig) error {
	labels := tc.GetLabels()
	ver, ok := labels[v1alpha1.ReleaseVersionKey]
	if ok && ver == r.operatorVersion {
		return nil
	}
	if ok && ver != r.operatorVersion {
		tc.Status.MarkComponentNotReady("Upgrade Pending")
		tc.Status.MarkPreInstallFailed(v1alpha1.UpgradePending)
		tc.Status.MarkPostInstallFailed(v1alpha1.UpgradePending)
		tc.Status.MarkNotReady("Upgrade Pending")
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels[v1alpha1.ReleaseVersionKey] = r.operatorVersion
	tc.SetLabels(labels)
	// Update the object for any spec changes
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonConfigs().Update(ctx,
		tc, v1.UpdateOptions{}); err != nil {
		return err
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}

func (r *Reconciler) addTargetNamespaceLabel(ctx context.Context, targetNamespace string) error {
	ns, err := r.kubeClientSet.CoreV1().Namespaces().Get(ctx, targetNamespace, v1.GetOptions{})
	if err != nil {
		return err
	}
	labels := ns.GetLabels()
	if labels == nil {
		labels = map[string]string{
			"operator.tekton.dev/targetNamespace": "true",
		}
	}
	ns.SetLabels(labels)
	_, err = r.kubeClientSet.CoreV1().Namespaces().Update(ctx, ns, v1.UpdateOptions{})
	return err
}
