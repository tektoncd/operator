/*
Copyright 2021 The Tekton Authors

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

package tektonresult

import (
	"context"

	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineInformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	tektonResultInformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonresult"
	tektonResultReconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonresult"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const versionConfigMap = "tekton-results-info"

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
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

		manifest, resultsVer := ctrl.InitController(ctx, common.PayloadOptions{})
		if resultsVer == common.ReleaseVersionUnknown {
			resultsVer = "devel"
		}

		operatorVer, err := common.OperatorVersion(ctx)
		if err != nil {
			logger.Fatal(err)
		}

		recorder, err := NewRecorder()
		if err != nil {
			logger.Fatalw("Error starting Results metrics")
		}

		metricsWrapper := NewRecorderWrapper(recorder)

		tisClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()

		c := &Reconciler{
			installerSetClient: client.NewInstallerSetClient(tisClient, operatorVer, resultsVer, v1alpha1.KindTektonResult, metricsWrapper),
			kubeClientSet:      kubeclient.Get(ctx),
			operatorClientSet:  operatorclient.Get(ctx),
			extension:          generator(ctx),
			manifest:           &manifest,
			pipelineInformer:   tektonPipelineInformer.Get(ctx),
			operatorVersion:    operatorVer,
			resultsVersion:     resultsVer,
			recorder:           recorder,
		}
		impl := tektonResultReconciler.NewImpl(ctx, c)

		logger.Debug("Setting up event handlers for tekton-results")

		if _, err := tektonResultInformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue)); err != nil {
			logger.Panicf("Couldn't register TektonResult informer event handler: %w", err)
		}

		if _, err := tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonResult{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		}); err != nil {
			logger.Panicf("Couldn't register TektonInstallerSet informer event handler: %w", err)
		}

		return impl
	}
}
