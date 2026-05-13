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

package tektontrigger

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	tektonConfiginformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektontrigger"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

const (
	tektonTriggersWebhookDeployment = "tekton-triggers-webhook"
	webhookContainerName            = "webhook"
	tektonTriggersCoreInterceptors  = "tekton-triggers-core-interceptors"
	coreInterceptorsContainerName   = "tekton-triggers-core-interceptors"
)

// triggersProperties holds fields for configuring runAsUser and runAsGroup.
type triggersProperties struct {
	DefaultRunAsUser  *string `json:"default-run-as-user,omitempty"`
	DefaultRunAsGroup *string `json:"default-run-as-group,omitempty"`
	DefaultFSGroup    *string `json:"default-fs-group,omitempty"`
}

// Updating the default values of runAsUser and runAsGroup to an empty string
// to ensure compatibility with OpenShift's requirements for managing these settings
// in Triggers Eventlistener containers SCC.
var triggersData = triggersProperties{
	DefaultRunAsUser:  ptr.String(""),
	DefaultRunAsGroup: ptr.String(""),
	DefaultFSGroup:    ptr.String(""),
}

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
	trns := []mf.Transformer{
		occommon.RemoveRunAsUser(),
		occommon.RemoveRunAsGroup(),
		occommon.ApplyCABundlesToDeployment,
		common.AddConfigMapValues(tektontrigger.ConfigDefaults, triggersData),
		replaceDeploymentArgs("-el-events", "enable"),
	}

	// Inject APIServer TLS profile env vars into the webhook and core interceptors
	// so that both apply the cluster-wide TLS version and cipher suite policy (PQC readiness).
	if oe.resolvedTLSConfig != nil {
		trns = append(trns,
			occommon.InjectTLSEnvVars(oe.resolvedTLSConfig, "Deployment", tektonTriggersWebhookDeployment, []string{webhookContainerName}),
			occommon.InjectTLSEnvVars(oe.resolvedTLSConfig, "Deployment", tektonTriggersCoreInterceptors, []string{coreInterceptorsContainerName}),
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
		logger.Infof("Injecting central TLS config into triggers webhook and core interceptors: MinVersion=%s", oe.resolvedTLSConfig.MinVersion)
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
