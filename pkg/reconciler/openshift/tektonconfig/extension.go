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

package tektonconfig

import (
	"context"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonconfig/extension"
	"knative.dev/pkg/logging"
)

// NoPlatform "generates" a NilExtension
func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)
	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("Error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
	if err != nil {
		logger.Fatalw("Error creating initial manifest", zap.Error(err))
	}
	return openshiftExtension{
		operatorClientSet: operatorclient.Get(ctx),
		manifest:          manifest,
	}
}

type openshiftExtension struct {
	operatorClientSet versioned.Interface
	manifest          mf.Manifest
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{}
}
func (oe openshiftExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)
	if configInstance.Spec.Profile == common.ProfileAll {
		if err := extension.CreateAddonCR(comp, oe.operatorClientSet.OperatorV1alpha1()); err != nil {
			return err
		}
	}

	// Run clean up jobs for OpenShift
	if err := RemoveDeprecatedConfigCRD(ctx, &oe.manifest, configInstance); err != nil {
		return err
	}
	return nil
}
func (oe openshiftExtension) Finalize(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)
	if configInstance.Spec.Profile == common.ProfileAll {
		return extension.TektonAddonCRDelete(oe.operatorClientSet.OperatorV1alpha1().TektonAddons(), common.AddonResourceName)
	}
	return nil
}

func RemoveDeprecatedConfigCRD(ctx context.Context, manifest *mf.Manifest, config *v1alpha1.TektonConfig) error {
	// Remove deprecated config.operator.tekton.dev CRD
	// by running 'oc delete crd config.operator.tekton.dev' in a kubernetes job
	stages := common.Stages{
		extension.AppendCleanupTarget,
		extension.CleanupTransforms,
		extension.RunCleanup,
		extension.CheckCleanup,
	}
	if err := stages.Execute(ctx, manifest, config); err != nil {
		return err
	}
	return nil
}
