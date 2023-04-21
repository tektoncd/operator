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

package tektonconfig

import (
	"context"
	"os"
	"regexp"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonChaininformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonchain"
	tektonConfiginformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
	tektonInstallerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektoninstallerset"
	tektonPipelineinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonpipeline"
	tektonTriggerinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektontrigger"
	tektonConfigreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	namespaceinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
)

// NewExtensibleController returns a controller extended to a specific platform
func NewExtensibleController(generator common.ExtensionGenerator) injection.ControllerConstructor {
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

		c := &Reconciler{
			kubeClientSet:     kubeclient.Get(ctx),
			operatorClientSet: operatorclient.Get(ctx),
			extension:         generator(ctx),
			manifest:          manifest,
			operatorVersion:   operatorVer,
		}
		impl := tektonConfigreconciler.NewImpl(ctx, c)

		logger.Info("Setting up event handlers for TektonConfig")

		tektonConfiginformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		tektonPipelineinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonConfig{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		tektonTriggerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonConfig{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		tektonChaininformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonConfig{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		tektonInstallerinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterController(&v1alpha1.TektonConfig{}),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		namespaceinformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(enqueueCustomName(impl, v1alpha1.ConfigResourceName)))

		if os.Getenv("AUTOINSTALL_COMPONENTS") == "true" {
			// try to ensure that there is an instance of tektonConfig
			newTektonConfig(operatorclient.Get(ctx), kubeclient.Get(ctx)).ensureInstance(ctx)
		}

		return impl
	}
}

// enqueueCustomName adds an event with name `config` in work queue so that
// whenever a namespace event occurs, the TektonConfig reconciler get triggered.
// This is required because we want to get our TektonConfig reconciler triggered
// for already existing and new namespaces, without manual intervention like adding
// a label/annotation on namespace to make it manageable by Tekton controller.
// This will also filter the namespaces by regex `^(openshift|kube)-`
// and enqueue only when namespace doesn't match the regex
func enqueueCustomName(impl *controller.Impl, name string) func(obj interface{}) {
	return func(obj interface{}) {
		var nsRegex = regexp.MustCompile(common.NamespaceIgnorePattern)
		object, err := kmeta.DeletionHandlingAccessor(obj)
		if err == nil && !nsRegex.MatchString(object.GetName()) {
			impl.EnqueueKey(types.NamespacedName{Namespace: "", Name: name})
		}
	}
}
