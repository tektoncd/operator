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
	"fmt"

	mf "github.com/manifestival/manifestival"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/route/clientset/versioned/scheme"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"knative.dev/pkg/logging"
)

func (r *Reconciler) EnsureConsoleCLI(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	tknservecliManifest := r.openShiftConsoleManifest
	if err := common.Transform(ctx, tknservecliManifest, ta); err != nil {
		return err
	}
	routeHost, err := getRouteHost(tknservecliManifest)
	if err != nil {
		return err
	}
	manifest := *r.consoleCLIManifest
	if err := consoleCLITransform(ctx, &manifest, routeHost); err != nil {
		return err
	}
	if err := r.installerSetClient.CustomSet(ctx, ta, ConsoleCLIInstallerSet, &manifest, filterAndTransformOCPResources()); err != nil {
		return err
	}
	return nil
}

func getRouteHost(manifest *mf.Manifest) (string, error) {
	var hostUrl string
	for _, r := range manifest.Filter(mf.ByKind("Route")).Resources() {
		u, err := manifest.Client.Get(&r)
		if err != nil {
			return "", err
		}
		if u.GetName() == "tkn-cli-serve" {
			route := &routev1.Route{}
			if err := scheme.Scheme.Convert(u, route, nil); err != nil {
				return "", err
			}
			hostUrl = route.Spec.Host
		}
	}
	return hostUrl, nil
}

func consoleCLITransform(ctx context.Context, manifest *mf.Manifest, baseURL string) error {
	tknVersion := "0.31.1"

	if baseURL == "" {
		return fmt.Errorf("route url should not be empty")
	}
	logger := logging.FromContext(ctx)
	logger.Debug("Transforming manifest")

	transformers := []mf.Transformer{
		replaceURLCCD(baseURL, tknVersion),
	}

	transformManifest, err := manifest.Transform(transformers...)
	if err != nil {
		return err
	}

	*manifest = transformManifest
	return nil
}
