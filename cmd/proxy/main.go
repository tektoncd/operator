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
	"fmt"
	"os"
	"strings"

	"github.com/tektoncd/operator/pkg/reconciler/proxy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
)

func newProxyDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {

	webhook, err := getWebhookName(ctx)
	if err != nil {
		webhook = "proxy.operator.tekton.dev"
	}

	return proxy.NewAdmissionController(ctx,

		// Name of the resource webhook.
		webhook,

		// The path on which to serve the webhook.
		"/defaulting",

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func main() {
	serviceName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "tekton-operator-proxy-webhook"
	}

	secretName := os.Getenv("WEBHOOK_SECRET_NAME")
	if secretName == "" {
		secretName = "proxy-webhook-certs"
	}
	systemNamespace := os.Getenv("SYSTEM_NAMESPACE")

	// Scope informers to the webhook's namespace instead of cluster-wide
	ctx := injection.WithNamespaceScope(signals.NewContext(), systemNamespace)

	// Set up a signal context with our webhook options
	ctx = webhook.WithOptions(ctx, webhook.Options{
		ServiceName: serviceName,
		Port:        8443,
		SecretName:  secretName,
	})

	sharedmain.WebhookMainWithConfig(ctx, "webhook-operator",
		sharedmain.ParseAndGetConfigOrDie(),
		certificates.NewController,
		newProxyDefaultingAdmissionController,
	)
}

func getWebhookName(ctx context.Context) (string, error) {
	client := kubeclient.Get(ctx)
	webhookList, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, w := range webhookList.Items {
		if strings.HasPrefix(w.Name, "proxy.operator.tekton.dev") {
			return w.Name, nil
		}
	}
	return "", fmt.Errorf("not able to find any webhook for proxy")
}
