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

package sharedresource

import (
	"context"

	"github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonconfiginformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	tektoninstallersetclient "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	openshiftcommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/common/legacybuilds"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// openshiftExtension holds the OpenShift-specific behaviour for the Shared
// Resource component. Shared Resource is OpenShift-only, so this extension only
// needs to adapt the workloads to OpenShift's SCC admission.
type openshiftExtension struct {
	installerSetClient *tektoninstallersetclient.InstallerSetClient
	kubernetesClient   kubernetes.Interface
	metricsMTLSReady   bool
	operatorClientset  versioned.Interface
	platformParams     networkpolicy.PlatformParams
	resolvedTLSConfig  *openshiftcommon.TLSEnvVars
	tektonConfigLister openshiftcommon.TektonConfigLister
	dynClient          dynamic.Interface
}

// NewOpenShiftExtension builds the OpenShift extension for the Shared Resource component
func NewOpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)

	operatorVersion, err := common.OperatorVersion(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	tektonInstallerSetClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()

	return &openshiftExtension{
		// component version is used for metrics only; a descriptive value is fine here.
		installerSetClient: tektoninstallersetclient.NewInstallerSetClient(tektonInstallerSetClient, operatorVersion, OperandName, v1alpha1.KindSharedResource, nil),
		kubernetesClient:   kubeclient.Get(ctx),
		operatorClientset:  operatorclient.Get(ctx),
		platformParams:     networkpolicy.OpenShiftPlatformDefaults(),
		tektonConfigLister: tektonconfiginformer.Get(ctx).Lister(),
		dynClient:          dynamic.NewForConfigOrDie(injection.GetConfig(ctx)),
	}
}

// Transformers returns the OpenShift-only manifest transformers for Shared Resource.
func (oe *openshiftExtension) Transformers(tc v1alpha1.TektonComponent) []manifestival.Transformer {
	// Drop hard-coded UID/GID so the restricted SCC controller assigns them, and
	// rely on the OpenShift service-ca operator to issue and inject the webhook's
	// serving certificate and CA bundle instead of managing them ourselves.
	transformers := []manifestival.Transformer{
		openshiftcommon.RemoveRunAsUser(),
		openshiftcommon.RemoveRunAsGroup(),
		openshiftcommon.UpdateServiceMonitorTargetNamespace(tc.GetSpec().GetTargetNamespace()),
		legacybuilds.WebhookReadiness(WebhookDeploymentName, WebhookContainerName, 8443),
	}

	// Only wire metrics over mTLS once the serving certificates exist, otherwise
	// the controller would fail to start serving on the HTTPS port.
	if oe.metricsMTLSReady {
		transformers = append(transformers,
			openshiftcommon.RenameServicePort(ServiceName, openshiftcommon.MetricsHTTPPort, openshiftcommon.MetricsHTTPSPort),
			openshiftcommon.UpdateServiceMonitorForMetricsMTLS(ServiceMonitorName, openshiftcommon.MetricsHTTPPort, openshiftcommon.MetricsHTTPSPort, ServiceName, tc.GetSpec().GetTargetNamespace()),
		)
	}

	// Inject APIServer TLS profile into the webhook so that both apply the cluster-wide TLS version
	// and cipher suite policy (PQC readiness).
	if oe.resolvedTLSConfig != nil {
		webhookArgs := []string{
			tlsMinVersion, oe.resolvedTLSConfig.MinVersion,
			tlsCipherSuites, oe.resolvedTLSConfig.CipherSuites,
		}
		transformers = append(transformers,
			common.AddContainerArgs(WebhookDeploymentName, WebhookContainerName, webhookArgs, false),
		)
	}

	return transformers
}

// PreReconcile resolves the cluster's TLS policy and metrics-mTLS readiness up
// front and caches them on the extension, so Transformers can shape the manifest
// without repeating these lookups.
func (oe *openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	if err := prepareLegacySharedResource(ctx, oe.kubernetesClient, oe.dynClient, tc.GetSpec().GetTargetNamespace()); err != nil {
		return err
	}
	tlsEnvVars, err := openshiftcommon.ResolveCentralTLSToEnvVars(ctx, oe.tektonConfigLister)
	if err != nil {
		return err
	}
	oe.resolvedTLSConfig = tlsEnvVars

	ready, err := openshiftcommon.ResolveMetricsMTLS(ctx, oe.operatorClientset, oe.kubernetesClient, tc.GetSpec().GetTargetNamespace())
	if err != nil {
		return err
	}
	oe.metricsMTLSReady = ready
	return nil
}

// PostReconcile applies the OpenShift NetworkPolicies for Shared Resource after
// the main manifests are in place.
func (oe *openshiftExtension) PostReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	sr := tc.(*v1alpha1.SharedResource)
	if err := oe.reconcileNetworkPolicies(ctx, sr); err != nil {
		return err
	}
	if err := waitForNodePluginReady(ctx, oe.kubernetesClient, sr.Spec.TargetNamespace); err != nil {
		return err
	}
	if err := legacybuilds.WaitForWebhookReady(ctx, oe.kubernetesClient, sr.Spec.TargetNamespace, WebhookDeploymentName); err != nil {
		return err
	}
	return migrateLegacySharedResource(ctx, oe.kubernetesClient, oe.dynClient, oe.operatorClientset, sr)
}

func (oe *openshiftExtension) reconcileNetworkPolicies(ctx context.Context, sr *v1alpha1.SharedResource) error {
	// Install Network Policies
	if sr.Spec.NetworkPolicy.Disabled {
		return oe.installerSetClient.CleanupCustomSet(ctx, NetworkPolicySet)
	}
	defaults := append(
		[]networkingv1.NetworkPolicy{defaultDenyPolicy()},
		defaultPolicies(oe.platformParams)...,
	)
	manifest, err := networkpolicy.Generate(
		sr.Spec.NetworkPolicy,
		sr.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}

	if err = oe.installerSetClient.CustomSet(ctx, sr, NetworkPolicySet, &manifest, NopFilterAndTransform(), nil); err != nil {
		return err
	}

	return nil
}

// Finalize is a no-op: installed resources are removed with their owning InstallerSets.
func (oe *openshiftExtension) Finalize(ctx context.Context, tc v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)
	if err := oe.installerSetClient.CleanupCustomSet(ctx, NetworkPolicySet); err != nil {
		logger.Error("Failed to cleanup sharedresource network policies installerset: ", err)
		return err
	}
	return nil
}

// GetPlatformData returns no extra platform data for Shared Resource.
func (oe *openshiftExtension) GetPlatformData() string {
	return ""
}
