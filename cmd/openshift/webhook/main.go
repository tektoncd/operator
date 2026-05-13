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
	"log"
	"os"

	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
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

	// cfg is obtained before signals.NewContext so it's available for the TLS watch setup.
	cfg := injection.ParseAndGetRESTConfigOrDie()
	signalCtx := signals.NewContext()

	// Set up the APIServer TLS watch using the same helper as the TektonConfig controller.
	// - Populates the shared lister so GetTLSProfileFromAPIServer works below.
	// - Calls os.Exit(1) when the TLS profile changes; Kubernetes restartPolicy: Always
	//   restarts the container so the new instance picks up the updated profile.
	if err := occommon.SetupAPIServerTLSWatch(signalCtx, cfg, func() {
		log.Println("APIServer TLS profile changed — restarting webhook to apply updated settings")
		os.Exit(1)
	}); err != nil {
		// On OpenShift clusters the APIServer resource should always exist.
		// SKIP_APISERVER_TLS_WATCH=true is an escape hatch for tests/edge cases —
		// same pattern as the TektonConfig controller.
		if os.Getenv(occommon.SkipAPIServerTLSWatch) == "true" {
			log.Printf("WARNING: APIServer TLS watch not available, using Knative defaults: %v", err)
		} else {
			log.Fatalf("Failed to set up APIServer TLS watch: %v", err)
		}
	}

	// Read the current TLS profile and inject as WEBHOOK_TLS_* env vars.
	// Knative's DefaultConfigFromEnv("WEBHOOK_") inside webhook.New() reads these automatically.
	// TLSEnvVarsFromProfile produces "1.2"/"1.3" and comma-separated IANA cipher names —
	// exactly the format Knative expects.
	if tlsProfile, err := occommon.GetTLSProfileFromAPIServer(signalCtx); err != nil {
		log.Printf("WARNING: could not read APIServer TLS profile, using Knative defaults: %v", err)
	} else if tlsProfile != nil {
		if envVars, err := occommon.TLSEnvVarsFromProfile(tlsProfile); err != nil {
			log.Printf("WARNING: could not convert TLS profile, using Knative defaults: %v", err)
		} else if envVars != nil {
			// Knative only accepts "1.2" or "1.3"; skip if the profile allows older versions
			// (e.g. OpenShift "Old" profile uses VersionTLS10). The webhook will then fall
			// back to Knative's default minimum of 1.2, which is always safe for admission webhooks.
			if envVars.MinVersion == "1.2" || envVars.MinVersion == "1.3" {
				os.Setenv("WEBHOOK_TLS_MIN_VERSION", envVars.MinVersion)
			}
			if envVars.CipherSuites != "" {
				os.Setenv("WEBHOOK_TLS_CIPHER_SUITES", envVars.CipherSuites)
			}
			if envVars.CurvePreferences != "" {
				os.Setenv("WEBHOOK_TLS_CURVE_PREFERENCES", envVars.CurvePreferences)
			}
		}
	}

	// kwebhook.Options is unchanged — no TLS fields needed.
	// Knative reads the WEBHOOK_TLS_* env vars we just set inside webhook.New().
	ctx := kwebhook.WithOptions(signalCtx, kwebhook.Options{
		ServiceName: serviceName,
		Port:        8443,
		SecretName:  secretName,
	})
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
