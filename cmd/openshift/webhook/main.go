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

package main

import (
	"context"
	"os"

	"github.com/tektoncd/operator/pkg/webhook"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	kwebhook "knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
)

func main() {
	serviceName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "tekton-operator-webhook"
	}

	secretName := os.Getenv("WEBHOOK_SECRET_NAME")
	if secretName == "" {
		secretName = "tekton-operator-webhook-certs"
	}

	//Set up a signal context with our webhook options
	ctx := kwebhook.WithOptions(signals.NewContext(), kwebhook.Options{
		ServiceName: serviceName,
		Port:        8443,
		SecretName:  secretName,
	})
	cfg := injection.ParseAndGetRESTConfigOrDie()
	ctx, _ = injection.EnableInjectionOrDie(ctx, cfg)
	webhook.CreateWebhookResources(ctx)
	webhook.SetTypes("openshift")

	go gracefulTermination(ctx)

	sharedmain.WebhookMainWithConfig(ctx, serviceName,
		cfg,
		certificates.NewController,
		webhook.NewDefaultingAdmissionController,
		webhook.NewValidationAdmissionController,
		webhook.NewConfigValidationController,
	)
}

func gracefulTermination(ctx context.Context) {
	<-ctx.Done()
	webhook.CleanupWebhookResources(ctx)
}
