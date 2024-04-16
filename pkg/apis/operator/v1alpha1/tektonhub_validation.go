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

	"knative.dev/pkg/apis"
)

func validateHubParams(params []Param, pathToParams string) *apis.FieldError {
	var errs *apis.FieldError

	for i, p := range params {
		paramValue, ok := HubParams[p.Name]
		if !ok {
			errs = errs.Also(apis.ErrInvalidKeyName(p.Name, pathToParams))
			continue
		}
		if !isValueInArray(paramValue.Possible, p.Value) {
			path := pathToParams + "." + p.Name
			errs = errs.Also(apis.ErrInvalidArrayValue(p.Value, path, i))
		}
	}

	return errs
}

func (th *TektonHub) Validate(ctx context.Context) (errs *apis.FieldError) {
	if apis.IsInDelete(ctx) {
		return nil
	}

	// disallow to update the targetNamespace
	if apis.IsInUpdate(ctx) {
		existingTC := apis.GetBaseline(ctx).(*TektonHub)
		if existingTC.Spec.GetTargetNamespace() != th.Spec.GetTargetNamespace() {
			errs = errs.Also(apis.ErrGeneric("doesn't allow to update targetNamespace, delete existing TektonHub object and create the updated TektonHub object", "spec.targetNamespace"))
		}
	}

	// execute common spec validations
	errs = errs.Also(th.Spec.CommonSpec.validate("spec"))

	// validate database secret name
	if th.Spec.Db.DbSecretName != "" && th.Spec.Db.DbSecretName != HubDbSecretName {
		errs = errs.Also(apis.ErrInvalidValue(th.Spec.Db.DbSecretName, "spec.db.secret"))
	}

	// validate api secret name
	if th.Spec.Api.ApiSecretName != "" && th.Spec.Api.ApiSecretName != HubApiSecretName {
		errs = errs.Also(apis.ErrInvalidValue(th.Spec.Api.ApiSecretName, "spec.api.secret"))
	}

	return errs
}
