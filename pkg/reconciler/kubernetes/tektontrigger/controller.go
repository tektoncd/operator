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

package tektontrigger

import (
	"context"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonPipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	tektonTriggerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektontrigger"
	tektonTriggerreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektontrigger"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return NewExtendedController(common.NoExtension)(ctx, cmw)
}

// NewExtendedController returns a controller extended to a specific platform
func NewExtendedController(generator common.ExtensionGenerator) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		tektonPipelineInformer := tektonPipelineinformer.Get(ctx)
		tektonTriggersInformer := tektonTriggerinformer.Get(ctx)
		deploymentInformer := deploymentinformer.Get(ctx)
		kubeClient := kubeclient.Get(ctx)
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

		metrics, err := NewRecorder()
		if err != nil {
			logger.Errorf("Failed to create trigger metrics recorder %v", err)
		}

		c := &Reconciler{
			kubeClientSet:     kubeClient,
			operatorClientSet: operatorclient.Get(ctx),
			extension:         generator(ctx),
			manifest:          manifest,
			pipelineInformer:  tektonPipelineInformer,
			metrics:           metrics,
		}
		impl := tektonTriggerreconciler.NewImpl(ctx, c)

		logger.Info("Setting up event handlers")

		tektonTriggersInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterControllerGVK(v1alpha1.SchemeGroupVersion.WithKind("TektonTrigger")),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
