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

	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"github.com/tektoncd/operator/pkg/webhook"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
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

	cfg := injection.ParseAndGetRESTConfigOrDie()
	ctx := signals.NewContext()
	ctx, _ = injection.EnableInjectionOrDie(ctx, cfg)

	logger := logging.FromContext(ctx)

	// Observe TLS configuration from OpenShift APIServer if feature is enabled
	webhookOpts := kwebhook.Options{
		ServiceName: serviceName,
		Port:        8443,
		SecretName:  secretName,
	}

	if occommon.IsCentralTLSConfigEnabled() {
		logger.Info("Central TLS config is enabled for webhook, observing APIServer TLS profile")

		// Observe TLS config (stores in context)
		ctx = occommon.ObserveAndStoreTLSConfig(ctx, cfg)

		// Get TLS config from context
		if tlsConfig := occommon.GetTLSConfigFromContext(ctx); tlsConfig != nil {
			// Only set MinVersion (not cipher suites or curves) to avoid knative version bump
			webhookOpts.TLSMinVersion = tlsConfig.MinVersion
			logger.Infof("Webhook TLS min version set to: %s", occommon.TLSVersionToString(tlsConfig.MinVersion))
		} else {
			logger.Warn("Central TLS config enabled but TLS config not available from context")
		}
	} else {
		logger.Info("Central TLS config is disabled for webhook")
	}

	// Set up context with webhook options
	ctx = kwebhook.WithOptions(ctx, webhookOpts)

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
