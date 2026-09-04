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
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonConfiginformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
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
	tektonPipelinesWebhookDeployment    = "tekton-pipelines-webhook"
	webhookContainerName                = "webhook"

	// tektonEventsControllerName is the Deployment that exposes an http-metrics
	// Service and must therefore receive the mTLS volumes and env vars.
	tektonEventsControllerName = "tekton-events-controller"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)
	version := os.Getenv(versionKey)
	if version == "" {
		logger.Fatal("Failed to find version from env")
	}

	opClient := operatorclient.Get(ctx)
	ext := &openshiftExtension{
		// component version is used for metrics, passing a dummy
		// value through extension not going to affect execution
		installerSetClient: client.NewInstallerSetClient(opClient.OperatorV1alpha1().TektonInstallerSets(),
			version, "pipelines-ext", v1alpha1.KindTektonPipeline, nil),
		kubeClientSet:      kubeclient.Get(ctx),
		operatorClientSet:  opClient,
		tektonConfigLister: tektonConfiginformer.Get(ctx).Lister(),
	}
	return ext
}

type openshiftExtension struct {
	installerSetClient *client.InstallerSetClient
	kubeClientSet      kubernetes.Interface
	operatorClientSet  versioned.Interface
	tektonConfigLister occommon.TektonConfigLister
	resolvedTLSConfig  *occommon.TLSEnvVars
	metricsMTLSReady   bool
}

func (oe *openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	trns := []mf.Transformer{
		occommon.ApplyCABundlesToDeployment,
		occommon.RemoveRunAsUser(),
		occommon.RemoveRunAsUserForStatefulSet(tektonPipelinesControllerName),
		occommon.RemoveRunAsUserForStatefulSet(tektonRemoteResolversControllerName),
		occommon.ApplyCABundlesForStatefulSet(tektonPipelinesControllerName),
		occommon.ApplyCABundlesForStatefulSet(tektonRemoteResolversControllerName),
		common.ReplaceNamespaceInClusterRoleBinding(comp.GetSpec().GetTargetNamespace()),
	}

	if oe.metricsMTLSReady {
		// mTLS for Prometheus scraping: annotate metric Services for cert provisioning,
		// rename their ports, and mount the TLS Secret + client-CA ConfigMap into pods.
		// The webhook is intentionally omitted: it does not support METRICS_PROMETHEUS_TLS_* env vars.
		trns = append(trns,
			occommon.AnnotateMetricsServingCert(tektonPipelinesControllerName),
			occommon.RenameServicePort(tektonPipelinesControllerName, occommon.MetricsHTTPPort, occommon.MetricsHTTPSPort),
			occommon.AnnotateMetricsServingCert(tektonEventsControllerName),
			occommon.RenameServicePort(tektonEventsControllerName, occommon.MetricsHTTPPort, occommon.MetricsHTTPSPort),
			occommon.AnnotateMetricsServingCert(tektonRemoteResolversControllerName),
			occommon.RenameServicePort(tektonRemoteResolversControllerName, occommon.MetricsHTTPPort, occommon.MetricsHTTPSPort),
			// Cover both Deployment and StatefulSet variants.
			occommon.ApplyMetricsTLS("Deployment", tektonPipelinesControllerName,
				occommon.MetricsServingCertSecretName(tektonPipelinesControllerName)),
			occommon.ApplyMetricsTLS("StatefulSet", tektonPipelinesControllerName,
				occommon.MetricsServingCertSecretName(tektonPipelinesControllerName)),
			occommon.ApplyMetricsTLS("Deployment", tektonEventsControllerName,
				occommon.MetricsServingCertSecretName(tektonEventsControllerName)),
			occommon.ApplyMetricsTLS("Deployment", tektonRemoteResolversControllerName,
				occommon.MetricsServingCertSecretName(tektonRemoteResolversControllerName)),
			occommon.ApplyMetricsTLS("StatefulSet", tektonRemoteResolversControllerName,
				occommon.MetricsServingCertSecretName(tektonRemoteResolversControllerName)),
		)
	}

	// Inject APIServer TLS profile env vars into the webhook so that it applies
	// the cluster-wide TLS version and cipher suite policy (PQC readiness).
	if oe.resolvedTLSConfig != nil {
		trns = append(trns, occommon.InjectTLSEnvVars(oe.resolvedTLSConfig, "Deployment", tektonPipelinesWebhookDeployment, []string{webhookContainerName}, occommon.WebhookEnvVarPrefix))
	}

	return trns
}

func (oe *openshiftExtension) PreReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)

	resolvedTLS, err := occommon.ResolveCentralTLSToEnvVars(ctx, oe.tektonConfigLister)
	if err != nil {
		return err
	}
	oe.resolvedTLSConfig = resolvedTLS
	if oe.resolvedTLSConfig != nil {
		logger.Infof("Injecting central TLS config into webhook: MinVersion=%s", oe.resolvedTLSConfig.MinVersion)
	}

	ready, err := occommon.ResolveMetricsMTLS(ctx, oe.operatorClientSet, oe.kubeClientSet, comp.GetSpec().GetTargetNamespace())
	if err != nil {
		return err
	}
	oe.metricsMTLSReady = ready

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

func (oe *openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	pipeline := comp.(*v1alpha1.TektonPipeline)

	// Install monitoring if metrics is enabled
	value := strings.ToLower(findParam(pipeline.Spec.Params, enableMetricsKey))

	if value == "" || value == "true" {
		manifest, err := postManifest()
		if err != nil {
			return err
		}
		if err := oe.installerSetClient.PostSet(ctx, comp, manifest, filterAndTransformMonitoring(comp, oe.metricsMTLSReady)); err != nil {
			return err
		}
	} else {
		if err := oe.installerSetClient.CleanupPostSet(ctx); err != nil {
			return err
		}
	}

	return nil
}
func (oe *openshiftExtension) Finalize(ctx context.Context, comp v1alpha1.TektonComponent) error {
	if err := oe.installerSetClient.CleanupPostSet(ctx); err != nil {
		return err
	}
	if err := oe.installerSetClient.CleanupPreSet(ctx); err != nil {
		return err
	}
	return nil
}

func (oe *openshiftExtension) GetPlatformData() string {
	return ""
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

// filterAndTransformMonitoring applies namespace substitution and, when mTLS is
// ready, the full mTLS config to each component's ServiceMonitor.
//
// The source monitoring YAMLs use plain http-metrics as the baseline; this
// function upgrades them to https-metrics + tlsConfig when metricsMTLSReady is
// true, so that the feature flag cleanly controls the scraping mode.
func filterAndTransformMonitoring(comp v1alpha1.TektonComponent, metricsMTLSReady bool) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		if err := common.Transform(ctx, manifest, comp); err != nil {
			return nil, err
		}
		targetNS := comp.GetSpec().GetTargetNamespace()
		tfs := []mf.Transformer{
			occommon.UpdateServiceMonitorTargetNamespace(targetNS),
		}
		if metricsMTLSReady {
			tfs = append(tfs,
				occommon.UpdateServiceMonitorForMetricsMTLS(
					"openshift-pipelines-monitor",
					occommon.MetricsHTTPPort, occommon.MetricsHTTPSPort,
					tektonPipelinesControllerName, targetNS),
				occommon.UpdateServiceMonitorForMetricsMTLS(
					"openshift-triggers-monitor",
					occommon.MetricsHTTPPort, occommon.MetricsHTTPSPort,
					"tekton-triggers-controller", targetNS),
				occommon.UpdateServiceMonitorForMetricsMTLS(
					"openshift-chains-monitor",
					occommon.MetricsHTTPPort, occommon.MetricsHTTPSPort,
					"tekton-chains-metrics", targetNS),
				occommon.UpdateServiceMonitorForMetricsMTLS(
					"openshift-pruner-monitor",
					occommon.MetricsHTTPPort, occommon.MetricsHTTPSPort,
					"tekton-pruner-controller", targetNS),
				// TektonResult's watcher ServiceMonitor ships in this same shared
				// openshift-monitoring manifest set (owned by the TektonPipeline
				// PostSet), so it must be upgraded here too, in lockstep with the
				// Service port rename tektonresult's own extension applies to
				// tekton-results-watcher. Results uses a non-standard port name
				// ("metrics" rather than "http-metrics"), hence the explicit name.
				//
				// tekton-results-api is intentionally excluded: its Prometheus
				// metrics endpoint is a bare net/http server hardcoded in
				// tektoncd/results (cmd/api/main.go) that does not honor
				// METRICS_PROMETHEUS_TLS_* env vars, so it cannot serve mTLS.
				// Its ServiceMonitor/Service stay on plain HTTP regardless of
				// this flag.
				occommon.UpdateServiceMonitorForMetricsMTLS(
					"openshift-results-watcher-monitor",
					"metrics", occommon.MetricsHTTPSPort,
					"tekton-results-watcher", targetNS),
			)
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
