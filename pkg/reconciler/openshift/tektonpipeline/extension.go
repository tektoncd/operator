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

package tektonpipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

const (
	monitoringLabelKey = "openshift.io/cluster-monitoring"
	enableMetricsKey   = "enableMetrics"
	versionKey         = "VERSION"

	tektonPipelinesControllerName       = "tekton-pipelines-controller"
	tektonRemoteResolversControllerName = "tekton-pipelines-remote-resolvers"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)
	version := os.Getenv(versionKey)
	if version == "" {
		logger.Fatal("Failed to find version from env")
	}

	ext := openshiftExtension{
		// component version is used for metrics, passing a dummy
		// value through extension not going to affect execution
		installerSetClient: client.NewInstallerSetClient(operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets(),
			version, "pipelines-ext", v1alpha1.KindTektonPipeline, nil),
		kubeClientSet: kubeclient.Get(ctx),
	}
	return ext
}

type openshiftExtension struct {
	installerSetClient *client.InstallerSetClient
	kubeClientSet      kubernetes.Interface
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	trns := []mf.Transformer{
		occommon.ApplyCABundlesToDeployment,
		occommon.RemoveRunAsUser(),
		occommon.RemoveRunAsUserForStatefulSet(tektonPipelinesControllerName),
		occommon.RemoveRunAsUserForStatefulSet(tektonRemoteResolversControllerName),
		occommon.ApplyCABundlesForStatefulSet(tektonPipelinesControllerName),
		occommon.ApplyCABundlesForStatefulSet(tektonRemoteResolversControllerName),
	}
	return trns
}
func (oe openshiftExtension) PreReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	manifest, err := preManifest()
	if err != nil {
		return err
	}

	// Filtering out the namespace because it add TektonPipeline as OwnerRef in targetNamespace
	*manifest = manifest.Filter(mf.Not(mf.ByKind("Namespace")))
	if err := oe.installerSetClient.PreSet(ctx, comp, manifest, filterAndTransform()); err != nil {
		return err
	}

	// update monitoring label based on metric enable status under params
	// namespace creation/modifications are not handled by manifests, see above, namespace filtered from manifests
	pipeline := comp.(*v1alpha1.TektonPipeline)
	value := strings.ToLower(findParam(pipeline.Spec.Params, enableMetricsKey))
	labels := map[string]string{
		monitoringLabelKey: "false",
	}
	if value == "" || value == "true" {
		labels[monitoringLabelKey] = "true"
	}

	// reconcile namespace with updated labels
	return common.ReconcileTargetNamespace(ctx, labels, nil, comp, oe.kubeClientSet)
}

func (oe openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	pipeline := comp.(*v1alpha1.TektonPipeline)

	// Install monitoring if metrics is enabled
	value := strings.ToLower(findParam(pipeline.Spec.Params, enableMetricsKey))

	if value == "" || value == "true" {
		manifest, err := postManifest()
		if err != nil {
			return err
		}
		if err := oe.installerSetClient.PostSet(ctx, comp, manifest, filterAndTransformMonitoring(comp)); err != nil {
			return err
		}
	} else {
		if err := oe.installerSetClient.CleanupPostSet(ctx); err != nil {
			return err
		}
	}

	return nil
}
func (oe openshiftExtension) Finalize(ctx context.Context, comp v1alpha1.TektonComponent) error {
	if err := oe.installerSetClient.CleanupPostSet(ctx); err != nil {
		return err
	}
	if err := oe.installerSetClient.CleanupPreSet(ctx); err != nil {
		return err
	}
	return nil
}

func preManifest() (*mf.Manifest, error) {
	koDataDir := os.Getenv(common.KoEnvKey)
	manifest := &mf.Manifest{}

	// make sure that openshift-pipelines namespace exists
	namespaceLocation := filepath.Join(koDataDir, "tekton-namespace")
	if err := common.AppendManifest(manifest, namespaceLocation); err != nil {
		return nil, err
	}

	// add inject CA bundles manifests
	cabundlesLocation := filepath.Join(koDataDir, "cabundles")
	if err := common.AppendManifest(manifest, cabundlesLocation); err != nil {
		return nil, err
	}

	return manifest, nil
}

func postManifest() (*mf.Manifest, error) {
	koDataDir := os.Getenv(common.KoEnvKey)
	manifest := &mf.Manifest{}

	monitoringLocation := filepath.Join(koDataDir, "openshift-monitoring")
	if err := common.AppendManifest(manifest, monitoringLocation); err != nil {
		return nil, err
	}
	return manifest, nil
}

func filterAndTransform() client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		if err := common.Transform(ctx, manifest, comp); err != nil {
			return nil, err
		}
		return manifest, nil
	}
}

// filterAndTransformMonitoring applies ServiceMonitor namespace updates to monitoring manifests
func filterAndTransformMonitoring(comp v1alpha1.TektonComponent) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		if err := common.Transform(ctx, manifest, comp); err != nil {
			return nil, err
		}
		// Apply ServiceMonitor namespace transformer specifically for monitoring manifests
		// This fixes hardcoded namespace in openshift-monitoring ServiceMonitors
		tfs := []mf.Transformer{
			occommon.UpdateServiceMonitorTargetNamespace(comp.GetSpec().GetTargetNamespace()),
		}
		if err := common.Transform(ctx, manifest, comp, tfs...); err != nil {
			return nil, err
		}
		return manifest, nil
	}
}

func findParam(params []v1alpha1.Param, param string) string {
	for _, p := range params {
		if p.Name == param {
			return p.Value
		}
	}
	return ""
}
