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

package tektonaddon

import (
	"context"
	"os"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonAddoninformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonaddon"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	tektonTriggerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektontrigger"
	tektonAddonreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonaddon"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"go.uber.org/zap"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	versionKey = "VERSION"
)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
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
		crdClient, err := apiextensionsclient.NewForConfig(injection.GetConfig(ctx))
		if err != nil {
			logger.Fatalw("Error creating client from injected config", zap.Error(err))
		}
		mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
		manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
		if err != nil {
			logger.Fatalw("Error creating initial manifest", zap.Error(err))
		}

		version := os.Getenv(versionKey)
		if version == "" {
			logger.Fatal("Failed to find version from env")
		}

		tisClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()
		metrics, _ := NewRecorder()

		resolverTaskManifest := &mf.Manifest{}
		if err := applyAddons(resolverTaskManifest, "06-ecosystem/tasks"); err != nil {
			logger.Fatalf("failed to read namespaced tasks from kodata: %v", err)
		}

		resolverStepActionManifest := &mf.Manifest{}
		if err := applyAddons(resolverStepActionManifest, "06-ecosystem/stepactions"); err != nil {
			logger.Fatalf("failed to read namespaced stepactions from kodata: %v", err)
		}

		triggersResourcesManifest := &mf.Manifest{}
		if err := applyAddons(triggersResourcesManifest, "01-clustertriggerbindings"); err != nil {
			logger.Fatalf("failed to read trigger Resources from kodata: %v", err)
		}

		pipelineTemplateManifest := &mf.Manifest{}
		if err := applyAddons(pipelineTemplateManifest, "02-pipelines"); err != nil {
			logger.Fatalf("failed to read pipeline template from kodata: %v", err)
		}
		if err := addPipelineTemplates(pipelineTemplateManifest); err != nil {
			logger.Fatalf("failed to add pipeline templates: %v", err)
		}

		openShiftConsoleManifest := &mf.Manifest{Client: mfclient}
		if err := applyAddons(openShiftConsoleManifest, "04-tkncliserve"); err != nil {
			logger.Fatalf("failed to read openshift console resources from kodata: %v", err)
		}
		if err := getOptionalAddons(openShiftConsoleManifest); err != nil {
			logger.Fatalf("failed to read optional addon resources from kodata: %v", err)
		}

		consoleCLIManifest := &mf.Manifest{}
		if err := applyAddons(consoleCLIManifest, "03-consolecli"); err != nil {
			logger.Fatalf("failed to read console cli from kodata: %v", err)
		}

		communityResolverTaskManifest := &mf.Manifest{}
		if err := fetchCommunityResolverTasks(communityResolverTaskManifest); err != nil {
			// if unable to fetch community task, don't fail
			logger.Errorf("failed to read community resolver task: %v", err)
		}

		c := &Reconciler{
			crdClientSet:                  crdClient,
			installerSetClient:            client.NewInstallerSetClient(tisClient, version, "addon", v1alpha1.KindTektonAddon, metrics),
			operatorClientSet:             operatorclient.Get(ctx),
			extension:                     generator(ctx),
			pipelineInformer:              tektonPipelineinformer.Get(ctx),
			triggerInformer:               tektonTriggerinformer.Get(ctx),
			manifest:                      manifest,
			operatorVersion:               version,
			resolverTaskManifest:          resolverTaskManifest,
			resolverStepActionManifest:    resolverStepActionManifest,
			triggersResourcesManifest:     triggersResourcesManifest,
			pipelineTemplateManifest:      pipelineTemplateManifest,
			openShiftConsoleManifest:      openShiftConsoleManifest,
			consoleCLIManifest:            consoleCLIManifest,
			communityResolverTaskManifest: communityResolverTaskManifest,
		}
		impl := tektonAddonreconciler.NewImpl(ctx, c)

		logger.Debug("Setting up event handlers for TektonAddon")

		if _, err := tektonAddoninformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue)); err != nil {
			logger.Panicf("Couldn't register TektonAddon informer event handler: %w", err)
		}

		if _, err := tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonAddon{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		}); err != nil {
			logger.Panicf("Couldn't register TektonInstallerSet informer event handler: %w", err)
		}

		return impl
	}
}
