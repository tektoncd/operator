/*
Copyright 2022 The Tekton Authors

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

package tektonchains

import (
	"context"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonChainsinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonchains"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	tektonChainsreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonchains"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"k8s.io/client-go/tools/cache"
	//kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// chains-info stores the version of Chains installed in the cluster
const versionConfigMap = "chains-info"

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

		manifest, releaseVersion := ctrl.InitController(ctx, common.PayloadOptions{})
		if releaseVersion == common.ReleaseVersionUnknown {
			releaseVersion = "devel"
		}

		c := &Reconciler{
			operatorClientSet: operatorclient.Get(ctx),
			extension:         generator(ctx),
			manifest:          manifest,
			pipelineInformer:  tektonPipelineinformer.Get(ctx),
			releaseVersion:    releaseVersion,
		}
		impl := tektonChainsreconciler.NewImpl(ctx, c)

		// Add enqueue func in reconciler
		c.enqueueAfter = impl.EnqueueAfter

		logger.Info("Setting up event handlers for Tekton Chains")

		tektonChainsinformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonChains{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
