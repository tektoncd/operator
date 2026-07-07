/*
Copyright 2025 The Tekton Authors

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

package tektonpruner

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonConfiginformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

const (
	tektonPrunerWebhookDeployment    = "tekton-pruner-webhook"
	tektonPrunerControllerDeployment = "tekton-pruner-controller"
	webhookContainerName             = "webhook"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	return &openshiftExtension{
		kubeClientSet:      kubeclient.Get(ctx),
		operatorClientSet:  operatorclient.Get(ctx),
		tektonConfigLister: tektonConfiginformer.Get(ctx).Lister(),
	}
}

type openshiftExtension struct {
	kubeClientSet      kubernetes.Interface
	operatorClientSet  versioned.Interface
	tektonConfigLister occommon.TektonConfigLister
	resolvedTLSConfig  *occommon.TLSEnvVars
	metricsMTLSReady   bool
}

func (oe *openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	trns := []mf.Transformer{
		occommon.RemoveRunAsUser(),
		occommon.RemoveRunAsGroup(),
	}

	if oe.metricsMTLSReady {
		trns = append(trns,
			occommon.AnnotateMetricsServingCert(tektonPrunerControllerDeployment),
			occommon.RenameServicePort(tektonPrunerControllerDeployment, occommon.MetricsHTTPPort, occommon.MetricsHTTPSPort),
			occommon.ApplyMetricsTLS("Deployment", tektonPrunerControllerDeployment,
				occommon.MetricsServingCertSecretName(tektonPrunerControllerDeployment)),
		)
	}

	if oe.resolvedTLSConfig != nil {
		trns = append(trns,
			occommon.InjectTLSEnvVars(oe.resolvedTLSConfig, "Deployment", tektonPrunerWebhookDeployment, []string{webhookContainerName}, occommon.WebhookEnvVarPrefix),
		)
	}

	return trns
}

func (oe *openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)

	resolvedTLS, err := occommon.ResolveCentralTLSToEnvVars(ctx, oe.tektonConfigLister)
	if err != nil {
		return err
	}
	oe.resolvedTLSConfig = resolvedTLS
	if oe.resolvedTLSConfig != nil {
		logger.Infof("Injecting central TLS config into pruner webhook: MinVersion=%s", oe.resolvedTLSConfig.MinVersion)
	}

	ready, err := occommon.ResolveMetricsMTLS(ctx, oe.operatorClientSet, oe.kubeClientSet, tc.GetSpec().GetTargetNamespace())
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
