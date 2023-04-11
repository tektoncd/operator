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

package v1beta1

import (
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var _ resourcesemantics.VerbLimited = (*TriggerBinding)(nil)

// SupportedVerbs returns the operations that validation should be called for
func (tb *TriggerBinding) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate TriggerBinding.
func (tb *TriggerBinding) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(tb.GetObjectMeta()).ViaField("metadata")
	return errs.Also(tb.Spec.Validate(ctx).ViaField("spec"))
}

// Validate TriggerBindingSpec.
func (s *TriggerBindingSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	if equality.Semantic.DeepEqual(s, &TriggerBindingSpec{}) {
		return errs.Also(apis.ErrMissingField(apis.CurrentField))
	}
	return errs.Also(validateParams(s.Params).ViaField("params"))
}

func validateParams(params []Param) *apis.FieldError {
	// Ensure there aren't multiple params with the same name.
	seen := sets.NewString()
	for i, param := range params {
		if seen.Has(param.Name) {
			return apis.ErrMultipleOneOf(fmt.Sprintf("[%d].name", i))
		}
		seen.Insert(param.Name)
		errs := validateParamValue(param.Value).ViaField(fmt.Sprintf("[%d]", i))
		if errs != nil {
			return errs
		}
	}
	return nil
}

func validateParamValue(in string) *apis.FieldError {
	if !strings.Contains(in, "$(") {
		return nil
	}
	// Splits string on $( to find potential Tekton expressions
	maybeExpressions := strings.Split(in, "$(")
	terminated := true
	for _, e := range maybeExpressions[1:] { // Split always returns at least one element
		// Iterate until we find the first unbalanced )
		numOpenBrackets := 0
		if !terminated {
			return apis.ErrInvalidValue(in, "value")
		}
		terminated = false
		for _, ch := range e {
			switch ch {
			case '(':
				numOpenBrackets++
			case ')':
				numOpenBrackets--
				if numOpenBrackets < 0 {
					terminated = true
				}
			default:
				continue
			}
			if numOpenBrackets < 0 {
				terminated = true
				break
			}
		}
	}
	return nil

}
