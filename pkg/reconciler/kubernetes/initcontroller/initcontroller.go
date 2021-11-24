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

package initcontroller

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"
)

type Controller struct {
	Manifest         *mf.Manifest
	Logger           *zap.SugaredLogger
	VersionConfigMap string
}

func (ctrl Controller) InitController(ctx context.Context) (mf.Manifest, string) {

	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		ctrl.Logger.Fatalw("Error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(ctrl.Logger.Named("manifestival").Desugar())

	manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
	if err != nil {
		ctrl.Logger.Fatalw("Error creating initial manifest", zap.Error(err))
	}

	ctrl.Manifest = &manifest
	if err := ctrl.fetchSourceManifests(ctx); err != nil {
		ctrl.Logger.Fatalw("failed to read manifest", err)
	}

	var releaseVersion string
	// Read the release version of component
	releaseVersion, err = common.FetchVersionFromConfigMap(manifest, ctrl.VersionConfigMap)
	if err != nil {
		if common.IsFetchVersionError(err) {
			ctrl.Logger.Warnf("failed to read version information from ConfigMap %s", ctrl.VersionConfigMap, err)
			releaseVersion = "Unknown"
		} else {
			ctrl.Logger.Fatalw("Error while reading ConfigMap", zap.Error(err))
		}
	}

	return manifest, releaseVersion
}

// fetchSourceManifests mutates the passed manifest by appending one
// appropriate for the passed TektonComponent
func (ctrl Controller) fetchSourceManifests(ctx context.Context) error {
	switch {
	case strings.Contains(ctrl.VersionConfigMap, "pipeline"):
		var pipeline *v1alpha1.TektonPipeline
		if err := common.AppendTarget(ctx, ctrl.Manifest, pipeline); err != nil {
			return err
		}
		// add proxy configs to pipeline if any
		return addProxy(ctrl.Manifest)
	case strings.Contains(ctrl.VersionConfigMap, "triggers"):
		var trigger *v1alpha1.TektonTrigger
		return common.AppendTarget(ctx, ctrl.Manifest, trigger)
	}

	return nil
}

func addProxy(manifest *mf.Manifest) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	proxyLocation := filepath.Join(koDataDir, "webhook")
	return common.AppendManifest(manifest, proxyLocation)
}
