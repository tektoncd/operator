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
	"os"
	"path/filepath"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)

	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
	if err != nil {
		logger.Fatalw("error creating initial manifest", zap.Error(err))
	}

	version := os.Getenv(versionKey)
	if version == "" {
		logger.Fatal("Failed to find version from env")
	}

	ext := openshiftExtension{
		operatorClientSet: operatorclient.Get(ctx),
		manifest:          manifest,
		version:           version,
	}
	return ext
}

type openshiftExtension struct {
	operatorClientSet versioned.Interface
	manifest          mf.Manifest
	version           string
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	addonImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.AddonsImagePrefix))
	return []mf.Transformer{
		common.TaskImages(addonImages),
	}
}
func (oe openshiftExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	addon := comp.(*v1alpha1.TektonAddon)

	exist, err := checkIfInstallerSetExist(ctx, oe.operatorClientSet, oe.version, addon, miscellaneousResourcesInstallerSet)
	if err != nil {
		return err
	}
	if !exist {

		if err := applyAddons(&oe.manifest, "04-consolecli"); err != nil {
			return err
		}

		if err := getOptionalAddons(&oe.manifest, comp); err != nil {
			return err
		}

		if err := addonTransform(ctx, &oe.manifest, addon); err != nil {
			return err
		}

		if err := createInstallerSet(ctx, oe.operatorClientSet, addon, oe.manifest, oe.version,
			miscellaneousResourcesInstallerSet, "addon-openshift"); err != nil {
			return err
		}
	}

	return nil
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func getOptionalAddons(manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	koDataDir := os.Getenv(common.KoEnvKey)

	optionalLocation := filepath.Join(koDataDir, "tekton-addon/"+common.TargetVersion(comp)+"/optional/samples")
	if err := common.AppendManifest(manifest, optionalLocation); err != nil {
		return err
	}

	optionalLocation = filepath.Join(koDataDir, "tekton-addon/"+common.TargetVersion(comp)+"/optional/quickstarts")
	return common.AppendManifest(manifest, optionalLocation)
}

func addonTransform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	return common.Transform(ctx, manifest, comp)
}
