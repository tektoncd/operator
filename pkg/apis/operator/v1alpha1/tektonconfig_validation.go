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
	"fmt"

	"knative.dev/pkg/apis"
)

func (tc *TektonConfig) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if tc.GetName() != ConfigResourceName {
		errMsg := fmt.Sprintf("metadata.name,  Only one instance of TektonConfig is allowed by name, %s", ConfigResourceName)
		errs = errs.Also(apis.ErrInvalidValue(tc.GetName(), errMsg))
	}

	if tc.Spec.TargetNamespace == "" {
		errs = errs.Also(apis.ErrMissingField("spec.targetNamespace"))
	}

	if tc.Spec.Profile != "" {
		if isValid := isValueInArray(Profiles, tc.Spec.Profile); !isValid {
			errs = errs.Also(apis.ErrInvalidValue(tc.Spec.Profile, "spec.profile"))
		}
	}

	// validate pruner specifications
	errs = errs.Also(tc.Spec.Pruner.validate())

	if !tc.Spec.Addon.IsEmpty() {
		errs = errs.Also(validateAddonParams(tc.Spec.Addon.Params, "spec.addon.params"))
	}

	if !tc.Spec.Hub.IsEmpty() {
		errs = errs.Also(validateHubParams(tc.Spec.Hub.Params, "spec.hub.params"))
	}

	errs = errs.Also(tc.Spec.Pipeline.PipelineProperties.validate("spec.pipeline"))

	return errs.Also(tc.Spec.Trigger.TriggersProperties.validate("spec.trigger"))
}

func (p Prune) validate() *apis.FieldError {
	var errs *apis.FieldError

	// if pruner job disable no validation required
	if p.Disabled {
		return errs
	}

	if len(p.Resources) != 0 {
		for i, r := range p.Resources {
			if !isValueInArray(PruningResource, r) {
				errs = errs.Also(apis.ErrInvalidArrayValue(r, "spec.pruner.resources", i))
			}
		}
	} else {
		errs = errs.Also(apis.ErrMissingField("spec.pruner.resources"))
	}

	// tkn cli supports both "keep" and "keep-since", even though there is an issue with the logic
	// when we supply both "keep" and "keep-since", the outcome always equivalent to "keep", "keep-since" ignored
	// hence we strict with a single flag support until the issue is fixed in tkn cli
	// cli issue: https://github.com/tektoncd/cli/issues/1990
	if p.Keep != nil && p.KeepSince != nil {
		errs = errs.Also(apis.ErrMultipleOneOf("spec.pruner.keep", "spec.pruner.keep-since"))
	}

	if p.Keep == nil && p.KeepSince == nil {
		errs = errs.Also(apis.ErrMissingOneOf("spec.pruner.keep", "spec.pruner.keep-since"))
	}
	if p.Keep != nil && *p.Keep == 0 {
		errs = errs.Also(apis.ErrInvalidValue(*p.Keep, "spec.pruner.keep"))
	}
	if p.KeepSince != nil && *p.KeepSince == 0 {
		errs = errs.Also(apis.ErrInvalidValue(*p.KeepSince, "spec.pruner.keep-since"))
	}

	return errs
}

func isValueInArray(arr []string, key string) bool {
	for _, p := range arr {
		if p == key {
			return true
		}
	}
	return false
}
