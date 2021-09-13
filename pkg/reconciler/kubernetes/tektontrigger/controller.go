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

package tektontrigger

import (
	"context"
	"fmt"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonTriggerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektontrigger"
	tektonTriggerreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektontrigger"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const releaseLabel = "triggers.tekton.dev/release"

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
		mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
		manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
		if err != nil {
			logger.Fatalw("Error creating initial manifest", zap.Error(err))
		}

		// Reads the source manifest from kodata while initializing the contoller
		if err := fetchSourceManifests(context.TODO(), &manifest); err != nil {
			logger.Fatalw("failed to read manifest", err)
		}

		// Read the release version of pipelines
		releaseVersion, err := fetchVersion(manifest)
		if err != nil {
			logger.Fatalw("failed to read release version from manifest", err)
		}

		c := &Reconciler{
			operatorClientSet: operatorclient.Get(ctx),
			pipelineInformer:  tektonPipelineinformer.Get(ctx),
			extension:         generator(ctx),
			manifest:          manifest,
			releaseVersion:    releaseVersion,
		}
		impl := tektonTriggerreconciler.NewImpl(ctx, c)

		// Add enqueue func in reconciler
		c.enqueueAfter = impl.EnqueueAfter

		logger.Info("Setting up event handlers for TektonTrigger")

		tektonTriggerinformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonTrigger{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}

// fetchSourceManifests mutates the passed manifest by appending one
// appropriate for the passed TektonComponent
func fetchSourceManifests(ctx context.Context, manifest *mf.Manifest) error {
	var trigger *v1alpha1.TektonTrigger
	return common.AppendTarget(ctx, manifest, trigger)
}

func fetchVersion(manifest mf.Manifest) (string, error) {
	crds := manifest.Filter(mf.CRDs)
	if len(crds.Resources()) == 0 {
		return "", fmt.Errorf("failed to find crds to get release version")
	}

	crd := crds.Resources()[0]
	version, ok := crd.GetLabels()[releaseLabel]
	if !ok {
		return version, fmt.Errorf("failed to find release label on crd")
	}

	return version, nil
}
