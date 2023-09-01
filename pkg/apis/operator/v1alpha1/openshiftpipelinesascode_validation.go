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

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"knative.dev/pkg/apis"
)

func (pac *OpenShiftPipelinesAsCode) Validate(ctx context.Context) *apis.FieldError {
	if apis.IsInDelete(ctx) {
		return nil
	}

	var errs *apis.FieldError

	// execute common spec validations
	errs = errs.Also(pac.Spec.CommonSpec.validate("spec"))

	errs = errs.Also(validatePACSetting(pac.Spec.PACSettings))

	return errs
}

func validatePACSetting(pacSettings PACSettings) *apis.FieldError {
	var errs *apis.FieldError

	if err := settings.Validate(pacSettings.Settings); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err, "spec.platforms.openshift.pipelinesAsCode"))
	}
	return errs
}
