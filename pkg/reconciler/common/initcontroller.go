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

package common

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const ReleaseVersionUnknown = "Unknown"

type Controller struct {
	Manifest         *mf.Manifest
	Logger           *zap.SugaredLogger
	VersionConfigMap string
}

type PayloadOptions struct {
	ReadOnly bool
}

func OperatorVersion(ctx context.Context) (string, error) {
	logger := logging.FromContext(ctx)
	operatorVersion, ok := os.LookupEnv(v1alpha1.VersionEnvKey)
	if !ok || operatorVersion == "" {
		logger.Errorf(v1alpha1.VERSION_ENV_NOT_SET_ERR.Error())
		return "", v1alpha1.VERSION_ENV_NOT_SET_ERR
	}
	return operatorVersion, nil
}

func (ctrl Controller) InitController(ctx context.Context, opts PayloadOptions) (mf.Manifest, string) {

	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		ctrl.Logger.Fatalw("Error creating client from injected config", zap.Error(err))
	}

	manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient))
	if err != nil {
		ctrl.Logger.Fatalw("Error creating initial manifest", zap.Error(err))
	}

	ctrl.Manifest = &manifest
	if err := ctrl.fetchSourceManifests(ctx, opts); err != nil {
		ctrl.Logger.Fatalw("failed to read manifest", err)
	}

	var releaseVersion string
	// Read the release version of component
	releaseVersion, err = FetchVersionFromConfigMap(manifest, ctrl.VersionConfigMap)
	if err != nil {
		if IsFetchVersionError(err) {
			ctrl.Logger.Warnf("failed to read version information from ConfigMap %s: %v", ctrl.VersionConfigMap, err)
			releaseVersion = ReleaseVersionUnknown
		} else {
			ctrl.Logger.Fatalw("Error while reading ConfigMap", zap.Error(err))
		}
	}

	return manifest, releaseVersion
}

// fetchSourceManifests mutates the passed manifest by appending one
// appropriate for the passed TektonComponent
func (ctrl Controller) fetchSourceManifests(ctx context.Context, opts PayloadOptions) error {
	component := strings.TrimSuffix(ctrl.VersionConfigMap, "-info")
	switch component {
	case "pipelines":
		var pipeline *v1alpha1.TektonPipeline
		if err := AppendTarget(ctx, ctrl.Manifest, pipeline); err != nil {
			return err
		}
		if strings.EqualFold(os.Getenv("DISABLE_PROXY_WEBHOOK"), "true") {
			return nil
		}
		return addProxy(ctrl.Manifest)
	case "triggers":
		var trigger *v1alpha1.TektonTrigger
		return AppendTarget(ctx, ctrl.Manifest, trigger)
	case "dashboard":
		if opts.ReadOnly {
			var dashboard v1alpha1.TektonDashboard
			dashboard.Spec.Readonly = true
			return AppendTarget(ctx, ctrl.Manifest, &dashboard)
		} else {
			var dashboard v1alpha1.TektonDashboard
			dashboard.Spec.Readonly = false
			return AppendTarget(ctx, ctrl.Manifest, &dashboard)
		}
	case "chains":
		var chain v1alpha1.TektonChain
		return AppendTarget(ctx, ctrl.Manifest, &chain)
	case "tekton-results":
		var results v1alpha1.TektonResult
		return AppendTarget(ctx, ctrl.Manifest, &results)
	case "pipelines-as-code":
		pacLocation := filepath.Join(os.Getenv(KoEnvKey), "tekton-addon", "pipelines-as-code")
		return AppendManifest(ctrl.Manifest, pacLocation)
	case "manual-approval-gate":
		var mag v1alpha1.ManualApprovalGate
		return AppendTarget(ctx, ctrl.Manifest, &mag)
	case v1alpha1.TektonPrunerResourceName:
		var pruner v1alpha1.TektonPruner
		return AppendTarget(ctx, ctrl.Manifest, &pruner)
	case v1alpha1.TektonSchedulerResourceName:
		var scheduler v1alpha1.TektonScheduler
		return AppendTarget(ctx, ctrl.Manifest, &scheduler)
	}

	return nil
}

func addProxy(manifest *mf.Manifest) error {
	koDataDir := os.Getenv(KoEnvKey)
	proxyLocation := filepath.Join(koDataDir, "webhook")
	return AppendManifest(manifest, proxyLocation)
}
