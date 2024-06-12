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
	"reflect"

	pacSettings "github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"go.uber.org/zap"
	kubernetesValidation "k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

// limit is 25 because this name goes in the installerset name which already have 38 characters, so additional length we
// can have for name is 25, as the kubernetes have restriction for 63
const additionalPACControllerNameMaxLength = 25

func (pac *OpenShiftPipelinesAsCode) Validate(ctx context.Context) *apis.FieldError {
	if apis.IsInDelete(ctx) {
		return nil
	}

	var errs *apis.FieldError

	logger := logging.FromContext(ctx)

	// execute common spec validations
	errs = errs.Also(pac.Spec.CommonSpec.validate("spec"))

	errs = errs.Also(pac.Spec.PACSettings.validate(logger, "spec"))

	return errs
}

func (ps *PACSettings) validate(logger *zap.SugaredLogger, path string) *apis.FieldError {
	var errs *apis.FieldError

	defaultPacSettings := pacSettings.DefaultSettings()
	if err := pacSettings.SyncConfig(logger, &defaultPacSettings, ps.Settings, pacSettings.DefaultValidators()); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err, fmt.Sprintf("%s.settings", path)))
	}

	for name, additionalPACControllerConfig := range ps.AdditionalPACControllers {
		if err := validateAdditionalPACControllerName(name); err != nil {
			errs = errs.Also(apis.ErrInvalidValue(err, fmt.Sprintf("%s.additionalPACControllers", path)))
		}

		errs = errs.Also(additionalPACControllerConfig.validate(fmt.Sprintf("%s.additionalPACControllers", path)))
	}

	return errs
}

func (aps AdditionalPACControllerConfig) validate(path string) *apis.FieldError {
	var errs *apis.FieldError

	if err := validateKubernetesName(aps.ConfigMapName); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err, fmt.Sprintf("%s.configMapName", path)))
	}

	if err := validateKubernetesName(aps.SecretName); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err, fmt.Sprintf("%s.secretName", path)))
	}

	if err := validateAdditionalPACControllerSettings(aps.Settings); err != nil {
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

// validates the settings of the additionalPACController
func validateAdditionalPACControllerSettings(settings map[string]string) *apis.FieldError {
	var errs *apis.FieldError
	validators := pacSettings.DefaultValidators()
	if len(settings) > 0 {
		fieldTagMapDetails := getFieldTagMap()
		for key, value := range settings {
			fieldName, ok := fieldTagMapDetails[key]
			if !ok {
				continue
			}
			if validationFunc, ok := validators[fieldName]; ok && value != "" {
				if err := validationFunc(value); err != nil {
					errs = errs.Also(apis.ErrInvalidValue(err, fmt.Sprintf("validation failed for field %s", key)))
					continue
				}
			}
		}
		return errs
	}
	return nil
}

// this will return map with all the json tags with value equal to their field names
func getFieldTagMap() map[string]string {
	var fieldTagMapping = make(map[string]string)
	rt := reflect.TypeOf(pacSettings.Settings{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		v := f.Tag.Get("json")
		if v == "" || v == "-" {
			continue
		}
		fieldTagMapping[v] = f.Name
	}
	return fieldTagMapping
}
