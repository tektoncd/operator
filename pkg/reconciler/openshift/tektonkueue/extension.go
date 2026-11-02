/*
Copyright 2026 The Tekton Authors

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

package tektonkueue

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"k8s.io/client-go/discovery"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)
	ctrl := common.Controller{Logger: logger, VersionConfigMap: common.MultiKueuePluginManifestDir + "-info"}
	manifest, pluginVersion := ctrl.InitController(ctx, common.PayloadOptions{})
	operatorVersion, err := common.OperatorVersion(ctx)
	if err != nil {
		logger.Fatal("Error while getting operator version", err)
	}
	return &openshiftExtension{
		manifest:  manifest,
		discovery: kubeclient.Get(ctx).Discovery(),
		installerSetClient: client.NewInstallerSetClient(operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets(),
			operatorVersion, pluginVersion, v1alpha1.KindTektonKueue, nil),
	}
}

type openshiftExtension struct {
	manifest           mf.Manifest
	discovery          discovery.DiscoveryInterface
	installerSetClient *client.InstallerSetClient
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{
		occommon.RemoveRunAsUser(),
		occommon.RemoveRunAsGroup(),
	}
}
func (oe openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	if !tc.(*v1alpha1.TektonKueue).Spec.Kueue.IsACMPluginEnabled() {
		return oe.cleanupPlugin(ctx)
	}
	return nil
}
func (oe openshiftExtension) PostReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	kueue := tc.(*v1alpha1.TektonKueue)
	if !kueue.Spec.Kueue.IsACMPluginEnabled() {
		return nil
	}
	return oe.reconcilePlugin(ctx, kueue)
}
func (oe openshiftExtension) Finalize(ctx context.Context, _ v1alpha1.TektonComponent) error {
	// Only start the removal. TektonKueue owns the plugin installer set, so it
	// is garbage collected even if this fails.
	return oe.installerSetClient.CleanupCustomSet(ctx, multiKueuePluginSet)
}

func (oe openshiftExtension) GetPlatformData() string {
	return ""
}
