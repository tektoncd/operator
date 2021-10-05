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
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	informer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektonaddonreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonaddon"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	tektonaddon "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonaddon/pipelinetemplates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
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

	version string
}

const (
	retain int = iota
	overwrite

	labelProviderType     = "operator.tekton.dev/provider-type"
	providerTypeCommunity = "community"
	providerTypeRedHat    = "redhat"
)

const (
	clusterTaskInstallerSet            = "ClusterTaskInstallerSet"
	pipelinesTemplateInstallerSet      = "PipelinesTemplateInstallerSet"
	triggersResourcesInstallerSet      = "TriggersResourcesInstallerSet"
	miscellaneousResourcesInstallerSet = "MiscellaneousResourcesInstallerSet"

	createdByKey       = "operator.tekton.dev/created-by"
	createdByValue     = "TektonAddon"
	releaseVersionKey  = "operator.tekton.dev/release-version"
	targetNamespaceKey = "operator.tekton.dev/target-namespace"
)

// Check that our Reconciler implements controller.Reconciler
var _ tektonaddonreconciler.Interface = (*Reconciler)(nil)
var _ tektonaddonreconciler.Finalizer = (*Reconciler)(nil)

var communityResourceURLs = []string{
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/jib-maven/0.4/jib-maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/maven/0.2/maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-source/0.3/helm-upgrade-from-source.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-repo/0.2/helm-upgrade-from-repo.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/trigger-jenkins-job/0.1/trigger-jenkins-job.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/git-cli/0.2/git-cli.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/pull-request/0.1/pull-request.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/kubeconfig-creator/0.1/kubeconfig-creator.yaml",
}

// FinalizeKind removes all resources after deletion of a TektonTriggers.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonAddon) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	installerSets := original.Status.AddonsInstallerSet
	if len(installerSets) == 0 {
		return nil
	}

	for _, value := range installerSets {
		err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Delete(ctx, value, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
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
func (r *Reconciler) ReconcileKind(ctx context.Context, ta *v1alpha1.TektonAddon) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	ta.Status.InitializeConditions()

	if ta.GetName() != common.AddonResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			common.AddonResourceName,
			ta.GetName(),
		)
		logger.Error(msg)
		ta.Status.MarkNotReady(msg)
		return nil
	}

	// Make sure TektonPipeline & TektonTrigger is installed before proceeding with
	// TektonAddons

	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady {
			ta.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return fmt.Errorf(common.PipelineNotReady)
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		ta.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}

	if _, err := common.TriggerReady(r.triggerInformer); err != nil {
		if err.Error() == common.TriggerNotReady {
			ta.Status.MarkDependencyInstalling("tekton-triggers is still installing")
			// wait for trigger status to change
			return fmt.Errorf(common.TriggerNotReady)
		}
		// (tektontrigger.operator.tekton.dev instance not available yet)
		ta.Status.MarkDependencyMissing("tekton-triggers does not exist")
		return err
	}

	ta.Status.MarkDependenciesInstalled()

	// Pass the object through defaulting
	ta.SetDefaults(ctx)

	// validate the params
	ptVal, _ := findValue(ta.Spec.Params, v1alpha1.PipelineTemplatesParam)
	ctVal, _ := findValue(ta.Spec.Params, v1alpha1.ClusterTasksParam)

	if ptVal == "true" && ctVal == "false" {
		ta.Status.MarkNotReady("pipelineTemplates cannot be true if clusterTask is false")
		return nil
	}

	if err := r.extension.PreReconcile(ctx, ta); err != nil {
		ta.Status.MarkPreReconcilerFailed(err.Error())
		return err
	}

	ta.Status.MarkPreReconcilerComplete()

	// If clusterTasks are enabled then create an InstallerSet
	// with their manifest
	if ctVal == "true" {

		exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.version, ta, clusterTaskInstallerSet)
		if err != nil {
			return err
		}

		if !exist {
			return r.ensureClusterTasks(ctx, ta)
		}
	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, ta, clusterTaskInstallerSet); err != nil {
			return err
		}
	}

	err := r.checkComponentStatus(ctx, ta, clusterTaskInstallerSet)
	if err != nil {
		ta.Status.MarkInstallerSetNotReady(err.Error())
		return nil
	}

	// If pipeline templates are enabled then create an InstallerSet
	// with their manifest
	if ptVal == "true" {

		exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.version, ta, pipelinesTemplateInstallerSet)
		if err != nil {
			return err
		}
		if !exist {
			return r.ensurePipelineTemplates(ctx, ta)
		}
	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, ta, pipelinesTemplateInstallerSet); err != nil {
			return err
		}
	}

	err = r.checkComponentStatus(ctx, ta, pipelinesTemplateInstallerSet)
	if err != nil {
		ta.Status.MarkInstallerSetNotReady(err.Error())
		return nil
	}

	// Ensure Triggers resources

	exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.version, ta, triggersResourcesInstallerSet)
	if err != nil {
		return err
	}
	if !exist {
		return r.ensureTriggerResources(ctx, ta)
	}

	err = r.checkComponentStatus(ctx, ta, triggersResourcesInstallerSet)
	if err != nil {
		ta.Status.MarkInstallerSetNotReady(err.Error())
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

func (r *Reconciler) checkComponentStatus(ctx context.Context, ta *v1alpha1.TektonAddon, component string) error {

	// Check if installer set is already created
	compInstallerSet, ok := ta.Status.AddonsInstallerSet[component]
	if !ok {
		return nil
	}

	if compInstallerSet != "" {

		ctIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, compInstallerSet, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		ready := ctIs.Status.GetCondition(apis.ConditionReady)
		if ready == nil || ready.Status == corev1.ConditionUnknown {
			return fmt.Errorf("InstallerSet %s: waiting for installation", ctIs.Name)
		} else if ready.Status == corev1.ConditionFalse {
			return fmt.Errorf("InstallerSet %s: ", ready.Message)
		}
	}

	return nil
}

func (r *Reconciler) ensureTriggerResources(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	triggerResourcesManifest := r.manifest

	if err := applyAddons(&triggerResourcesManifest, "01-clustertriggerbindings"); err != nil {
		return err
	}
	// Run transformers
	if err := r.addonTransform(ctx, &triggerResourcesManifest, ta); err != nil {
		return err
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, triggerResourcesManifest, r.version,
		triggersResourcesInstallerSet, "addon-triggers"); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) ensurePipelineTemplates(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	pipelineTemplateManifest := r.manifest

	// Read pipeline template manifest from kodata
	if err := applyAddons(&pipelineTemplateManifest, "03-pipelines"); err != nil {
		return err
	}

	// generate pipeline templates
	if err := addPipelineTemplates(&pipelineTemplateManifest); err != nil {
		return err
	}

	// Run transformers
	if err := r.addonTransform(ctx, &pipelineTemplateManifest, ta); err != nil {
		return err
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, pipelineTemplateManifest, r.version,
		pipelinesTemplateInstallerSet, "addon-pipelines"); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) ensureClusterTasks(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	clusterTaskManifest := r.manifest
	// Read clusterTasks from ko data
	if err := applyAddons(&clusterTaskManifest, "02-clustertasks"); err != nil {
		return err
	}
	// Run transformers
	if err := r.addonTransform(ctx, &clusterTaskManifest, ta); err != nil {
		return err
	}

	communityClusterTaskManifest := r.manifest

	if err := r.appendCommunityTarget(ctx, &communityClusterTaskManifest, ta); err != nil {
		return err
	}

	if err := r.communityTransform(ctx, &communityClusterTaskManifest, ta); err != nil {
		return err
	}

	clusterTaskManifest = clusterTaskManifest.Append(communityClusterTaskManifest)

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, clusterTaskManifest,
		r.version, clusterTaskInstallerSet, "addon-clustertasks"); err != nil {
		return err
	}

	return nil
}

// checkIfInstallerSetExist checks if installer set exists for a component and return true/false based on it
// and if installer set which already exist is of older version then it deletes and return false to create a new
// installer set
func checkIfInstallerSetExist(ctx context.Context, oc clientset.Interface, relVersion string,
	ta *v1alpha1.TektonAddon, component string) (bool, error) {

	// Check if installer set is already created
	compInstallerSet, ok := ta.Status.AddonsInstallerSet[component]
	if !ok {
		return false, nil
	}

	if compInstallerSet != "" {
		// if already created then check which version it is
		ctIs, err := oc.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, compInstallerSet, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		version, ok := ctIs.Annotations[releaseVersionKey]
		if ok && version == relVersion {
			// if installer set already exist and release version is same
			// then ignore and move on
			return true, nil
		}

		// release version doesn't exist or is different from expected
		// deleted existing InstallerSet and create a new one

		err = oc.OperatorV1alpha1().TektonInstallerSets().
			Delete(ctx, compInstallerSet, metav1.DeleteOptions{})
		if err != nil {
			return false, err
		}
	}

	return false, nil
}

func createInstallerSet(ctx context.Context, oc clientset.Interface, ta *v1alpha1.TektonAddon,
	manifest mf.Manifest, releaseVersion, component, installerSetPrefix string) error {

	is := makeInstallerSet(ta, manifest, installerSetPrefix, releaseVersion)

	createdIs, err := oc.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, is, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	if len(ta.Status.AddonsInstallerSet) == 0 {
		ta.Status.AddonsInstallerSet = map[string]string{}
	}

	// Update the status of addon with created installerSet name
	ta.Status.AddonsInstallerSet[component] = createdIs.Name
	ta.Status.SetVersion(releaseVersion)

	_, err = oc.OperatorV1alpha1().TektonAddons().
		UpdateStatus(ctx, ta, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func makeInstallerSet(ta *v1alpha1.TektonAddon, manifest mf.Manifest, prefix, releaseVersion string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(ta, ta.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", prefix),
			Labels: map[string]string{
				createdByKey: createdByValue,
			},
			Annotations: map[string]string{
				releaseVersionKey:  releaseVersion,
				targetNamespaceKey: ta.Spec.TargetNamespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}

func (r *Reconciler) deleteInstallerSet(ctx context.Context, ta *v1alpha1.TektonAddon, component string) error {

	compInstallerSet, ok := ta.Status.AddonsInstallerSet[component]
	if !ok {
		return nil
	}

	if compInstallerSet != "" {
		// delete the installer set
		err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Delete(ctx, ta.Status.AddonsInstallerSet[component], metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		// clear the name of installer set from TektonAddon status
		delete(ta.Status.AddonsInstallerSet, component)
		_, err = r.operatorClientSet.OperatorV1alpha1().TektonAddons().
			UpdateStatus(ctx, ta, metav1.UpdateOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func addPipelineTemplates(manifest *mf.Manifest) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	addonLocation := filepath.Join(koDataDir, "tekton-pipeline-template")
	return tektonaddon.GeneratePipelineTemplates(addonLocation, manifest)
}

func applyAddons(manifest *mf.Manifest, subpath string) error {
	comp := &v1alpha1.TektonAddon{}
	koDataDir := os.Getenv(common.KoEnvKey)
	addonLocation := filepath.Join(koDataDir, "tekton-addon/"+common.TargetVersion(comp)+"/addons/"+subpath)
	return common.AppendManifest(manifest, addonLocation)
}

// appendCommunityTarget mutates the passed manifest by appending one
// appropriate for the passed TektonComponent
func (r *Reconciler) appendCommunityTarget(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	urls := strings.Join(communityResourceURLs, ",")
	m, err := mf.ManifestFrom(mf.Path(urls))
	if err != nil {
		return err
	}
	*manifest = manifest.Append(m)
	return nil
}

// addonTransform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) addonTransform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	instance := comp.(*v1alpha1.TektonAddon)
	extra := []mf.Transformer{
		injectLabel(labelProviderType, providerTypeRedHat, overwrite, "ClusterTask"),
	}
	extra = append(extra, r.extension.Transformers(instance)...)
	return common.Transform(ctx, manifest, instance, extra...)
}

// communityTransform mutates the passed manifest to one with common component
// and platform transformations applied
func (r *Reconciler) communityTransform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	instance := comp.(*v1alpha1.TektonAddon)
	extra := []mf.Transformer{
		replaceKind("Task", "ClusterTask"),
		injectLabel(labelProviderType, providerTypeCommunity, overwrite, "ClusterTask"),
	}
	extra = append(extra, r.extension.Transformers(instance)...)
	return common.Transform(ctx, manifest, instance, extra...)
}

func findValue(params []v1alpha1.Param, name string) (string, bool) {
	for _, p := range params {
		if p.Name == name {
			return p.Value, true
		}
	}
	return "", false
}
