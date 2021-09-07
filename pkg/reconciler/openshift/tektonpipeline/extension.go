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

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

const (
	// DefaultDisableAffinityAssistant is default value of disable affinity assistant flag
	DefaultDisableAffinityAssistant = true
	monitoringLabel                 = "openshift.io/cluster-monitoring=true"

	enableMetricsKey          = "enableMetrics"
	enableMetricsDefaultValue = "true"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)
	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
	if err != nil {
		logger.Fatalw("error creating initial manifest", zap.Error(err))
	}
	ext := openshiftExtension{
		operatorClientSet: operatorclient.Get(ctx),
		manifest:          manifest,
	}
	return ext
}

type openshiftExtension struct {
	operatorClientSet versioned.Interface
	manifest          mf.Manifest
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	trns := []mf.Transformer{
		occommon.ApplyCABundles,
		occommon.RemoveRunAsUser(),
	}

	pipeline := comp.(*v1alpha1.TektonPipeline)

	// Add monitoring label if metrics is enabled
	value := findParam(pipeline.Spec.Params, enableMetricsKey)
	if value == "" || value == "true" {
		trns = append(trns, common.InjectLabelOnNamespace(monitoringLabel))
	}

	return trns
}
func (oe openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	koDataDir := os.Getenv(common.KoEnvKey)

	// make sure that openshift-pipelines namespace exists
	namespaceLocation := filepath.Join(koDataDir, "tekton-namespace")
	if err := common.AppendManifest(&oe.manifest, namespaceLocation); err != nil {
		return err
	}

	// add inject CA bundles manifests
	cabundlesLocation := filepath.Join(koDataDir, "cabundles")
	if err := common.AppendManifest(&oe.manifest, cabundlesLocation); err != nil {
		return err
	}

	// add pipelines-scc
	pipelinesSCCLocation := filepath.Join(koDataDir, "tekton-pipeline", "00-prereconcile")
	if err := common.AppendManifest(&oe.manifest, pipelinesSCCLocation); err != nil {
		return err
	}

	// Apply the resources
	if err := oe.manifest.Apply(); err != nil {
		return err
	}

	tp := tc.(*v1alpha1.TektonPipeline)
	if crUpdated := SetDefault(&tp.Spec.Pipeline); crUpdated {
		if _, err := oe.operatorClientSet.OperatorV1alpha1().TektonPipelines().Update(ctx, tp, v1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (oe openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	pipeline := comp.(*v1alpha1.TektonPipeline)

	// Install monitoring if metrics is enabled
	value := findParam(pipeline.Spec.Params, enableMetricsKey)

	monitoringLocation := filepath.Join(koDataDir, "openshift-monitoring")
	if err := common.AppendManifest(&oe.manifest, monitoringLocation); err != nil {
		return err
	}

	trns, err := oe.manifest.Transform(
		mf.InjectNamespace(pipeline.Spec.TargetNamespace),
		mf.InjectOwner(pipeline),
	)
	if err != nil {
		return err
	}

	if value == "true" {
		return trns.Apply()
	}
	return trns.Delete()
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func SetDefault(pipeline *v1alpha1.Pipeline) bool {

	var updated = false

	// Set default service account as pipeline
	if pipeline.DefaultServiceAccount == "" {
		pipeline.DefaultServiceAccount = common.DefaultSA
		updated = true
	}

	// Set `disable-affinity-assistant` to true if not set in CR
	// webhook will not set any value but by default in pipelines configmap it will be false
	if pipeline.DisableAffinityAssistant == nil {
		pipeline.DisableAffinityAssistant = ptr.Bool(DefaultDisableAffinityAssistant)
		updated = true
	}

	// Add params with default values if not defined by user
	var found = false
	for i, p := range pipeline.Params {
		if p.Name == enableMetricsKey {
			found = true
			// If the value set is invalid then set key to default value
			// Not returning an error if the values is invalid as
			// we validate in reconciler and this would affect the
			// rest of the installation
			if p.Value != "false" && p.Value != "true" {
				pipeline.Params[i].Value = enableMetricsDefaultValue
				updated = true
			}
			break
		}
	}

	if !found {
		pipeline.Params = append(pipeline.Params, v1alpha1.Param{
			Name:  enableMetricsKey,
			Value: enableMetricsDefaultValue,
		})
		updated = true
	}

	return updated
}

func findParam(params []v1alpha1.Param, param string) string {
	for _, p := range params {
		if p.Name == param {
			return p.Value
		}
	}
	return ""
}
