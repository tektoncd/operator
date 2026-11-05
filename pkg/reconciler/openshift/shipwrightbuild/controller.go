/*
Copyright 2026 The Tekton Authors

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

// Package shipwrightbuild installs the Shipwright Build whose desired state is expressed through the
// ShipwrightBuild CRD.

package shipwrightbuild

import (
	"context"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/shipwrightbuild"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	impl := shipwrightbuild.NewExtendedController(NewOpenShiftExtension)(ctx, cmw)
	// Legacy Deployments are not owned by our InstallerSets. Watch them
	// explicitly so OLM restarts and shutdown progress trigger reconciliation.
	if _, err := deploymentinformer.Get(ctx).Informer().AddEventHandler(legacyDeploymentEventHandler(impl.EnqueueKey)); err != nil {
		logging.FromContext(ctx).Panicf("Couldn't register legacy Builds Deployment informer event handler: %w", err)
	}
	return impl
}

func legacyDeploymentEventHandler(enqueue func(types.NamespacedName)) cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			deployment, ok := obj.(*appsv1.Deployment)
			return ok && deployment.Namespace == legacyNamespace &&
				(deployment.Name == legacyOperatorDeployment || deployment.Name == shipwrightbuild.ControllerDeploymentName || deployment.Name == shipwrightbuild.WebhookDeploymentName)
		},
		Handler: controller.HandleAll(func(interface{}) {
			enqueue(types.NamespacedName{Name: v1alpha1.ShipwrightBuildResourceName})
		}),
	}
}
