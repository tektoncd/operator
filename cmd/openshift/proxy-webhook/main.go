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

package main

import (
	"context"

	"github.com/tektoncd/operator/pkg/reconciler/openshift/annotation"
	"github.com/tektoncd/operator/pkg/reconciler/proxy"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/webhook/certificates"
)

func newAnnotationDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {

	return annotation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"annotation.operator.tekton.dev",

		// The path on which to serve the webhook.
		"/annotation-defaulting",

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func main() {
	sharedmain.WebhookMainWithConfig(proxy.Getctx(), "webhook-operator",
		injection.ParseAndGetRESTConfigOrDie(),
		certificates.NewController,
		proxy.NewProxyDefaultingAdmissionController,
		newAnnotationDefaultingAdmissionController,
	)
}
