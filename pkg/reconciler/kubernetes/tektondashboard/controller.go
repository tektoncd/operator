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
	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonDashboardinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektondashboard"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	tektonDashboardreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektondashboard"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
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
		tektonDashboardInformer := tektonDashboardinformer.Get(ctx)
		kubeClient := kubeclient.Get(ctx)
		logger := logging.FromContext(ctx)

		mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
		if err != nil {
			logger.Fatalw("Error creating client from injected config", zap.Error(err))
		}
		mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())

		readonlyManifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
		if err != nil {
			logger.Fatalw("Error creating initial manifest", zap.Error(err))
		}

		// Reads the source manifest from kodata while initializing the contoller
		if err := fetchSourceReadOnlyManifests(ctx, &readonlyManifest); err != nil {
			logger.Fatalw("failed to read manifest", err)
		}

		fullaccessManifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
		if err != nil {
			logger.Fatalw("Error creating initial manifest", zap.Error(err))
		}

		// Reads the source manifest from kodata while initializing the contoller
		if err := fetchSourceFullAccessManifests(context.TODO(), &fullaccessManifest); err != nil {
			logger.Fatalw("failed to read manifest", err)
		}

		c := &Reconciler{
			kubeClientSet:      kubeClient,
			operatorClientSet:  operatorclient.Get(ctx),
			extension:          generator(ctx),
			readonlyManifest:   readonlyManifest,
			fullaccessManifest: fullaccessManifest,
			pipelineInformer:   tektonPipelineInformer,
		}
		impl := tektonDashboardreconciler.NewImpl(ctx, c)

		// Add enqueue func in reconciler
		c.enqueueAfter = impl.EnqueueAfter

		logger.Info("Setting up event handlers for tekton-dashboard")

		tektonDashboardInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonDashboard{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}

// fetchSourceReadOnlyManifests mutates the passed manifest by appending one
// appropriate for the passed TektonComponent with readonly value set to true
func fetchSourceReadOnlyManifests(ctx context.Context, manifest *mf.Manifest) error {
	var dashboard v1alpha1.TektonDashboard
	dashboard.Spec.Readonly = true
	return common.AppendTarget(ctx, manifest, &dashboard)
}

// fetchSourceFullAccessManifests mutates the passed manifest by appending one
// appropriate for the passed TektonComponent with readonly value set to false
func fetchSourceFullAccessManifests(ctx context.Context, manifest *mf.Manifest) error {
	var dashboard v1alpha1.TektonDashboard
	dashboard.Spec.Readonly = false
	return common.AppendTarget(ctx, manifest, &dashboard)
}
