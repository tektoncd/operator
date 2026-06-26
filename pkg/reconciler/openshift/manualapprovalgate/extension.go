/*
Copyright 2024 The Tekton Authors

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

package manualapprovalgate

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	tektonConfiginformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"knative.dev/pkg/logging"
)

const (
	// magWebhookDeployment is the name of the MAG webhook Deployment as defined in
	// the upstream manual-approval-gate config (config/kubernetes/500-webhook.yaml).
	magWebhookDeployment = "manual-approval-gate-webhook"
	// magWebhookContainerName is the container name inside the MAG webhook Deployment.
	magWebhookContainerName = "manual-approval"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	return &openshiftExtension{
		tektonConfigLister: tektonConfiginformer.Get(ctx).Lister(),
	}
}

type openshiftExtension struct {
	tektonConfigLister occommon.TektonConfigLister
	resolvedTLSConfig  *occommon.TLSEnvVars
}

func (oe *openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	var trns []mf.Transformer

	// Inject APIServer TLS profile env vars into the MAG webhook so that it
	// applies the cluster-wide TLS version and cipher suite policy (PQC readiness).
	// The MAG webhook uses the Knative webhook framework (sharedmain.MainWithConfig),
	// which calls knativetls.DefaultConfigFromEnv("WEBHOOK_"), so it reads the
	// WEBHOOK_TLS_* env vars.
	if oe.resolvedTLSConfig != nil {
		trns = append(trns,
			occommon.InjectTLSEnvVars(oe.resolvedTLSConfig, "Deployment", magWebhookDeployment, []string{magWebhookContainerName}, occommon.WebhookEnvVarPrefix),
		)
	}

	return trns
}

func (oe *openshiftExtension) PreReconcile(ctx context.Context, _ v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)

	resolvedTLS, err := occommon.ResolveCentralTLSToEnvVars(ctx, oe.tektonConfigLister)
	if err != nil {
		return err
	}
	oe.resolvedTLSConfig = resolvedTLS
	if oe.resolvedTLSConfig != nil {
		logger.Infof("Injecting central TLS config into MAG webhook: MinVersion=%s", oe.resolvedTLSConfig.MinVersion)
	}

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
