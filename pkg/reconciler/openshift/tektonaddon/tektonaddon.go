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
	"time"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	informer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektonaddonreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonaddon"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	tektonaddon "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonaddon/pipelinetemplates"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonAddon resources.
type Reconciler struct {
	manifest          mf.Manifest
	operatorClientSet clientset.Interface
	extension         common.Extension

	// enqueueAfter enqueues a obj after a duration
	enqueueAfter func(obj interface{}, after time.Duration)

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

// Check that our Reconciler implements controller.Reconciler
var _ tektonaddonreconciler.Interface = (*Reconciler)(nil)
var _ tektonaddonreconciler.Finalizer = (*Reconciler)(nil)

var communityResourceURLs = []string{
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/jib-maven/0.4/jib-maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/maven/0.2/maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-source/0.3/helm-upgrade-from-source.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-repo/0.2/helm-upgrade-from-repo.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/trigger-jenkins-job/0.1/trigger-jenkins-job.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/git-cli/0.3/git-cli.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/pull-request/0.1/pull-request.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/kubeconfig-creator/0.1/kubeconfig-creator.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/main/task/argocd-task-sync-and-wait/0.1/argocd-task-sync-and-wait.yaml",
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

	if ta.GetName() != v1alpha1.AddonResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.AddonResourceName,
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

	if err := tektoninstallerset.CleanUpObsoleteResources(ctx, r.operatorClientSet, CreatedByValue); err != nil {
		return err
	}

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
	clusterTaskLS := metav1.LabelSelector{
		MatchLabels: map[string]string{
			tektoninstallerset.InstallerSetType: ClusterTaskInstallerSet,
		},
	}
	clusterTaskLabelSelector, err := common.LabelSelector(clusterTaskLS)
	if err != nil {
		return err
	}

	if ctVal == "true" {

		exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.version, clusterTaskLabelSelector)
		if err != nil {
			return err
		}

		if !exist {
			return r.ensureClusterTasks(ctx, ta)
		}
	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, clusterTaskLabelSelector); err != nil {
			return err
		}
	}

	if err := r.checkComponentStatus(ctx, clusterTaskLabelSelector); err != nil {
		ta.Status.MarkInstallerSetNotReady(err.Error())
		return nil
	}

	// If clusterTasks are enabled then create an InstallerSet
	// with the versioned clustertask manifest
	versionedClusterTaskLS := metav1.LabelSelector{
		MatchLabels: map[string]string{
			tektoninstallerset.InstallerSetType:       VersionedClusterTaskInstallerSet,
			tektoninstallerset.ReleaseMinorVersionKey: getPatchVersionTrimmed(r.version),
		},
	}
	versionedClusterTaskLabelSelector, err := common.LabelSelector(versionedClusterTaskLS)
	if err != nil {
		return err
	}
	if ctVal == "true" {

		// here pass two labels one for type and other for minor release version to remove the previous minor release installerset only not all
		exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.version, versionedClusterTaskLabelSelector)
		if err != nil {
			return err
		}

		if !exist {
			return r.ensureVersionedClusterTasks(ctx, ta)
		}
	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, versionedClusterTaskLabelSelector); err != nil {
			return err
		}
	}

	// here pass two labels one for type and other for operator release version to get the latest installerset of current version
	vClusterTaskLS := metav1.LabelSelector{
		MatchLabels: map[string]string{
			tektoninstallerset.InstallerSetType:  VersionedClusterTaskInstallerSet,
			tektoninstallerset.ReleaseVersionKey: r.version,
		},
	}
	vClusterTaskLabelSelector, err := common.LabelSelector(vClusterTaskLS)
	if err != nil {
		return err
	}
	if err := r.checkComponentStatus(ctx, vClusterTaskLabelSelector); err != nil {
		ta.Status.MarkInstallerSetNotReady(err.Error())
		return nil
	}

	// If pipeline templates are enabled then create an InstallerSet
	// with their manifest
	pipelineTemplateLS := metav1.LabelSelector{
		MatchLabels: map[string]string{
			tektoninstallerset.InstallerSetType: PipelinesTemplateInstallerSet,
		},
	}
	pipelineTemplateLSLabelSelector, err := common.LabelSelector(pipelineTemplateLS)
	if err != nil {
		return err
	}
	if ptVal == "true" {

		exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.version, pipelineTemplateLSLabelSelector)
		if err != nil {
			return err
		}
		if !exist {
			return r.ensurePipelineTemplates(ctx, ta)
		}
	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, pipelineTemplateLSLabelSelector); err != nil {
			return err
		}
	}

	if err := r.checkComponentStatus(ctx, pipelineTemplateLSLabelSelector); err != nil {
		ta.Status.MarkInstallerSetNotReady(err.Error())
		return nil
	}

	// Ensure Triggers resources
	triggerResourceLS := metav1.LabelSelector{
		MatchLabels: map[string]string{
			tektoninstallerset.InstallerSetType: TriggersResourcesInstallerSet,
		},
	}
	triggerResourceLabelSelector, err := common.LabelSelector(triggerResourceLS)
	if err != nil {
		return err
	}
	exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.version, triggerResourceLabelSelector)
	if err != nil {
		return err
	}
	if !exist {
		return r.ensureTriggerResources(ctx, ta)
	}

	err = r.checkComponentStatus(ctx, triggerResourceLabelSelector)
	if err != nil {
		ta.Status.MarkInstallerSetNotReady(err.Error())
		return nil
	}

	ta.Status.MarkInstallerSetReady()

	if err := r.extension.PostReconcile(ctx, ta); err != nil {
		if err == v1alpha1.RECONCILE_AGAIN_ERR {
			r.enqueueAfter(ta, 10*time.Second)
			return nil
		}
		ta.Status.MarkPostReconcilerFailed(err.Error())
		return err
	}

	ta.Status.MarkPostReconcilerComplete()

	ta.Status.SetVersion(r.version)

	return nil
}

func (r *Reconciler) checkComponentStatus(ctx context.Context, labelSelector string) error {

	// Check if installer set is already created
	installerSets, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})

	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// To make sure there won't be duplicate installersets.
	if len(installerSets.Items) == 1 {
		ready := installerSets.Items[0].Status.GetCondition(apis.ConditionReady)
		if ready == nil || ready.Status == corev1.ConditionUnknown {
			return fmt.Errorf("InstallerSet %s: waiting for installation", installerSets.Items[0].Name)
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
		TriggersResourcesInstallerSet, "addon-triggers"); err != nil {
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
		PipelinesTemplateInstallerSet, "addon-pipelines"); err != nil {
		return err
	}

	return nil
}

// installerset for non versioned clustertask like buildah and community clustertask
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

	clusterTaskManifest = clusterTaskManifest.Filter(
		mf.Not(byContains(getFormattedVersion(r.version))),
	)

	communityClusterTaskManifest := r.manifest
	if err := r.appendCommunityTarget(ctx, &communityClusterTaskManifest, ta); err != nil {
		// Continue if failed to resolve community task URL.
		// (Ex: on disconnected cluster community tasks won't be reachable because of proxy).
		logging.FromContext(ctx).Error("Failed to get community task: Skipping community tasks installation  ", err)
	} else {
		if err := r.communityTransform(ctx, &communityClusterTaskManifest, ta); err != nil {
			return err
		}

		clusterTaskManifest = clusterTaskManifest.Append(communityClusterTaskManifest)
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, clusterTaskManifest,
		r.version, ClusterTaskInstallerSet, "addon-clustertasks"); err != nil {
		return err
	}

	return nil
}

// installerset for versioned clustertask like buildah-1-6-0
func (r *Reconciler) ensureVersionedClusterTasks(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	clusterTaskManifest := r.manifest
	// Read clusterTasks from ko data
	if err := applyAddons(&clusterTaskManifest, "02-clustertasks"); err != nil {
		return err
	}
	// Run transformers
	if err := r.addonTransform(ctx, &clusterTaskManifest, ta); err != nil {
		return err
	}

	clusterTaskManifest = clusterTaskManifest.Filter(
		byContains(getFormattedVersion(r.version)),
	)

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, clusterTaskManifest,
		r.version, VersionedClusterTaskInstallerSet, "addon-versioned-clustertasks"); err != nil {
		return err
	}

	return nil
}

// checkIfInstallerSetExist checks if installer set exists for a component and return true/false based on it
// and if installer set which already exist is of older version then it deletes and return false to create a new
// installer set
func checkIfInstallerSetExist(ctx context.Context, oc clientset.Interface, relVersion string,
	labelSelector string) (bool, error) {

	installerSets, err := oc.OperatorV1alpha1().TektonInstallerSets().
		List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if len(installerSets.Items) == 1 {
		// if already created then check which version it is
		version, ok := installerSets.Items[0].Labels[tektoninstallerset.ReleaseVersionKey]
		if ok && version == relVersion {
			// if installer set already exist and release version is same
			// then ignore and move on
			return true, nil
		}
	}

	// release version doesn't exist or is different from expected
	// deleted existing InstallerSet and create a new one
	// or there is more than one installerset (unexpected)
	if err = oc.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err != nil {
		return false, err
	}

	return false, nil
}

func createInstallerSet(ctx context.Context, oc clientset.Interface, ta *v1alpha1.TektonAddon,
	manifest mf.Manifest, releaseVersion, component, installerSetPrefix string) error {

	specHash, err := hash.Compute(ta.Spec)
	if err != nil {
		return err
	}

	is := makeInstallerSet(ta, manifest, installerSetPrefix, releaseVersion, component, specHash)

	if _, err := oc.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, is, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func makeInstallerSet(ta *v1alpha1.TektonAddon, manifest mf.Manifest, prefix, releaseVersion, component, specHash string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(ta, ta.GetGroupVersionKind())
	labels := map[string]string{
		tektoninstallerset.CreatedByKey:      CreatedByValue,
		tektoninstallerset.InstallerSetType:  component,
		tektoninstallerset.ReleaseVersionKey: releaseVersion,
	}
	// special label to make sure no two versioned clustertask installerset exist
	// for all patch releases
	if component == VersionedClusterTaskInstallerSet {
		labels[tektoninstallerset.ReleaseMinorVersionKey] = getPatchVersionTrimmed(releaseVersion)
	}
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", prefix),
			Labels:       labels,
			Annotations: map[string]string{
				tektoninstallerset.TargetNamespaceKey: ta.Spec.TargetNamespace,
				tektoninstallerset.LastAppliedHashKey: specHash,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}

func (r *Reconciler) deleteInstallerSet(ctx context.Context, labelSelector string) error {

	err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func addPipelineTemplates(manifest *mf.Manifest) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	addonLocation := filepath.Join(koDataDir, "tekton-addon", "tekton-pipeline-template")
	return tektonaddon.GeneratePipelineTemplates(addonLocation, manifest)
}

func applyAddons(manifest *mf.Manifest, subpath string) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	addonLocation := filepath.Join(koDataDir, "tekton-addon", "addons", subpath)
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

// byContains returns resources with specific string in name
func byContains(name string) mf.Predicate {
	return func(u *unstructured.Unstructured) bool {
		return strings.Contains(u.GetName(), name)
	}
}

// To get the version in the format as in clustertask name i.e. 1-6
func getFormattedVersion(version string) string {
	version = strings.TrimPrefix(getPatchVersionTrimmed(version), "v")
	return strings.Replace(version, ".", "-", -1)
}

// To get the minor major version for label i.e. v1.6
func getPatchVersionTrimmed(version string) string {
	endIndex := strings.LastIndex(version, ".")
	if endIndex != -1 {
		version = version[:endIndex]
	}
	return version
}
