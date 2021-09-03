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
	return []mf.Transformer{
		common.InjectLabelOnNamespace(monitoringLabel),
		occommon.ApplyCABundles,
		occommon.RemoveRunAsUser(),
	}
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
	if crUpdated := SetDefault(&tp.Spec.PipelineProperties); crUpdated {
		if _, err := oe.operatorClientSet.OperatorV1alpha1().TektonPipelines().Update(ctx, tp, v1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (oe openshiftExtension) PostReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func SetDefault(properties *v1alpha1.PipelineProperties) bool {

	var updated = false

	// Set default service account as pipeline
	if properties.DefaultServiceAccount == "" {
		properties.DefaultServiceAccount = common.DefaultSA
		updated = true
	}

	// Set `disable-affinity-assistant` to true if not set in CR
	// webhook will not set any value but by default in pipelines configmap it will be false
	if properties.DisableAffinityAssistant == nil {
		properties.DisableAffinityAssistant = ptr.Bool(DefaultDisableAffinityAssistant)
		updated = true
	}

	return updated
}
