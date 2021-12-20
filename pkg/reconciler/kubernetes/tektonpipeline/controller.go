/*
Copyright 2019 The Tekton Authors

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

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineInformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	tektonPipelineReconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonpipeline"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/initcontroller"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const versionConfigMap = "pipelines-info"

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return NewExtendedController(common.NoExtension)(ctx, cmw)
}

// NewExtendedController returns a controller extended to a specific platform
func NewExtendedController(generator common.ExtensionGenerator) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)

		ctrl := initcontroller.Controller{
			Logger:           logger,
			VersionConfigMap: versionConfigMap,
		}

		manifest, operatorRV, pipelinesRV := ctrl.InitController(ctx, initcontroller.PayloadOptions{})

		metrics, err := NewRecorder()
		if err != nil {
			logger.Errorf("Failed to create pipeline metrics recorder %v", err)
		}

		c := &Reconciler{
			operatorClientSet:       operatorclient.Get(ctx),
			extension:               generator(ctx),
			manifest:                manifest,
			pipelinesReleaseVersion: pipelinesRV,
			operatorReleaseVersion:  operatorRV,
			metrics:                 metrics,
		}
		impl := tektonPipelineReconciler.NewImpl(ctx, c)

		// Add enqueue func in reconciler
		c.enqueueAfter = impl.EnqueueAfter

		logger.Info("Setting up event handlers for TektonPipeline")

		tektonPipelineInformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonPipeline{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
