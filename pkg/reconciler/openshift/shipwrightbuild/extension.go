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

	"github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonconfiginformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/shipwrightbuild"
	openshiftcommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

type openshiftExtension struct {
	kubeClientset      kubernetes.Interface
	operatorClientset  versioned.Interface
	tektonConfigLister openshiftcommon.TektonConfigLister
	resolvedTLSConfig  *openshiftcommon.TLSEnvVars
	metricsMTLSReady   bool
}

func NewOpenShiftExtension(ctx context.Context) common.Extension {
	return &openshiftExtension{
		kubeClientset:      kubeclient.Get(ctx),
		operatorClientset:  operatorclient.Get(ctx),
		tektonConfigLister: tektonconfiginformer.Get(ctx).Lister(),
	}
}

func (oe *openshiftExtension) Transformers(tc v1alpha1.TektonComponent) []manifestival.Transformer {
	transformers := []manifestival.Transformer{
		openshiftcommon.RemoveRunAsUser(),
		openshiftcommon.RemoveRunAsGroup(),
	}

	if oe.metricsMTLSReady {
		transformers = append(transformers,
			openshiftcommon.AnnotateMetricsServingCert(shipwrightbuild.ControllerDeploymentName),
			openshiftcommon.RenameServicePort(shipwrightbuild.ControllerDeploymentName, openshiftcommon.MetricsHTTPPort, openshiftcommon.MetricsHTTPSPort),
			openshiftcommon.ApplyMetricsTLS(common.KindDeployment, shipwrightbuild.ControllerDeploymentName,
			openshiftcommon.MetricsServingCertSecretName(shipwrightbuild.ControllerDeploymentName)),
		)
	}

	// Inject APIServer TLS profile into the webhook so that both apply the cluster-wide TLS version
	// and cipher suite policy (PQC readiness).
	if oe.resolvedTLSConfig != nil {
		webhookArgs := []string{
			tlsMinVersionFlag, oe.resolvedTLSConfig.MinVersion,
			tlsCipherSuites, oe.resolvedTLSConfig.CipherSuites,
		}
		transformers = append(transformers,
			common.AddContainerArgs(shipwrightbuild.WebhookDeploymentName, shipwrightbuild.WebhookContainerName, webhookArgs, false),
		)
	}

	return transformers
}

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

func (oe *openshiftExtension) PostReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func (oe *openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func (oe *openshiftExtension) GetPlatformData() string {
	return ""
}
