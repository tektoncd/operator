/*
Copyright 2025 The Tekton Authors

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

package tektonpruner

import (
	"context"

	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonPrunerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpruner"
	tektonPrunerreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonpruner"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const versionConfigMap = v1alpha1.TektonPrunerResourceName + "-info"

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return NewExtendedController(common.NoExtension)(ctx, cmw)
}

// NewExtendedController returns a controller extended to a specific platform
func NewExtendedController(generator common.ExtensionGenerator) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)

		ctrl := common.Controller{
			Logger:           logger,
			VersionConfigMap: versionConfigMap,
		}

		manifest, prunerVer := ctrl.InitController(ctx, common.PayloadOptions{})
		operatorVer, err := common.OperatorVersion(ctx)
		if err != nil {
			logger.Fatal(err)
		}

		tisClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()
		metrics, _ := NewRecorder()
		c := &Reconciler{
			kubeClientSet:      kubeclient.Get(ctx),
			pipelineInformer:   tektonPipelineinformer.Get(ctx),
			installerSetClient: client.NewInstallerSetClient(tisClient, operatorVer, prunerVer, v1alpha1.KindTektonPruner, metrics),
			extension:          generator(ctx),
			manifest:           manifest,
			prunerVersion:      prunerVer,
		}
		impl := tektonPrunerreconciler.NewImpl(ctx, c)

		logger.Info("Setting up event handlers for TektonPruner")

		if _, err := tektonPrunerinformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue)); err != nil {
			logger.Panicf("Couldn't register TektonPruner informer event handler: %w", err)
		}

		if _, err := tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonPruner{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		}); err != nil {
			logger.Panicf("Couldn't register TektonInstallerSet informer event handler: %w", err)
		}

		return impl
	}
}
