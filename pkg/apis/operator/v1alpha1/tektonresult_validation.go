/*
Copyright 2023 The Tekton Authors

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
	"strings"

	"knative.dev/pkg/apis"
)

const (
	LogsTypeLoki = "loki"
)

func (tp *TektonResult) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if tp.GetName() != ResultResourceName {
		errMsg := fmt.Sprintf("metadata.name, Only one instance of TektonResult is allowed by name, %s", ResultResourceName)
		return errs.Also(apis.ErrInvalidValue(tp.GetName(), errMsg))
	}
	errs = errs.Also(tp.Spec.validate("spec"))
	return errs
}

func (trs *TektonResultSpec) validate(path string) (errs *apis.FieldError) {
	if trs.LokiStackName != "" {
		if strings.ToLower(trs.LogsType) != LogsTypeLoki && trs.LogsType != "" {
			errMsg := fmt.Sprintf("Loki stack is only supported when logs_type is loki or empty, got logs_type: %s", trs.LogsType)
			errs = errs.Also(apis.ErrInvalidValue(trs.LogsType, fmt.Sprintf("%s.logs_type", path), errMsg))
		}
		if trs.LokiStackNamespace == "" {
			errMsg := "Loki stack namespace is required when loki_stack_name is provided"
			errs = errs.Also(apis.ErrInvalidValue(trs.LokiStackNamespace, fmt.Sprintf("%s.loki_stack_namespace", path), errMsg))
		}
	}
	return errs
}
