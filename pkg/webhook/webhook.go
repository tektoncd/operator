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

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.KindTektonConfig):   &v1alpha1.TektonConfig{},
	v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.KindTektonPipeline): &v1alpha1.TektonPipeline{},
	v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.KindTektonTrigger):  &v1alpha1.TektonTrigger{},
	v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.KindTektonHub):      &v1alpha1.TektonHub{},
	v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.KindTektonResult):   &v1alpha1.TektonResult{},
	v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.KindTektonChain):    &v1alpha1.TektonChain{},
}

func SetTypes(platform string) {
	if platform == "openshift" {
		types[v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.KindTektonAddon)] = &v1alpha1.TektonAddon{}
		types[v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.KindOpenShiftPipelinesAsCode)] = &v1alpha1.OpenShiftPipelinesAsCode{}
	} else {
		types[v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.KindTektonDashboard)] = &v1alpha1.TektonDashboard{}
	}
}

func NewDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"webhook.operator.tekton.dev",
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
	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"validation.webhook.operator.tekton.dev",

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
	return configmaps.NewAdmissionController(ctx,
		// Name of the configmap webhook.
		"config.webhook.operator.tekton.dev",

		// The path on which to serve the webhook.
		"/config-validation",

		configmap.Constructors{
			logging.ConfigMapName(): logging.NewConfigFromConfigMap,
		},
	)
}
