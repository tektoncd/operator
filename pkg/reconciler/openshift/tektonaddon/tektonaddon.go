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
	tektonaddon "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonaddon/pipelinetemplates"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for Knativeserving resources.
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

	pipelineInformer informer.TektonPipelineInformer
	triggerInformer  informer.TektonTriggerInformer
	config           *rest.Config
}

const (
	retain int = iota
	overwrite

	labelProviderType     = "operator.tekton.dev/provider-type"
	providerTypeCommunity = "community"
	providerTypeRedHat    = "redhat"
	consoleYamlSamples    = "consoleyamlsamples.console.openshift.io"
	consoleQuickStart     = "consolequickstarts.console.openshift.io"
)

// Check that our Reconciler implements controller.Reconciler
var _ tektonaddonreconciler.Interface = (*Reconciler)(nil)
var _ tektonaddonreconciler.Finalizer = (*Reconciler)(nil)

var communityResourceURLs = []string{
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/jib-maven/0.2/jib-maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/maven/0.1/maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/tkn/0.1/tkn.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-source/0.2/helm-upgrade-from-source.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-repo/0.2/helm-upgrade-from-repo.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/trigger-jenkins-job/0.1/trigger-jenkins-job.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/git-cli/0.1/git-cli.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/pull-request/0.1/pull-request.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/kubeconfig-creator/0.1/kubeconfig-creator.yaml",
}

// FinalizeKind removes all resources after deletion of a TektonTriggers.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonAddon) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// List all TektonAddons to determine if cluster-scoped resources should be deleted.
	tps, err := r.operatorClientSet.OperatorV1alpha1().TektonAddons().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list all TektonAddons: %w", err)
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
	if err := common.Uninstall(ctx, manifest); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}
	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tt *v1alpha1.TektonAddon) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tt.Status.InitializeConditions()
	tt.Status.ObservedGeneration = tt.Generation

	logger.Infow("Reconciling TektonAddons", "status", tt.Status)

	if tt.GetName() != common.AddonResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			common.AddonResourceName,
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
	//find the valid tekton-trigger installation
	if _, err := common.TriggerReady(r.triggerInformer); err != nil {
		if err.Error() == common.TriggerNotReady {
			tt.Status.MarkDependencyInstalling("tekton-triggers is still installing")
			// wait for trigger status to change
			return fmt.Errorf(common.TriggerNotReady)
		}
		// (tektontriggers.opeator.tekton.dev instance not available yet)
		tt.Status.MarkDependencyMissing("tekton-triggers does not exist")
		return err
	}
	tt.Status.MarkDependenciesInstalled()

	if err := r.extension.PreReconcile(ctx, tt); err != nil {
		return err
	}
	stages := common.Stages{
		r.appendAddonTarget,
		r.addonTransform,
		common.Install,
		common.CheckDeployments,
	}
	manifest := r.manifest.Append()
	if err := stages.Execute(ctx, &manifest, tt); err != nil {
		return err
	}
	// Install addon for community tasks
	stages = common.Stages{
		r.appendCommunityTarget,
		r.communityTransform,
		common.Install,
		common.CheckDeployments,
	}
	manifest = r.manifest.Append()
	return stages.Execute(ctx, &manifest, tt)
}

// appendAddonTarget mutates the passed manifest by appending one
// appropriate for the passed TektonComponent
func (r *Reconciler) appendAddonTarget(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	if err := addPipelineTemplates(manifest); err != nil {
		return err
	}
	if err := applyAddons(manifest, comp); err != nil {
		return err
	}
	// add optionals to addons if any
	return applyOptionalAddons(ctx, manifest, comp, r.config)
}

func addPipelineTemplates(manifest *mf.Manifest) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	addonLocation := filepath.Join(koDataDir, "tekton-pipeline-template")
	return tektonaddon.GeneratePipelineTemplates(addonLocation, manifest)
}

func applyAddons(manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	addonLocation := filepath.Join(koDataDir, "tekton-addon/"+common.TargetVersion(comp)+"/addons")
	addons, err := mf.ManifestFrom(mf.Recursive(addonLocation))
	if err != nil {
		return err
	}
	if runtime.GOARCH == "ppc64le" || runtime.GOARCH == "s390x" {
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

func applyOptionalAddons(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent, cfg *rest.Config) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	consoleYamlCRDinstalled, err := isCRDExist(ctx, cfg, consoleYamlSamples)
	if err != nil {
		return err
	}
	if consoleYamlCRDinstalled {
		optionalLocation := filepath.Join(koDataDir, "tekton-addon/"+common.TargetVersion(comp)+"/optional/samples")
		if err := common.AppendManifest(manifest, optionalLocation); err != nil {
			return err
		}
	}
	consoleQuickStartCRDinstalled, err := isCRDExist(ctx, cfg, consoleQuickStart)
	if err != nil {
		return err
	}
	if consoleQuickStartCRDinstalled {
		optionalLocation := filepath.Join(koDataDir, "tekton-addon/"+common.TargetVersion(comp)+"/optional/quickstarts")
		return common.AppendManifest(manifest, optionalLocation)
	}
	return nil
}

func isCRDExist(ctx context.Context, config *rest.Config, crdName string) (bool, error) {
	apiextensionsclientset, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return false, err
	}
	_, err = apiextensionsclientset.ApiextensionsV1beta1().
		CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
	return err == nil, ignoreNotFound(err)
}

func ignoreNotFound(err error) error {
	if errors.IsNotFound(err) {
		return nil
	}
	return err
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

func (r *Reconciler) installed(ctx context.Context, instance v1alpha1.TektonComponent) (*mf.Manifest, error) {
	// Create new, empty manifest with valid client and logger
	installed := r.manifest.Append()
	// TODO: add ingress, etc
	stages := common.Stages{r.appendAddonTarget, r.addonTransform, r.appendCommunityTarget, r.communityTransform}
	err := stages.Execute(ctx, &installed, instance)
	return &installed, err
}

func replaceKind(fromKind, toKind string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := u.GetKind()
		if kind != fromKind {
			return nil
		}
		err := unstructured.SetNestedField(u.Object, toKind, "kind")
		if err != nil {
			return fmt.Errorf(
				"failed to change resource Name:%s, KIND from %s to %s, %s",
				u.GetName(),
				fromKind,
				toKind,
				err,
			)
		}
		return nil
	}
}

//injectLabel adds label key:value to a resource
// overwritePolicy (Retain/Overwrite) decides whehther to overwrite an already existing label
// []kinds specify the Kinds on which the label should be applied
// if len(kinds) = 0, label will be apllied to all/any resources irrespective of its Kind
func injectLabel(key, value string, overwritePolicy int, kinds ...string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := u.GetKind()
		if len(kinds) != 0 && !itemInSlice(kind, kinds) {
			return nil
		}
		labels, found, err := unstructured.NestedStringMap(u.Object, "metadata", "labels")
		if err != nil {
			return fmt.Errorf("could not find labels set, %q", err)
		}
		if overwritePolicy == retain && found {
			if _, ok := labels[key]; ok {
				return nil
			}
		}
		if !found {
			labels = map[string]string{}
		}
		labels[key] = value
		err = unstructured.SetNestedStringMap(u.Object, labels, "metadata", "labels")
		if err != nil {
			return fmt.Errorf("error updating labels for %s:%s, %s", kind, u.GetName(), err)
		}
		return nil
	}
}

func itemInSlice(item string, items []string) bool {
	for _, v := range items {
		if v == item {
			return true
		}
	}
	return false
}
