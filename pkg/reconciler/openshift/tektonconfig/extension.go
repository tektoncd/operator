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
	"fmt"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonaddon"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonconfig/extension"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
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
	configInstance := tc.(*v1alpha1.TektonConfig)

	// If profile is all then validates the passed addon params and
	// add the missing params with their default values
	if configInstance.Spec.Profile == common.ProfileAll {
		updated, err := common.ValidateParamsAndSetDefault(ctx, &configInstance.Spec.Addon.Params, tektonaddon.AddonParams,
			tektonaddon.ValidateParamsConditions())
		if err != nil {
			return err
		}
		if updated {
			_, err := oe.operatorClientSet.OperatorV1alpha1().TektonConfigs().Update(ctx, configInstance, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			// Returning error just to make reconcile it again so that further code gets updated TektonConfig
			return fmt.Errorf("reconcile")
		}
	}

	r := rbac{
		kubeClientSet:     oe.kubeClientSet,
		operatorClientSet: oe.operatorClientSet,
		manifest:          oe.manifest,
		ownerRef:          configOwnerRef(tc),
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
		return extension.TektonAddonCRDelete(oe.operatorClientSet.OperatorV1alpha1().TektonAddons(), common.AddonResourceName)
	}
	return nil
}

// configOwnerRef returns owner reference pointing to passed instance
func configOwnerRef(tc v1alpha1.TektonComponent) metav1.OwnerReference {
	return *metav1.NewControllerRef(tc, tc.GroupVersionKind())
}
