/*
Copyright 2022 The Tekton Authors

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
	"os"
	"path/filepath"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) EnsureOpenShiftConsoleResources(ctx context.Context, ta *v1alpha1.TektonAddon) (error, bool) {
	filteredManifest := *r.openShiftConsoleManifest
	consoleYamlSampleExist, err := r.checkCRDExist(ctx, "consoleyamlsamples.console.openshift.io")
	if err != nil {
		return err, true
	}
	if !consoleYamlSampleExist {
		filteredManifest = filteredManifest.Filter(mf.Not(mf.ByKind("ConsoleYAMLSample")))
	}

	consoleQuickStartExist, err := r.checkCRDExist(ctx, "consolequickstarts.console.openshift.io")
	if err != nil {
		return err, true
	}
	if !consoleQuickStartExist {
		filteredManifest = filteredManifest.Filter(mf.Not(mf.ByKind("ConsoleQuickStart")))
	}

	consoleCLIDownloadExist, err := r.checkCRDExist(ctx, "consoleclidownloads.console.openshift.io")
	if err != nil {
		return err, true
	}
	if !consoleCLIDownloadExist {
		filteredManifest = filteredManifest.Filter(mf.Not(mf.Any(mf.ByKind("Deployment"), mf.ByKind("Service"), mf.ByKind("Route"))))
	}

	if len(filteredManifest.Resources()) == 0 {
		return nil, consoleCLIDownloadExist
	}
	if err := r.installerSetClient.CustomSet(ctx, ta, OpenShiftConsoleInstallerSet, &filteredManifest, filterAndTransformOCPResources(), nil); err != nil {
		return err, consoleCLIDownloadExist
	}
	return nil, consoleCLIDownloadExist
}

func (r *Reconciler) checkCRDExist(ctx context.Context, crdName string) (bool, error) {
	_, err := r.crdClientSet.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	}
	return true, nil
}

func filterAndTransformOCPResources() client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		addon := comp.(*v1alpha1.TektonAddon)
		imagesRaw := common.ToLowerCaseKeys(common.ImagesFromEnv(common.AddonsImagePrefix))
		images := common.ImageRegistryDomainOverride(imagesRaw)
		tfs := []mf.Transformer{
			common.DeploymentImages(images),
			common.AddConfiguration(addon.Spec.Config),
		}
		if err := transformers(ctx, manifest, addon, tfs...); err != nil {
			return nil, err
		}
		return manifest, nil
	}
}

func getOptionalAddons(manifest *mf.Manifest) error {
	koDataDir := os.Getenv(common.KoEnvKey)

	optionalLocation := filepath.Join(koDataDir, "tekton-addon", "optional", "samples")
	if err := common.AppendManifest(manifest, optionalLocation); err != nil {
		return err
	}

	optionalLocation = filepath.Join(koDataDir, "tekton-addon", "optional", "quickstarts")
	return common.AppendManifest(manifest, optionalLocation)
}
