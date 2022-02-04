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

package tektonaddon

import (
	"context"
	"os"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonAddoninformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonaddon"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	tektonTriggerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektontrigger"
	tektonAddonreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonaddon"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	versionKey = "VERSION"
)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return NewExtendedController(OpenShiftExtension)(ctx, cmw)
}

// NewExtendedController returns a controller extended to a specific platform
func NewExtendedController(generator common.ExtensionGenerator) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
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

		version := os.Getenv(versionKey)
		if version == "" {
			logger.Fatal("Failed to find version from env")
		}

		c := &Reconciler{
			operatorClientSet: operatorclient.Get(ctx),
			extension:         generator(ctx),
			pipelineInformer:  tektonPipelineinformer.Get(ctx),
			triggerInformer:   tektonTriggerinformer.Get(ctx),
			manifest:          manifest,
			operatorVersion:   version,
		}
		impl := tektonAddonreconciler.NewImpl(ctx, c)

		// Add enqueue func in reconciler
		c.enqueueAfter = impl.EnqueueAfter

		logger.Info("Setting up event handlers for TektonAddon")

		tektonAddoninformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonAddon{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
