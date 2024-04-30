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

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	manualapprovalgateinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/manualapprovalgate"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	manualapprovalgatereconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/manualapprovalgate"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const versionConfigMap = "manual-approval-gate-info"

func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return NewExtendedController(common.NoExtension)(ctx, cmw)
}

func NewExtendedController(generator common.ExtensionGenerator) injection.ControllerConstructor {
	return func(ctx context.Context, w configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)

		ctrl := common.Controller{
			Logger:           logger,
			VersionConfigMap: versionConfigMap,
		}

		manifest, ver := ctrl.InitController(ctx, common.PayloadOptions{})
		if ver == common.ReleaseVersionUnknown {
			ver = "devel"
		}

		operatorVer, err := common.OperatorVersion(ctx)
		if err != nil {
			logger.Fatal(err)
		}

		metrics, _ := NewRecorder()

		tisClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()

		c := &Reconciler{
			operatorClientSet:         operatorclient.Get(ctx),
			kubeClientSet:             kubeclient.Get(ctx),
			extension:                 generator(ctx),
			manifest:                  manifest,
			installerSetClient:        client.NewInstallerSetClient(tisClient, operatorVer, ver, v1alpha1.KindManualApprovalGate, metrics),
			pipelineInformer:          tektonPipelineinformer.Get(ctx),
			operatorVersion:           operatorVer,
			manualApprovalGateVersion: ver,
		}
		impl := manualapprovalgatereconciler.NewImpl(ctx, c)

		logger.Info("Setting up event handlers for ManualApprovalGate")

		if _, err := manualapprovalgateinformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue)); err != nil {
			logger.Panicf("Couldn't register ManualApprovalGate informer event handler: %w", err)
		}

		if _, err := tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.ManualApprovalGate{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		}); err != nil {
			logger.Panicf("Couldn't register TektonInstallerSet informer event handler: %w", err)
		}

		return impl
	}
}
