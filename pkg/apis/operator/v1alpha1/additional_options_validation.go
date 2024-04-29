/*
Copyright 2024 The Tekton Authors

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

package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var (
	validatePipelineWebhookConfigurationFailurePolicy = sets.NewString("Ignore", "Fail")
	validatePipelineWebhookConfigurationSideEffects   = sets.NewString("NoneOnDryRun", "None", "Unknown", "Some")
)

func (w *WebhookConfigurationOptions) validate(path string) (errs *apis.FieldError) {
	if w.FailurePolicy != nil && !validatePipelineWebhookConfigurationFailurePolicy.Has(string(*w.FailurePolicy)) {
		errs = errs.Also(apis.ErrInvalidValue(*w.FailurePolicy, fmt.Sprintf("%s.webhookconfigurationoptions.failurePolicy", path)))
	}
	if w.SideEffects != nil && !validatePipelineWebhookConfigurationSideEffects.Has(string(*w.SideEffects)) {
		errs = errs.Also(apis.ErrInvalidValue(*w.SideEffects, fmt.Sprintf("%s.webhookconfigurationoptions.sideEffects", path)))
	}
	return errs
}

func (op *AdditionalOptions) validate(path string) (errs *apis.FieldError) {
	if op.WebhookConfigurationOptions != nil {
		for _, webhookConfig := range op.WebhookConfigurationOptions {
			return webhookConfig.validate(path)
		}
	}
	return errs
}
