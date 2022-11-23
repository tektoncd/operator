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
	"os"
	"path/filepath"
	"runtime"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	informer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektonaddonreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonaddon"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonAddon resources.
type Reconciler struct {
	// installer Set client to do CRUD operations for components
	installerSetClient *client.InstallerSetClient
	// crdClientSet allows us to talk to the k8s for core APIs
	crdClientSet                 *apiextensionsclient.Clientset
	manifest                     mf.Manifest
	operatorClientSet            clientset.Interface
	extension                    common.Extension
	pipelineInformer             informer.TektonPipelineInformer
	triggerInformer              informer.TektonTriggerInformer
	operatorVersion              string
	clusterTaskManifest          *mf.Manifest
	triggersResourcesManifest    *mf.Manifest
	pipelineTemplateManifest     *mf.Manifest
	communityClusterTaskManifest *mf.Manifest
	openShiftConsoleManifest     *mf.Manifest
	consoleCLIManifest           *mf.Manifest
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

// FinalizeKind removes all resources after deletion of a TektonTriggers.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonAddon) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	if err := r.installerSetClient.CleanupAllCustomSet(ctx); err != nil {
		logger.Errorf("failed to cleanup custom set: %v", err)
		return err
	}
	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, ta *v1alpha1.TektonAddon) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	ta.Status.InitializeConditions()
	ta.Status.SetVersion(r.operatorVersion)

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

	// Make sure TektonPipeline & TektonTrigger is installed before proceeding with
	// TektonAddons

	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			ta.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		ta.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}

	if _, err := common.TriggerReady(r.triggerInformer); err != nil {
		if err.Error() == common.TriggerNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			ta.Status.MarkDependencyInstalling("tekton-triggers is still installing")
			// wait for trigger status to change
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		// (tektontrigger.operator.tekton.dev instance not available yet)
		ta.Status.MarkDependencyMissing("tekton-triggers does not exist")
		return err
	}

	ta.Status.MarkDependenciesInstalled()

	// validate the params
	ptVal, _ := findValue(ta.Spec.Params, v1alpha1.PipelineTemplatesParam)
	ctVal, _ := findValue(ta.Spec.Params, v1alpha1.ClusterTasksParam)
	cctVal, _ := findValue(ta.Spec.Params, v1alpha1.CommunityClusterTasks)

	if ptVal == "true" && ctVal == "false" {
		ta.Status.MarkNotReady("pipelineTemplates cannot be true if clusterTask is false")
		return nil
	}

	if err := r.installerSetClient.RemoveObsoleteSets(ctx); err != nil {
		return err
	}

	if err := r.extension.PreReconcile(ctx, ta); err != nil {
		ta.Status.MarkPreReconcilerFailed(err.Error())
		return err
	}

	ta.Status.MarkPreReconcilerComplete()

	// this to check if all sets are in ready set
	ready := true
	var errorMsg string

	if err := r.EnsureClusterTask(ctx, ctVal, ta); err != nil {
		ready = false
		errorMsg = fmt.Sprintf("cluster tasks not yet ready: %v", err)
		logger.Error(errorMsg)
	}

	if err := r.EnsureVersionedClusterTask(ctx, ctVal, ta); err != nil {
		ready = false
		errorMsg = fmt.Sprintf("versioned cluster tasks not yet ready:  %v", err)
		logger.Error(errorMsg)
	}

	if err := r.EnsureCommunityClusterTask(ctx, cctVal, ta); err != nil {
		ready = false
		errorMsg = fmt.Sprintf("community cluster tasks not yet ready:  %v", err)
		logger.Error(errorMsg)
	}

	if err := r.EnsurePipelineTemplates(ctx, ptVal, ta); err != nil {
		ready = false
		errorMsg = fmt.Sprintf("pipelines templates not yet ready:  %v", err)
		logger.Error(errorMsg)
	}

	if err := r.EnsureTriggersResources(ctx, ta); err != nil {
		ready = false
		errorMsg = fmt.Sprintf("triggers resources not yet ready:  %v", err)
		logger.Error(errorMsg)
	}

	err, consoleCLIDownloadExist := r.EnsureOpenShiftConsoleResources(ctx, ta)
	if err != nil {
		ready = false
		errorMsg = fmt.Sprintf("openshift console resources not yet ready:  %v", err)
		logger.Error(errorMsg)
	}

	if consoleCLIDownloadExist {
		if err := r.EnsureConsoleCLI(ctx, ta); err != nil {
			ready = false
			errorMsg = fmt.Sprintf("console cli not yet ready:  %v", err)
			logger.Error(errorMsg)
		}
	}

	if !ready {
		ta.Status.MarkInstallerSetNotReady(errorMsg)
		return nil
	}

	ta.Status.MarkInstallerSetReady()

	if err := r.extension.PostReconcile(ctx, ta); err != nil {
		ta.Status.MarkPostReconcilerFailed(err.Error())
		return err
	}

	ta.Status.MarkPostReconcilerComplete()
	return nil
}

func applyAddons(manifest *mf.Manifest, subpath string) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	addonLocation := filepath.Join(koDataDir, "tekton-addon", "addons", subpath)
	addons, err := mf.ManifestFrom(mf.Recursive(addonLocation))
	if err != nil {
		return err
	}
	// install knative addons only where knative is available
	switch runtime.GOARCH {
	case "amd64", "ppc64le", "s390x":
	default:
		version := common.TargetVersion((*v1alpha1.TektonPipeline)(nil))
		version_formated := strings.Replace(version, ".", "-", -1)
		addons = addons.Filter(
			mf.Not(mf.Any(
				mf.ByName("kn"),
				mf.ByName("kn-v"+version_formated),
				mf.ByName("kn-apply"),
				mf.ByName("kn-apply-v"+version_formated),
			)))
	}
	*manifest = manifest.Append(addons)
	return nil
}

func findValue(params []v1alpha1.Param, name string) (string, bool) {
	for _, p := range params {
		if p.Name == name {
			return p.Value, true
		}
	}
	return "", false
}
