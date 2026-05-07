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
	"os"

	"github.com/tektoncd/operator/pkg/reconciler/proxy"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	kwebhook "knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
)

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

	cfg := injection.ParseAndGetRESTConfigOrDie()
	ctx := kwebhook.WithOptions(
		injection.WithNamespaceScope(signals.NewContext(), systemNamespace),
		kwebhook.Options{
			ServiceName: serviceName,
			Port:        8443,
			SecretName:  secretName,
		},
	)

	sharedmain.WebhookMainWithConfig(ctx, "webhook-operator",
		cfg,
		certificates.NewController,
		proxy.NewProxyDefaultingAdmissionController,
	)
}
