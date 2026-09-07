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

package shipwrightbuild

import (
	"context"
	"strings"

	"github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonconfiginformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/shipwrightbuild"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	openshiftcommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

// openshiftExtension carries the OpenShift-specific behaviour layered on top of
// the shared ShipwrightBuild reconciler. The resolved TLS and metrics-mTLS state
// is captured during PreReconcile and consumed by Transformers, so the fields
// act as a hand-off between the two hooks.
type openshiftExtension struct {
	// installerSetClient applies the NetworkPolicy CustomSet owned by the
	// ShipwrightBuild instance.
	installerSetClient *client.InstallerSetClient
	kubernetesClient   kubernetes.Interface
	// metricsMTLSReady reports whether the OpenShift service-ca has provisioned
	// the certificates needed to serve metrics over mTLS.
	metricsMTLSReady bool
	operatorClient   versioned.Interface
	// platformParams holds platform-specific values for building NetworkPolicy rules.
	platformParams networkpolicy.PlatformParams
	// resolvedTLSConfig holds the cluster-wide TLS version/cipher policy to push
	// onto the webhook; nil when no central policy is configured.
	resolvedTLSConfig  *openshiftcommon.TLSEnvVars
	tektonConfigLister openshiftcommon.TektonConfigLister
}

// NewOpenShiftExtension builds the OpenShift extension for the Shipwright Build component.
func NewOpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)

	operatorVersion, err := common.OperatorVersion(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	tektonInstallerSetClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()

	return &openshiftExtension{
		installerSetClient: client.NewInstallerSetClient(tektonInstallerSetClient, operatorVersion, shipwrightbuild.OperandName, v1alpha1.KindShipwrightBuild, nil),
		kubernetesClient:   kubeclient.Get(ctx),
		operatorClient:     operatorclient.Get(ctx),
		platformParams:     networkpolicy.OpenShiftPlatformDefaults(),
		tektonConfigLister: tektonconfiginformer.Get(ctx).Lister(),
	}
}

// Transformers returns the OpenShift-only manifest transformers applied to the
// Shipwright Build payload, adapting it to OpenShift security and TLS conventions.
func (oe *openshiftExtension) Transformers(tc v1alpha1.TektonComponent) []manifestival.Transformer {
	// Drop hard-coded UID/GID so the restricted SCC controller assigns them, and
	// rely on the OpenShift service-ca operator to issue and inject the webhook's
	// serving certificate and CA bundle instead of managing them ourselves.
	transformers := []manifestival.Transformer{
		openshiftcommon.RemoveRunAsUser(),
		openshiftcommon.RemoveRunAsGroup(),
		openshiftcommon.AddServiceCABundle(common.KindCustomResourceDefinition, nil),
		openshiftcommon.AddServiceCertificate(shipwrightbuild.WebhookServiceName, shipwrightbuild.WebhookSecretName),
	}

	// Only wire metrics over mTLS once the serving certificates exist, otherwise
	// the controller would fail to start serving on the HTTPS port.
	// TODO enable metric TLS when upstream ships a service for that
	//  https://github.com/shipwright-io/build/issues/2411
	/*
		if oe.metricsMTLSReady {
			transformers = append(transformers,
				openshiftcommon.AnnotateMetricsServingCert(shipwrightbuild.ControllerDeploymentName),
				openshiftcommon.RenameServicePort(shipwrightbuild.ControllerDeploymentName, openshiftcommon.MetricsHTTPPort, openshiftcommon.MetricsHTTPSPort),
				openshiftcommon.ApplyMetricsTLS(common.KindDeployment, shipwrightbuild.ControllerDeploymentName,
					openshiftcommon.MetricsServingCertSecretName(shipwrightbuild.ControllerDeploymentName)),
			)
		}
	*/

	// Inject APIServer TLS profile into the webhook so that both apply the cluster-wide TLS version
	// and cipher suite policy (PQC readiness).
	if oe.resolvedTLSConfig != nil {
		// TODO: Fix upstream. TSL Version flag should accept in format "1.2"
		// https://github.com/shipwright-io/build/pull/2372
		minVersion := "VersionTLS" + strings.ReplaceAll(oe.resolvedTLSConfig.MinVersion, ".", "")
		webhookArgs := []string{
			tlsMinVersion, minVersion,
			tlsCipherSuites, oe.resolvedTLSConfig.CipherSuites,
		}
		transformers = append(transformers,
			common.AddContainerArgs(shipwrightbuild.WebhookDeploymentName, shipwrightbuild.WebhookContainerName, webhookArgs, false),
		)
	}

	return transformers
}

// PreReconcile resolves the cluster's TLS policy and metrics-mTLS readiness up
// front and caches them on the extension, so Transformers can shape the manifest
// without repeating these lookups.
func (oe *openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	tlsEnvVars, err := openshiftcommon.ResolveCentralTLSToEnvVars(ctx, oe.tektonConfigLister)
	if err != nil {
		return err
	}
	oe.resolvedTLSConfig = tlsEnvVars

	ready, err := openshiftcommon.ResolveMetricsMTLS(ctx, oe.operatorClient, oe.kubernetesClient, tc.GetSpec().GetTargetNamespace())
	if err != nil {
		return err
	}
	oe.metricsMTLSReady = ready

	return nil
}

// PostReconcile applies the OpenShift NetworkPolicies for Shipwright Build after
// the main manifests are in place.
func (oe *openshiftExtension) PostReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	sb := tc.(*v1alpha1.ShipwrightBuild)

	// Install Network Policies
	if sb.Spec.NetworkPolicy.Disabled {
		return oe.installerSetClient.CleanupCustomSet(ctx, NetworkPolicySet)
	}
	defaults := append(
		[]networkingv1.NetworkPolicy{defaultDenyPolicy()},
		defaultPolicies(oe.platformParams)...,
	)
	manifest, err := networkpolicy.Generate(
		sb.Spec.NetworkPolicy,
		sb.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}

	if err = oe.installerSetClient.CustomSet(ctx, sb, NetworkPolicySet, &manifest, shipwrightbuild.NopFilterAndTransform(), nil); err != nil {
		return err
	}

	return nil
}

// Finalize is a no-op: the installed resources are cleaned up via their owning
// InstallerSets, so there is nothing OpenShift-specific to tear down here.
func (oe *openshiftExtension) Finalize(ctx context.Context, tc v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)
	if err := oe.installerSetClient.CleanupCustomSet(ctx, NetworkPolicySet); err != nil {
		logger.Error("Failed to cleanup shipwrightbuild network policies installerset: ", err)
		return err
	}
	return nil
}

// GetPlatformData returns no extra platform data for Shipwright Build.
func (oe *openshiftExtension) GetPlatformData() string {
	return ""
}
