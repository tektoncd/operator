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

package tektonaddon

import (
	"context"
	"fmt"
	"github.com/tektoncd/operator/pkg/reconciler/openshift"
	"os"
	"path/filepath"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	informer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektonaddonreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonaddon"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonAddon resources.
type Reconciler struct {
	manifest          mf.Manifest
	operatorClientSet clientset.Interface
	extension         common.Extension

	pipelineInformer informer.TektonPipelineInformer
	triggerInformer  informer.TektonTriggerInformer

	operatorVersion string
}

const (
	retain int = iota
	overwrite

	labelProviderType     = "operator.tekton.dev/provider-type"
	providerTypeCommunity = "community"
	providerTypeRedHat    = "redhat"
)

// Check that our Reconciler implements controller.Reconciler
var _ tektonaddonreconciler.Interface = (*Reconciler)(nil)
var _ tektonaddonreconciler.Finalizer = (*Reconciler)(nil)

var ls = metav1.LabelSelector{
	MatchLabels: map[string]string{
		v1alpha1.CreatedByKey: CreatedByValue,
	},
}

// FinalizeKind removes all resources after deletion of a TektonTriggers.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonAddon) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}
	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err != nil {
		logger.Error("Failed to delete installer set created by TektonAddon", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, ta *v1alpha1.TektonAddon) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	ta.Status.InitializeConditions()

	if ta.GetName() != v1alpha1.AddonResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.AddonResourceName,
			ta.GetName(),
		)
		logger.Error(msg)
		ta.Status.MarkNotReady(msg)
		return nil
	}

	// Pass the object through defaulting
	ta.SetDefaults(ctx)

	// Mark TektonAddon Instance as Not Ready if an upgrade is needed
	if err := r.markUpgrade(ctx, ta); err != nil {
		return err
	}

	// Make sure TektonPipeline & TektonTrigger is installed before proceeding with
	// TektonAddons

	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady {
			ta.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		ta.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}

	if _, err := common.TriggerReady(r.triggerInformer); err != nil {
		if err.Error() == common.TriggerNotReady {
			ta.Status.MarkDependencyInstalling("tekton-triggers is still installing")
			// wait for trigger status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektontrigger.operator.tekton.dev instance not available yet)
		ta.Status.MarkDependencyMissing("tekton-triggers does not exist")
		return err
	}

	ta.Status.MarkDependenciesInstalled()

	if err := tektoninstallerset.CleanUpObsoleteResources(ctx, r.operatorClientSet, CreatedByValue); err != nil {
		return err
	}

	// validate the params
	ptVal, _ := findValue(ta.Spec.Params, v1alpha1.PipelineTemplatesParam)
	ctVal, _ := findValue(ta.Spec.Params, v1alpha1.ClusterTasksParam)
	cctVal, _ := findValue(ta.Spec.Params, v1alpha1.CommunityClusterTasks)

	if ptVal == "true" && ctVal == "false" {
		ta.Status.MarkNotReady("pipelineTemplates cannot be true if clusterTask is false")
		return nil
	}

	if err := r.extension.PreReconcile(ctx, ta); err != nil {
		ta.Status.MarkPreReconcilerFailed(err.Error())
		return err
	}

	ta.Status.MarkPreReconcilerComplete()

	if err := r.EnsureClusterTask(ctx, ctVal, ta); err != nil {
		return err
	}

	if err := r.EnsureVersionedClusterTask(ctx, ctVal, ta); err != nil {
		return err
	}

	if err := r.EnsureCommunityClusterTask(ctx, cctVal, ta); err != nil {
		return err
	}

	if err := r.EnsurePipelineTemplates(ctx, ptVal, ta); err != nil {
		return err
	}

	if err := r.EnsureTriggersResources(ctx, ta); err != nil {
		return err
	}

	if err := r.EnsurePipelinesAsCode(ctx, ta); err != nil {
		return err
	}

	ta.Status.MarkInstallerSetReady()

	if err := r.extension.PostReconcile(ctx, ta); err != nil {
		if err == v1alpha1.RECONCILE_AGAIN_ERR {
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		ta.Status.MarkPostReconcilerFailed(err.Error())
		return err
	}

	ta.Status.MarkPostReconcilerComplete()

	ta.Status.SetVersion(r.operatorVersion)

	return nil
}

func applyAddons(manifest *mf.Manifest, subpath string) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	addonLocation := filepath.Join(koDataDir, "tekton-addon", "addons", subpath)
	return common.AppendManifest(manifest, addonLocation)
}

// addonTransform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) addonTransform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent, addnTfs ...mf.Transformer) error {
	instance := comp.(*v1alpha1.TektonAddon)
	addonTfs := []mf.Transformer{
		// using common.InjectOperandNameLabelPreserveExisting instead of common.InjectLabelOverwriteExisting
		// to highlight that TektonAddon is a basket of various operands(components)
		// note: using common.InjectLabelOverwriteExisting here  doesnot affect the ability to
		// use InjectOperandNameLabelPreserveExisting or InjectLabelOverwriteExisting again in the transformer chain
		// However, it is recomended to use InjectOperandNameLabelPreserveExisting here (in Addons) as we cannot be sure
		// about order of future addition of transformers in this reconciler or in sub functions which take care of various addons
		common.InjectOperandNameLabelPreserveExisting(openshift.OperandOpenShiftPipelinesAddons),
		injectLabel(labelProviderType, providerTypeRedHat, overwrite, "ClusterTask"),
	}
	addonTfs = append(addonTfs, addnTfs...)
	addonTfs = append(addonTfs, r.extension.Transformers(instance)...)
	return common.Transform(ctx, manifest, instance, addonTfs...)
}

func findValue(params []v1alpha1.Param, name string) (string, bool) {
	for _, p := range params {
		if p.Name == name {
			return p.Value, true
		}
	}
	return "", false
}

func (r *Reconciler) markUpgrade(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	labels := ta.GetLabels()
	ver, ok := labels[v1alpha1.ReleaseVersionKey]
	if ok && ver == r.operatorVersion {
		return nil
	}
	if ok && ver != r.operatorVersion {
		ta.Status.MarkInstallerSetNotReady(v1alpha1.UpgradePending)
		ta.Status.MarkPreReconcilerFailed(v1alpha1.UpgradePending)
		ta.Status.MarkPostReconcilerFailed(v1alpha1.UpgradePending)
		ta.Status.MarkNotReady(v1alpha1.UpgradePending)
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels[v1alpha1.ReleaseVersionKey] = r.operatorVersion
	ta.SetLabels(labels)

	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonAddons().Update(ctx,
		ta, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}
