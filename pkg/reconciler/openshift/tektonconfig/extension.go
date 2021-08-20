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
	"os"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonconfig/extension"
	openshiftPipeline "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonpipeline"
	openshiftTrigger "github.com/tektoncd/operator/pkg/reconciler/openshift/tektontrigger"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	versionKey = "VERSION"
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
		kubeClientSet:     kubeclient.Get(ctx),
		manifest:          manifest,
	}

	return ext
}

type openshiftExtension struct {
	operatorClientSet versioned.Interface
	kubeClientSet     kubernetes.Interface
	manifest          mf.Manifest
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{}
}
func (oe openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {

	config := tc.(*v1alpha1.TektonConfig)
	pipelineUpdated := openshiftPipeline.SetDefault(&config.Spec.Pipeline.PipelineProperties)
	triggerUpdated := openshiftTrigger.SetDefault(&config.Spec.Trigger.TriggersProperties)
	if pipelineUpdated || triggerUpdated {
		if _, err := oe.operatorClientSet.OperatorV1alpha1().TektonConfigs().Update(ctx, config, v1.UpdateOptions{}); err != nil {
			return err
		}
	}

	r := rbac{
		kubeClientSet:     oe.kubeClientSet,
		operatorClientSet: oe.operatorClientSet,
		manifest:          oe.manifest,
		ownerRef:          configOwnerRef(tc),
		version:           os.Getenv(versionKey),
	}
	return r.createResources(ctx)
}
func (oe openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)
	if configInstance.Spec.Profile == common.ProfileAll {
		if err := extension.CreateAddonCR(comp, oe.operatorClientSet.OperatorV1alpha1()); err != nil {
			return err
		}
	}
	return nil
}
func (oe openshiftExtension) Finalize(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)
	if configInstance.Spec.Profile == common.ProfileAll {
		if err := extension.TektonAddonCRDelete(oe.operatorClientSet.OperatorV1alpha1().TektonAddons(), common.AddonResourceName); err != nil {
			return err
		}
	}

	r := rbac{
		kubeClientSet: oe.kubeClientSet,
		version:       os.Getenv(versionKey),
	}
	return r.cleanUp(ctx)
}

// configOwnerRef returns owner reference pointing to passed instance
func configOwnerRef(tc v1alpha1.TektonComponent) metav1.OwnerReference {
	return *metav1.NewControllerRef(tc, tc.GroupVersionKind())
}
