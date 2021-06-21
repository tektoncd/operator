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

func (tp *TektonPipeline) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if tp.Spec.TargetNamespace == "" {
		errs = errs.Also(apis.ErrMissingField("spec.targetNamespace"))
	}

	return errs.Also(tp.Spec.PipelineProperties.validate("spec"))
}

func (p *PipelineProperties) validate(path string) (errs *apis.FieldError) {

	if p.EnableApiFields != "" {
		if p.EnableApiFields == PipelineApiFieldStable || p.EnableApiFields == PipelineApiFieldAlpha {
			return errs
		}
		errs = errs.Also(apis.ErrInvalidValue(p.EnableApiFields, path+".enable-api-fields"))
	}
	if p.DefaultTimeoutMinutes != nil {
		if *p.DefaultTimeoutMinutes == 0 {
			errs = errs.Also(apis.ErrInvalidValue(p.DefaultTimeoutMinutes, path+".default-timeout-minutes"))
		}
	}
	return errs
}
