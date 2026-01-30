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

package syncerservice

import (
	"context"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	syncerServiceInformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/syncerservice"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineInformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	syncerServiceReconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/syncerservice"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const versionConfigMap = "syncer-service-info"

// NewController initializes the controller and is called by the generated code
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	ctrl := common.Controller{
		Logger:           logger,
		VersionConfigMap: versionConfigMap,
	}

	manifest, syncerVer := ctrl.InitController(ctx, common.PayloadOptions{})
	if syncerVer == common.ReleaseVersionUnknown {
		syncerVer = "devel"
	}

	operatorVer, err := common.OperatorVersion(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	tisClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()

	c := &Reconciler{
		installerSetClient: client.NewInstallerSetClient(tisClient, operatorVer, syncerVer, v1alpha1.KindSyncerService, nil),
		kubeClientSet:      kubeclient.Get(ctx),
		operatorClientSet:  operatorclient.Get(ctx),
		extension:          OpenShiftExtension(ctx),
		manifest:           manifest,
		pipelineInformer:   tektonPipelineInformer.Get(ctx),
		operatorVersion:    operatorVer,
		syncerVersion:      syncerVer,
	}
	impl := syncerServiceReconciler.NewImpl(ctx, c)

	logger.Debug("Setting up event handlers for syncer-service")

	if _, err := syncerServiceInformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue)); err != nil {
		logger.Panicf("Couldn't register SyncerService informer event handler: %w", err)
	}

	if _, err := tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1alpha1.SyncerService{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	}); err != nil {
		logger.Panicf("Couldn't register TektonInstallerSet informer event handler: %w", err)
	}

	return impl
}
