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
	"log"
	"os"

	"github.com/tektoncd/operator/pkg/reconciler/openshift/annotation"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/namespace"
	"github.com/tektoncd/operator/pkg/reconciler/proxy"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	kwebhook "knative.dev/pkg/webhook"
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
	serviceName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "tekton-operator-proxy-webhook"
	}
	secretName := os.Getenv("WEBHOOK_SECRET_NAME")
	if secretName == "" {
		secretName = "proxy-webhook-certs"
	}

	cfg := injection.ParseAndGetRESTConfigOrDie()
	signalCtx := signals.NewContext()

	if err := occommon.SetupAPIServerTLSWatch(signalCtx, cfg, func() {
		log.Println("APIServer TLS profile changed — restarting proxy webhook to apply updated settings")
		os.Exit(1)
	}); err != nil {
		if os.Getenv(occommon.SkipAPIServerTLSWatch) == "true" {
			log.Printf("WARNING: APIServer TLS watch not available, using Knative defaults: %v", err)
		} else {
			log.Fatalf("Failed to set up APIServer TLS watch: %v", err)
		}
	}

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

	// Inline the context setup (same as proxy.Getctx but reuses the signal
	// context already created above instead of calling signals.NewContext again).
	systemNamespace := os.Getenv("SYSTEM_NAMESPACE")
	ctx := kwebhook.WithOptions(
		injection.WithNamespaceScope(signalCtx, systemNamespace),
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
		newAnnotationDefaultingAdmissionController,
		namespace.NewNamespaceAdmissionController,
	)
}
