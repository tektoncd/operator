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

package tektonresult

import (
	"context"
	"os"
	"path/filepath"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"knative.dev/pkg/logging"
)

const (
	// manifests console plugin yaml directory location
	routeRBACYamlDirectory  = "static/tekton-results/route-rbac"
	internalDBYamlDirectory = "static/tekton-results/internal-db"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)

	version := os.Getenv(v1alpha1.VersionEnvKey)
	if version == "" {
		logger.Fatal("Failed to find version from env")
	}

	routeManifest, err := getRouteManifest()
	if err != nil {
		logger.Fatal("Failed to fetch route rbac static manifest")

	}

	internalDBManifest, err := getDBManifest()
	if err != nil {
		logger.Fatal("Failed to fetch internal db static manifest")

	}

	ext := openshiftExtension{
		installerSetClient: client.NewInstallerSetClient(operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets(),
			version, "results-ext", v1alpha1.KindTektonResult, nil),
		internalDBManifest: internalDBManifest,
		routeManifest:      routeManifest,
	}
	return ext
}

type openshiftExtension struct {
	installerSetClient *client.InstallerSetClient
	routeManifest      *mf.Manifest
	internalDBManifest *mf.Manifest
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{
		occommon.RemoveRunAsUser(),
		occommon.RemoveRunAsGroup(),
		occommon.ApplyCABundles,
	}
}

func (oe openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	result := tc.(*v1alpha1.TektonResult)

	mf := mf.Manifest{}
	if !result.Spec.IsExternalDB {
		mf = *oe.internalDBManifest
	}

	return oe.installerSetClient.PreSet(ctx, tc, &mf, filterAndTransform())
}

func (oe openshiftExtension) PostReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	mf := *oe.routeManifest
	return oe.installerSetClient.PostSet(ctx, tc, &mf, filterAndTransform())
}

func (oe openshiftExtension) Finalize(ctx context.Context, tc v1alpha1.TektonComponent) error {
	if err := oe.installerSetClient.CleanupPostSet(ctx); err != nil {
		return err
	}
	if err := oe.installerSetClient.CleanupPreSet(ctx); err != nil {
		return err
	}
	return nil
}

func getRouteManifest() (*mf.Manifest, error) {
	manifest := &mf.Manifest{}
	resultsRbac := filepath.Join(common.ComponentBaseDir(), routeRBACYamlDirectory)
	if err := common.AppendManifest(manifest, resultsRbac); err != nil {
		return nil, err
	}
	return manifest, nil
}

func getDBManifest() (*mf.Manifest, error) {
	manifest := &mf.Manifest{}
	internalDB := filepath.Join(common.ComponentBaseDir(), internalDBYamlDirectory)
	if err := common.AppendManifest(manifest, internalDB); err != nil {
		return nil, err
	}
	return manifest, nil
}

func filterAndTransform() client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		resultImgs := common.ToLowerCaseKeys(common.ImagesFromEnv(common.ResultsImagePrefix))

		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdResults),
			common.ApplyProxySettings,
			common.AddStatefulSetRestrictedPSA(),
			common.DeploymentImages(resultImgs),
			common.StatefulSetImages(resultImgs),
		}

		if err := common.Transform(ctx, manifest, comp, extra...); err != nil {
			return nil, err
		}
		return manifest, nil
	}
}
