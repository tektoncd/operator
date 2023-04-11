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

package tektoninstallerset

import (
	"context"

	mfc "github.com/manifestival/client-go-client"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonInstallerReconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektoninstallerset"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	serviceAccountInformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount"
	clusterRoleInformer "knative.dev/pkg/client/injection/kube/informers/rbac/v1/clusterrole"
	clusterRoleBindingInformer "knative.dev/pkg/client/injection/kube/informers/rbac/v1/clusterrolebinding"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return NewExtendedController()(ctx, cmw)
}

// NewExtendedController returns a controller extended to a specific platform
func NewExtendedController() injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)

		mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
		if err != nil {
			logger.Fatalw("Error creating client from injected config", zap.Error(err))
		}

		c := &Reconciler{
			operatorClientSet: operatorclient.Get(ctx),
			mfClient:          mfclient,
		}
		impl := tektonInstallerReconciler.NewImpl(ctx, c)

		logger.Info("Setting up event handlers for TektonInstallerSet")

		tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		deploymentinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonInstallerSet{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		clusterRoleBindingInformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonInstallerSet{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		clusterRoleInformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonInstallerSet{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		serviceAccountInformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonInstallerSet{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
