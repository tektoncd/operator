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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
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
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonaddonreconciler.Interface = (*Reconciler)(nil)
var _ tektonaddonreconciler.Finalizer = (*Reconciler)(nil)

var communityResourceURLs = []string{
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/jib-maven/0.1/jib-maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/maven/0.1/maven.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/tkn/0.1/tkn.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-source/0.1/helm-upgrade-from-source.yaml",
	"https://raw.githubusercontent.com/tektoncd/catalog/master/task/helm-upgrade-from-repo/0.1/helm-upgrade-from-repo.yaml",
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
		r.appendTarget,
		r.transform,
		common.Install,
		common.CheckDeployments,
	}
	manifest := r.manifest.Append()
	return stages.Execute(ctx, &manifest, tt)
}

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) appendTarget(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	if runtime.GOARCH == "amd64" {
		if err := applyCommunityResources(manifest); err != nil {
			return err
		}
	}
	return applyAddons(manifest)
}

func applyAddons(manifest *mf.Manifest) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	addonLocation := filepath.Join(koDataDir, "tekton-addon")
	var files []string
	if err := filepath.Walk(addonLocation, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	}); err != nil {
		return err
	}
	for i := range files {
		m, err := common.Fetch(files[i])
		if err != nil {
			return err
		}
		*manifest = manifest.Append(m)
	}
	return nil
}

func applyCommunityResources(manifest *mf.Manifest) error {
	urls := strings.Join(communityResourceURLs, ",")
	m, err := mf.ManifestFrom(mf.Path(urls))
	if err != nil {
		return err
	}
	*manifest = manifest.Append(m)
	return nil
}

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	instance := comp.(*v1alpha1.TektonAddon)
	extra := []mf.Transformer{
		replaceKind("Task", "ClusterTask"),
	}
	extra = append(extra, r.extension.Transformers(instance)...)
	return common.Transform(ctx, manifest, instance, extra...)
}

func (r *Reconciler) installed(ctx context.Context, instance v1alpha1.TektonComponent) (*mf.Manifest, error) {
	// Create new, empty manifest with valid client and logger
	installed := r.manifest.Append()
	// TODO: add ingress, etc
	stages := common.Stages{common.AppendInstalled, r.transform}
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
