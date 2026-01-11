/*
Copyright 2023 The Tekton Authors

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

	tektonResultInformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonresult"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	k8s_ctrl "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonresult"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return NewExtendedController(OpenShiftExtension)(ctx, cmw)
}

// NewExtendedController wraps the base Kubernetes controller and adds OpenShift-specific watches
func NewExtendedController(generator common.ExtensionGenerator) func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)

		// Create the base Kubernetes controller with OpenShift extension
		impl := k8s_ctrl.NewExtendedController(generator)(ctx, cmw)

		// Setup OpenShift APIServer watch for TLS profile changes.
		// This will trigger reconciliation when the cluster TLS policy changes.
		// Errors are logged by SetupAPIServerTLSWatch; we don't fail controller startup.
		restConfig := injection.GetConfig(ctx)
		lister := tektonResultInformer.Get(ctx).Lister()
		listerAdapter := occommon.TektonResultListerAdapter{Lister: lister}

		if err := occommon.SetupAPIServerTLSWatch(ctx, restConfig, impl, listerAdapter, "TektonResult"); err == nil {
			logger.Info("APIServer TLS profile watch enabled")
		}

		return impl
	}
}
