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

// Package sharedresource installs the Shared Resource component whose desired
// state is expressed through the SharedResource CRD. It is an OpenShift-only
// component sourced from the upstream openshift-builds operator.

package sharedresource

import (
	"context"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	sharedresourceinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/sharedresource"
	tektoninstallersetinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonpipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	"github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/sharedresource"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	tektoninstallersetclient "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return NewExtendedController(NewOpenShiftExtension)(ctx, cmw)
}

// NewExtendedController returns a controller extended to a specific platform
func NewExtendedController(generator common.ExtensionGenerator) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)

		operatorVersion, err := common.OperatorVersion(ctx)
		if err != nil {
			logger.Fatal(err)
		}

		c := common.Controller{
			Logger:           logger,
			VersionConfigMap: VersionConfigMap,
		}

		manifest, componentVersion := c.InitController(ctx, common.PayloadOptions{})
		if componentVersion == common.ReleaseVersionUnknown {
			componentVersion = "devel"
		}

		metrics, err := common.NoMetrics()
		if err != nil {
			logger.Errorf("Failed to create SharedResource metrics recorder %v", err)
		}

		params := networkpolicy.KubernetesPlatformDefaults()
		if v1alpha1.IsOpenShiftPlatform() {
			params = networkpolicy.OpenShiftPlatformDefaults()
		}

		tektonInstallerSetInterface := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()

		r := &Reconciler{
			extension:              generator(ctx),
			installerSetClient:     tektoninstallersetclient.NewInstallerSetClient(tektonInstallerSetInterface, operatorVersion, componentVersion, v1alpha1.KindSharedResource, metrics),
			operatorClientset:      operatorclient.Get(ctx),
			operatorVersion:        operatorVersion,
			platformParams:         params,
			releaseManifest:        &manifest,
			componentVersion:       componentVersion,
			tektonPipelineInformer: tektonpipelineinformer.Get(ctx),
		}

		impl := sharedresource.NewImpl(ctx, r)

		if _, err := sharedresourceinformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue)); err != nil {
			logger.Panicf("Couldn't register SharedResource informer event handler: %w", err)
		}

		if _, err := tektoninstallersetinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.SharedResource{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		}); err != nil {
			logger.Panicf("Couldn't register TektonInstallerSet informer event handler: %w", err)
		}

		return impl
	}
}
