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

func (td *TektonDashboard) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if td.GetName() != DashboardResourceName {
		errMsg := fmt.Sprintf("metadata.name,  Only one instance of TektonDashboard is allowed by name, %s", DashboardResourceName)
		errs = errs.Also(apis.ErrInvalidValue(td.GetName(), errMsg))
	}

	if td.Spec.TargetNamespace == "" {
		errs = errs.Also(apis.ErrMissingField("spec.targetNamespace"))
	}

	return errs
}

func (td *TektonDashboard) SetDefaults(ctx context.Context) {
}
