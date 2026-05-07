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

package tektonconfig

import (
	"context"
	"os"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	openshiftpipelinesascodeinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/openshiftpipelinesascode"
	tektonAddoninformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonaddon"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)
	ctrl := tektonconfig.NewExtensibleController(OpenShiftExtension)(ctx, cmw)
	if _, err := tektonAddoninformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1alpha1.TektonConfig{}),
		Handler:    controller.HandleAll(ctrl.EnqueueControllerOf),
	}); err != nil {
		logger.Panicf("Couldn't register TektonAddon informer event handler: %w", err)
	}
	if _, err := openshiftpipelinesascodeinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1alpha1.TektonConfig{}),
		Handler:    controller.HandleAll(ctrl.EnqueueControllerOf),
	}); err != nil {
		logger.Panicf("Couldn't register OpenShiftPipelinesAsCode informer event handler: %w", err)
	}

	// Setup APIServer TLS profile watcher
	// When the cluster's TLS security profile changes, enqueue TektonConfig for reconciliation
	if err := setupAPIServerTLSWatch(ctx, ctrl); err != nil {
		// On OpenShift clusters the APIServer resource should always exist.
		// This env var is an escape hatch for edge cases and must be explicitly enabled.
		if os.Getenv(occommon.SkipAPIServerTLSWatch) == "true" {
			logger.Warnf("APIServer TLS profile watch not enabled: %v", err)
		} else {
			logger.Panicf("Couldn't setup APIServer TLS profile watch: %v", err)
		}
	}

	return ctrl
}

// setupAPIServerTLSWatch sets up a watch on the OpenShift APIServer resource to
// monitor TLS security profile changes. When changes are detected, it enqueues
// TektonConfig for reconciliation so TLS config can be propagated to components.
func setupAPIServerTLSWatch(ctx context.Context, impl *controller.Impl) error {
	logger := logging.FromContext(ctx)
	return occommon.SetupAPIServerTLSWatch(ctx, injection.GetConfig(ctx), func() {
		logger.Info("APIServer TLS security profile changed, triggering TektonConfig reconciliation")
		impl.EnqueueKey(types.NamespacedName{Name: v1alpha1.ConfigResourceName})
	})
}
