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

package webhook

import (
	"context"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	v1alpha1.SchemeGroupVersion.WithKind("TektonConfig"):   &v1alpha1.TektonConfig{},
	v1alpha1.SchemeGroupVersion.WithKind("TektonPipeline"): &v1alpha1.TektonPipeline{},
	v1alpha1.SchemeGroupVersion.WithKind("TektonTrigger"):  &v1alpha1.TektonTrigger{},
}

func SetTypes(platform string) {
	if platform == "openshift" {
		types[v1alpha1.SchemeGroupVersion.WithKind("TektonAddon")] = &v1alpha1.TektonAddon{}
	} else {
		types[v1alpha1.SchemeGroupVersion.WithKind("TektonDashboard")] = &v1alpha1.TektonDashboard{}
	}
}

func NewDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	name := findAndUpdateMutatingWebhookConfigurationNameOrDie(ctx, "webhook.operator.tekton.dev")
	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		name,
		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to validate and default.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func NewValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	name := findAndUpdateValidatingWebhookConfigurationNameOrDie(ctx, "validation.webhook.operator.tekton.dev")
	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		name,

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate and default.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func NewConfigValidationController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	name := findAndUpdateValidatingWebhookConfigurationNameOrDie(ctx, "config.webhook.operator.tekton.dev")
	return configmaps.NewAdmissionController(ctx,
		// Name of the configmap webhook.
		name,

		// The path on which to serve the webhook.
		"/config-validation",

		configmap.Constructors{
			logging.ConfigMapName(): logging.NewConfigFromConfigMap,
		},
	)
}

func findAndUpdateMutatingWebhookConfigurationNameOrDie(ctx context.Context, namePrefix string) string {
	logger := logging.FromContext(ctx)
	kubeClientSet := kubeclient.Get(ctx)

	mutatingWebhookConfigurations, err := kubeClientSet.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error(err)
		logger.Fatal("MutatingWebhookConfiguration with prefix ", namePrefix, " not found")
		return ""
	}

	// Find the mutatingWebhookConfiguration with the given generateName prefix
	var mutatingWebhookConfiguration *v1.MutatingWebhookConfiguration
	for _, item := range mutatingWebhookConfigurations.Items {
		if strings.HasPrefix(item.Name, namePrefix) {
			mutatingWebhookConfiguration = &item
			break
		}
	}
	if mutatingWebhookConfiguration == nil {
		logger.Fatal("MutatingWebhookConfiguration with prefix ", namePrefix, " not found")
		return ""
	}
	webhookName := mutatingWebhookConfiguration.Name

	// Update the webhooks[*].name field with the generated Name (metadata.name) of the mutatingWebhookConfiguration
	for i := range mutatingWebhookConfiguration.Webhooks {
		mutatingWebhookConfiguration.Webhooks[i].Name = webhookName
	}
	_, err = kubeClientSet.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(ctx, mutatingWebhookConfiguration, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err)
		logger.Fatal("Could not update MutatingWebhookConfiguration ", webhookName)
		return ""
	}

	return webhookName
}

func findAndUpdateValidatingWebhookConfigurationNameOrDie(ctx context.Context, namePrefix string) string {
	logger := logging.FromContext(ctx)
	kubeClientSet := kubeclient.Get(ctx)

	validatingWebhookConfigurations, err := kubeClientSet.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error(err)
		logger.Fatal("ValidatingWebhookConfiguration with prefix ", namePrefix, " not found")
		return ""
	}

	// Find the validatingWebhookConfiguration with the given generateName prefix
	var validatingWebhookConfiguration *v1.ValidatingWebhookConfiguration
	for _, item := range validatingWebhookConfigurations.Items {
		if strings.HasPrefix(item.Name, namePrefix) {
			validatingWebhookConfiguration = &item
			break
		}
	}
	if validatingWebhookConfiguration == nil {
		logger.Fatal("ValidatingWebhookConfiguration with prefix ", namePrefix, " not found")
		return ""
	}
	webhookName := validatingWebhookConfiguration.Name

	// Update the webhooks[*].name field with the generated Name (metadata.name) of the validatingWebhookConfiguration
	for i := range validatingWebhookConfiguration.Webhooks {
		validatingWebhookConfiguration.Webhooks[i].Name = webhookName
	}
	_, err = kubeClientSet.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(ctx, validatingWebhookConfiguration, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err)
		logger.Fatal("Could not update ValidatingWebhookConfiguration ", webhookName)
		return ""
	}
	return webhookName
}
