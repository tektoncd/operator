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

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// DefaultSA is the default service account
	DefaultSA = "pipeline"
	// DefaultDisableAffinityAssistant is default value of disable affinity assistant flag
	DefaultDisableAffinityAssistant = "true"
	DefaultTargetNamespace          = "openshift-pipelines"
	AnnotationPreserveNS            = "operator.tekton.dev/preserve-namespace"
	AnnotationPreserveRBSubjectNS   = "operator.tekton.dev/preserve-rb-subject-namespace"
)

// NoPlatform "generates" a NilExtension
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
		injectDefaultSA(DefaultSA),
		setDisableAffinityAssistant(DefaultDisableAffinityAssistant),
		occommon.ApplyCABundles,
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

	return common.Install(ctx, &oe.manifest, tc)
}

func (oe openshiftExtension) PostReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

// injectDefaultSA adds default service account into config-defaults configMap
func injectDefaultSA(defaultSA string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if strings.ToLower(u.GetKind()) != "configmap" {
			return nil
		}
		if u.GetName() != "config-defaults" {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		cm.Data["default-service-account"] = defaultSA
		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

// setDisableAffinityAssistant set value of disable-affinity-assistant into feature-flags configMap
func setDisableAffinityAssistant(disableAffinityAssistant string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if strings.ToLower(u.GetKind()) != "configmap" {
			return nil
		}
		if u.GetName() != "feature-flags" {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		cm.Data["disable-affinity-assistant"] = disableAffinityAssistant
		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}
