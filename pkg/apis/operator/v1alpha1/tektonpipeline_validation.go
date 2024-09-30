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
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var (
	validatePipelineAllowedApiFields          = sets.NewString("", config.AlphaAPIFields, config.BetaAPIFields, config.StableAPIFields)
	validatePipelineVerificationNoMatchPolicy = sets.NewString("", config.FailNoMatchPolicy, config.WarnNoMatchPolicy, config.IgnoreNoMatchPolicy)
	validatePipelineResultExtractionMethod    = sets.NewString("", config.ResultExtractionMethodTerminationMessage, config.ResultExtractionMethodSidecarLogs)
	validatePipelineEnforceNonFalsifiability  = sets.NewString("", config.EnforceNonfalsifiabilityNone, config.EnforceNonfalsifiabilityWithSpire)
	validatePipelineCoschedule                = sets.NewString("", config.CoscheduleDisabled, config.CoscheduleWorkspaces, config.CoschedulePipelineRuns, config.CoscheduleIsolatePipelineRun)
	validatePipelineInlineSpecDisable         = sets.NewString("", "pipeline", "pipelinerun", "taskrun")
)

func (tp *TektonPipeline) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if tp.GetName() != PipelineResourceName {
		errMsg := fmt.Sprintf("metadata.name, Only one instance of TektonPipeline is allowed by name, %s", PipelineResourceName)
		errs = errs.Also(apis.ErrInvalidValue(tp.GetName(), errMsg))
	}

	// execute common spec validations
	errs = errs.Also(tp.Spec.CommonSpec.validate("spec"))

	errs = errs.Also(tp.Spec.PipelineProperties.validate("spec"))

	errs = errs.Also(tp.Spec.Options.validate("spec"))

	return errs
}

func (p *PipelineProperties) validate(path string) (errs *apis.FieldError) {

	if !validatePipelineAllowedApiFields.Has(p.EnableApiFields) {
		errs = errs.Also(apis.ErrInvalidValue(p.EnableApiFields, fmt.Sprintf("%s.enable-api-fields", path)))
	}

	if p.DisableInlineSpec != "" {
		val := strings.Split(p.DisableInlineSpec, ",")
		for _, v := range val {
			if !validatePipelineInlineSpecDisable.Has(v) {
				errs = errs.Also(apis.ErrInvalidValue(p.DisableInlineSpec, fmt.Sprintf("%s.disable-inline-spec", path)))
			}
		}
	}

	if p.DefaultTimeoutMinutes != nil {
		if *p.DefaultTimeoutMinutes == 0 {
			errs = errs.Also(apis.ErrInvalidValue(p.DefaultTimeoutMinutes, path+".default-timeout-minutes"))
		}
	}
	if p.MaxResultSize != nil {
		if *p.MaxResultSize >= 1572864 {
			errs = errs.Also(apis.ErrInvalidValue(p.MaxResultSize, path+".max-result-size"))
		}
	}

	// validate trusted-resources-verification-no-match-policy
	if !validatePipelineVerificationNoMatchPolicy.Has(p.VerificationNoMatchPolicy) {
		errs = errs.Also(apis.ErrInvalidValue(p.VerificationNoMatchPolicy, fmt.Sprintf("%s.trusted-resources-verification-no-match-policy", path)))
	}

	if !validatePipelineResultExtractionMethod.Has(p.ResultExtractionMethod) {
		errs = errs.Also(apis.ErrInvalidValue(p.ResultExtractionMethod, fmt.Sprintf("%s.results-from", path)))
	}

	if !validatePipelineEnforceNonFalsifiability.Has(p.EnforceNonfalsifiability) {
		errs = errs.Also(apis.ErrInvalidValue(p.EnforceNonfalsifiability, fmt.Sprintf("%s.enforce-nonfalsifiability", path)))
	}

	if !validatePipelineCoschedule.Has(p.Coschedule) {
		errs = errs.Also(apis.ErrInvalidValue(p.Coschedule, fmt.Sprintf("%s.coschedule", path)))
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

	// check for StatefulsetOrdinals and Replicas
	if prof.StatefulsetOrdinals != nil && *prof.StatefulsetOrdinals {
		if prof.Replicas != nil {
			replicas := uint(*prof.Replicas)
			if *prof.Buckets != replicas {
				errs = errs.Also(apis.ErrInvalidValue(*prof.Replicas, fmt.Sprintf("%s.replicas", path), "spec.performance.replicas must equal spec.performance.buckets for statefulset ordinals"))
			}
		}
	}

	return errs
}
