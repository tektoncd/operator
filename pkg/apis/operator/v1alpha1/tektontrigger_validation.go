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

	"github.com/tektoncd/triggers/pkg/apis/config"
	"knative.dev/pkg/apis"
)

func (tr *TektonTrigger) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if tr.GetName() != TriggerResourceName {
		errMsg := fmt.Sprintf("metadata.name,  Only one instance of TektonTrigger is allowed by name, %s", TriggerResourceName)
		errs = errs.Also(apis.ErrInvalidValue(tr.GetName(), errMsg))
	}

	// execute common spec validations
	errs = errs.Also(tr.Spec.CommonSpec.validate("spec"))

	return errs.Also(tr.Spec.TriggersProperties.validate("spec"))
}

func (tr *TriggersProperties) validate(path string) (errs *apis.FieldError) {

	if tr.EnableApiFields != "" {
		if tr.EnableApiFields != config.StableAPIFieldValue && tr.EnableApiFields != config.AlphaAPIFieldValue {
			errs = errs.Also(apis.ErrInvalidValue(tr.EnableApiFields, path+".enable-api-fields"))
		}
	}
	return errs
}
