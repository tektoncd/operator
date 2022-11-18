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

package tektondashboard

import (
	"context"

	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonDashboardinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektondashboard"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	tektonDashboardreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektondashboard"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const versionConfigMap = "dashboard-info"

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return NewExtendedController(common.NoExtension)(ctx, cmw)
}

// NewExtendedController returns a controller extended to a specific platform
func NewExtendedController(generator common.ExtensionGenerator) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		tektonPipelineInformer := tektonPipelineinformer.Get(ctx)
		tektonDashboardInformer := tektonDashboardinformer.Get(ctx)

		logger := logging.FromContext(ctx)

		ctrl := common.Controller{
			Logger:           logger,
			VersionConfigMap: versionConfigMap,
		}

		readonlyManifest, dashboardVer := ctrl.InitController(ctx, common.PayloadOptions{ReadOnly: true})

		fullaccessManifest, _ := ctrl.InitController(ctx, common.PayloadOptions{ReadOnly: false})

		operatorVer, err := common.OperatorVersion(ctx)
		if err != nil {
			logger.Fatal(err)
		}

		metrics, err := common.NoMetrics()
		if err != nil {
			logger.Errorf("Failed to create dashboard metrics recorder %v", err)
		}

		tisClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()
		c := &Reconciler{
			pipelineInformer:   tektonPipelineInformer,
			installerSetClient: client.NewInstallerSetClient(tisClient, operatorVer, dashboardVer, v1alpha1.KindTektonDashboard, metrics),
			operatorClientSet:  operatorclient.Get(ctx),
			extension:          generator(ctx),
			readonlyManifest:   readonlyManifest,
			fullaccessManifest: fullaccessManifest,
			dashboardVersion:   dashboardVer,
			operatorVersion:    operatorVer,
		}
		impl := tektonDashboardreconciler.NewImpl(ctx, c)

		logger.Info("Setting up event handlers for tekton-dashboard")

		tektonDashboardInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonDashboard{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
