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

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineInformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	tektonResultInformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonresult"
	tektonResultReconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonresult"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return NewExtendedController(common.NoExtension)(ctx, cmw)
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

		operatorVer, err := common.OperatorVersion(ctx)
		if err != nil {
			logger.Fatal(err)
		}

		if err := common.AppendTarget(ctx, &manifest, &v1alpha1.TektonResult{}); err != nil {
			logger.Fatalw("Error fetching manifests", zap.Error(err))
		}

		c := &Reconciler{
			kubeClientSet:     kubeclient.Get(ctx),
			operatorClientSet: operatorclient.Get(ctx),
			extension:         generator(ctx),
			manifest:          manifest,
			pipelineInformer:  tektonPipelineInformer.Get(ctx),
			operatorVersion:   operatorVer,
		}
		impl := tektonResultReconciler.NewImpl(ctx, c)

		logger.Info("Setting up event handlers for tekton-results")

		tektonResultInformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonResult{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
