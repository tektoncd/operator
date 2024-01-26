/*
Copyright 2022 The Tekton Authors

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
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	kubernetesValidation "k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

// limit is 25 because this name goes in the installerset name which already have 38 characters, so additional length we
// can have for name is 25, as the kubernetes have restriction for 63
const additionalPACControllerNameMaxLength = 25

func (pac *OpenShiftPipelinesAsCode) Validate(ctx context.Context) *apis.FieldError {
	if apis.IsInDelete(ctx) {
		return nil
	}

	var errs *apis.FieldError

	// execute common spec validations
	errs = errs.Also(pac.Spec.CommonSpec.validate("spec"))

	errs = errs.Also(pac.Spec.PACSettings.validate("spec"))

	return errs
}

func (pacSettings *PACSettings) validate(path string) *apis.FieldError {
	var errs *apis.FieldError

	if err := settings.Validate(pacSettings.Settings); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err, fmt.Sprintf("%s.settings", path)))
	}

	for name, additionalPACControllerConfig := range pacSettings.AdditionalPACControllers {
		if err := validateAdditionalPACControllerName(name); err != nil {
			errs = errs.Also(apis.ErrInvalidValue(err, fmt.Sprintf("%s.additionalPACControllers", path)))
		}

		errs = errs.Also(additionalPACControllerConfig.validate(fmt.Sprintf("%s.additionalPACControllers", path)))
	}

	return errs
}

func (additionalPACControllerConfig AdditionalPACControllerConfig) validate(path string) *apis.FieldError {
	var errs *apis.FieldError

	if err := validateKubernetesName(additionalPACControllerConfig.ConfigMapName); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err, fmt.Sprintf("%s.configMapName", path)))
	}

	if err := validateKubernetesName(additionalPACControllerConfig.SecretName); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err, fmt.Sprintf("%s.secretName", path)))
	}

	if err := settings.Validate(additionalPACControllerConfig.Settings); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err, fmt.Sprintf("%s.settings", path)))
	}

	return errs
}

// validates the name of the controller resource is valid kubernetes name
func validateAdditionalPACControllerName(name string) *apis.FieldError {
	if err := kubernetesValidation.IsDNS1123Subdomain(name); len(err) > 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("invalid resource name %q: must be a valid DNS label", name),
			Paths:   []string{"name"},
		}
	}

	if len(name) > additionalPACControllerNameMaxLength {
		return &apis.FieldError{
			Message: fmt.Sprintf("invalid resource name %q: length must be no more than %d characters", name, additionalPACControllerNameMaxLength),
			Paths:   []string{"name"},
		}
	}
	return nil
}

// validates the name of the resource is valid kubernetes name
func validateKubernetesName(name string) *apis.FieldError {
	if err := kubernetesValidation.IsDNS1123Subdomain(name); len(err) > 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("invalid resource name %q: must be a valid DNS label", name),
			Paths:   []string{"name"},
		}
	}

	if len(name) > kubernetesValidation.DNS1123LabelMaxLength {
		return &apis.FieldError{
			Message: fmt.Sprintf("invalid resource name %q: length must be no more than %d characters", name, kubernetesValidation.DNS1123LabelMaxLength),
			Paths:   []string{"name"},
		}
	}
	return nil
}
