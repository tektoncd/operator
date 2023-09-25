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

package namespace

import (
	"context"

	"github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	vwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1/validatingwebhookconfiguration"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
)

func NewNamespaceAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {

	return NewAdmissionController(ctx,

		// Name of the resource webhook.
		"namespace.operator.tekton.dev",

		// The path on which to serve the webhook.
		"/namespace-validation",

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

// NewAdmissionController constructs a reconciler
func NewAdmissionController(
	ctx context.Context,
	name, path string,
	wc func(context.Context) context.Context,
	disallowUnknownFields bool,
) *controller.Impl {

	client := kubeclient.Get(ctx)
	vwhInformer := vwhinformer.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	options := webhook.GetOptions(ctx)
	tektonConfigInformer := tektonconfig.Get(ctx)

	key := types.NamespacedName{Name: name}

	wh := &reconciler{
		LeaderAwareFuncs: pkgreconciler.LeaderAwareFuncs{
			// Have this reconciler enqueue our singleton whenever it becomes leader.
			PromoteFunc: func(bkt pkgreconciler.Bucket, enq func(pkgreconciler.Bucket, types.NamespacedName)) error {
				enq(bkt, key)
				return nil
			},
		},

		key:  key,
		path: path,

		withContext:           wc,
		disallowUnknownFields: disallowUnknownFields,
		secretName:            options.SecretName,

		client:             client,
		vwhlister:          vwhInformer.Lister(),
		secretlister:       secretInformer.Lister(),
		tektonConfigLister: tektonConfigInformer.Lister(),
	}

	logger := logging.FromContext(ctx)
	c := controller.NewContext(ctx, wh, controller.ControllerOptions{WorkQueueName: "NamespaceAdmissionWebhook", Logger: logger})

	// Reconcile when the named ValidatingWebhookConfiguration changes.
	if _, err := vwhInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithName(name),
		// It doesn't matter what we enqueue because we will always Reconcile
		// the named VWH resource.
		Handler: controller.HandleAll(c.Enqueue),
	}); err != nil {
		logger.Panicf("Couldn't register ValidatingWebhookConfugration informer event handler: %w", err)
	}

	// Reconcile when the cert bundle changes.
	if _, err := secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(system.Namespace(), wh.secretName),
		// It doesn't matter what we enqueue because we will always Reconcile
		// the named VWH resource.
		Handler: controller.HandleAll(c.Enqueue),
	}); err != nil {
		logger.Panicf("Couldn't register Secret informer event handler: %w", err)
	}

	return c
}
