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

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var (
	validatePipelineAllowedApiFields          = sets.NewString("", config.AlphaAPIFields, config.BetaAPIFields, config.StableAPIFields)
	validatePipelineVerificationNoMatchPolicy = sets.NewString("", config.FailNoMatchPolicy, config.WarnNoMatchPolicy, config.IgnoreNoMatchPolicy)
)

func (tp *TektonPipeline) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if tp.GetName() != PipelineResourceName {
		errMsg := fmt.Sprintf("metadata.name, Only one instance of TektonPipeline is allowed by name, %s", PipelineResourceName)
		errs = errs.Also(apis.ErrInvalidValue(tp.GetName(), errMsg))
	}

	if tp.Spec.TargetNamespace == "" {
		errs = errs.Also(apis.ErrMissingField("spec.targetNamespace"))
	}

	return errs.Also(tp.Spec.PipelineProperties.validate("spec"))
}

func (p *PipelineProperties) validate(path string) (errs *apis.FieldError) {

	if !validatePipelineAllowedApiFields.Has(p.EnableApiFields) {
		errs = errs.Also(apis.ErrInvalidValue(p.EnableApiFields, fmt.Sprintf("%s.enable-api-fields", path)))
	}

	if p.DefaultTimeoutMinutes != nil {
		if *p.DefaultTimeoutMinutes == 0 {
			errs = errs.Also(apis.ErrInvalidValue(p.DefaultTimeoutMinutes, path+".default-timeout-minutes"))
		}
	}

	// validate trusted-resources-verification-no-match-policy
	if !validatePipelineVerificationNoMatchPolicy.Has(p.VerificationNoMatchPolicy) {
		errs = errs.Also(apis.ErrInvalidValue(p.VerificationNoMatchPolicy, fmt.Sprintf("%s.trusted-resources-verification-no-match-policy", path)))
	}

	// validate performance properties
	errs = errs.Also(p.Performance.validate(fmt.Sprintf("%s.performance", path)))

	return errs
}

func (prof *PipelinePerformanceProperties) validate(path string) *apis.FieldError {
	var errs *apis.FieldError

	bucketsPath := fmt.Sprintf("%s.buckets", path)
	// minimum and maximum allowed buckets value
	if prof.Buckets != nil {
		if *prof.Buckets < 1 || *prof.Buckets > 10 {
			errs = errs.Also(apis.ErrOutOfBoundsValue(*prof.Buckets, 1, 10, bucketsPath))
		}
	}

	return errs
}
