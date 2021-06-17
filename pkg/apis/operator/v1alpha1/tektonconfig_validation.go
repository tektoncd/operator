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

package v1alpha1

import (
	"context"
	"knative.dev/pkg/apis"
)

func (tc *TektonConfig) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if tc.Spec.TargetNamespace == "" {
		errs = errs.Also(apis.ErrMissingField("spec.targetNamespace"))
	}

	if tc.Spec.Profile != "" {
		if isValid := isValueInArray(Profiles, tc.Spec.Profile); !isValid {
			errs = errs.Also(apis.ErrInvalidValue(tc.Spec.Profile, "spec.profile"))
		}
	}

	if !tc.Spec.Pruner.IsEmpty() {
		errs = errs.Also(tc.Spec.Pruner.validate())
	}

	if !tc.Spec.Addon.IsEmpty() {
		errs = errs.Also(tc.Spec.Addon.validate())
	}

	return errs
}

func (a Addon) validate() *apis.FieldError {
	var errs *apis.FieldError

	for i, p := range a.Params {
		paramValue, ok := AddonParams[p.Name]
		if !ok {
			errs = errs.Also(apis.ErrInvalidKeyName(p.Name, "spec.addon.params"))
			continue
		}
		if !isValueInArray(paramValue.Possible, p.Value) {
			errs = errs.Also(apis.ErrInvalidArrayValue(p.Value, "spec.addon.params"+p.Name, i))
		}
	}

	return errs
}

func (p Prune) validate() *apis.FieldError {
	var errs *apis.FieldError

	if len(p.Resources) != 0 {
		for i, r := range p.Resources {
			if !isValueInArray(PruningResource, r) {
				errs = errs.Also(apis.ErrInvalidArrayValue(r, "spec.pruner.resources", i))
			}
		}
		if p.Schedule == "" {
			errs = errs.Also(apis.ErrMissingField("spec.pruner.schedule"))
		}
		return errs
	}

	if p.Schedule == "" {
		errs = errs.Also(apis.ErrMissingField("spec.pruner.schedule"))
	}

	return errs.Also(apis.ErrMissingField("spec.pruner.resources"))
}

func isValueInArray(arr []string, key string) bool {
	for _, p := range arr {
		if p == key {
			return true
		}
	}
	return false
}
