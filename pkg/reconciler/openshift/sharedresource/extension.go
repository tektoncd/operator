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
	openshiftcommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

// openshiftExtension holds the OpenShift-specific behaviour for the Shared
// Resource component. Shared Resource is OpenShift-only, so this extension only
// needs to adapt the workloads to OpenShift's SCC admission.
type openshiftExtension struct {
	kubeClientset      kubernetes.Interface
	operatorClientset  versioned.Interface
	tektonConfigLister openshiftcommon.TektonConfigLister
	// resolvedTLSConfig holds the cluster-wide TLS version/cipher policy to push
	// onto the webhook; nil when no central policy is configured.
	resolvedTLSConfig *openshiftcommon.TLSEnvVars
	// metricsMTLSReady reports whether the OpenShift service-ca has provisioned
	// the certificates needed to serve metrics over mTLS.
	metricsMTLSReady bool
}

// NewOpenShiftExtension builds the OpenShift extension for the Shared Resource component
func NewOpenShiftExtension(ctx context.Context) common.Extension {
	return &openshiftExtension{
		kubeClientset:      kubeclient.Get(ctx),
		operatorClientset:  operatorclient.Get(ctx),
		tektonConfigLister: tektonconfiginformer.Get(ctx).Lister(),
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
	}

	// Only wire metrics over mTLS once the serving certificates exist, otherwise
	// the controller would fail to start serving on the HTTPS port.
	if oe.metricsMTLSReady {
		transformers = append(transformers,
			openshiftcommon.RenameServicePort(ControllerDeploymentName, openshiftcommon.MetricsHTTPPort, openshiftcommon.MetricsHTTPSPort),
			openshiftcommon.UpdateServiceMonitorForMetricsMTLS(ServiceMonitorName, openshiftcommon.MetricsHTTPPort, openshiftcommon.MetricsHTTPSPort, ControllerDeploymentName, tc.GetSpec().GetTargetNamespace()),
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

	ready, err := openshiftcommon.ResolveMetricsMTLS(ctx, oe.operatorClientset, oe.kubeClientset, tc.GetSpec().GetTargetNamespace())
	if err != nil {
		return err
	}
	oe.metricsMTLSReady = ready
	return nil
}

// PostReconcile is a no-op: no OpenShift-specific work is needed after apply.
func (oe *openshiftExtension) PostReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

// Finalize is a no-op: installed resources are removed with their owning InstallerSets.
func (oe *openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

// GetPlatformData returns no extra platform data for Shared Resource.
func (oe *openshiftExtension) GetPlatformData() string {
	return ""
}
