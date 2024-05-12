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
	"regexp"

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// paramsRegexp captures TriggerTemplate parameter names $(tt.params.NAME)
var paramsRegexp = regexp.MustCompile(`\$\(tt.params.(?P<var>[_a-zA-Z][_a-zA-Z0-9.-]*)\)`)

var _ resourcesemantics.VerbLimited = (*TriggerTemplate)(nil)

// SupportedVerbs returns the operations that validation should be called for
func (t *TriggerTemplate) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate validates a TriggerTemplate.
func (t *TriggerTemplate) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(t.GetObjectMeta()).ViaField("metadata")
	return errs.Also(t.Spec.validate(ctx).ViaField("spec"))
}

// revive:disable:unused-parameter

// Validate validates a TriggerTemplateSpec.
func (s *TriggerTemplateSpec) validate(ctx context.Context) (errs *apis.FieldError) {
	if equality.Semantic.DeepEqual(s, &TriggerTemplateSpec{}) {
		errs = errs.Also(apis.ErrMissingField(apis.CurrentField))
	}
	if len(s.ResourceTemplates) == 0 {
		errs = errs.Also(apis.ErrMissingField("resourcetemplates"))
	}
	errs = errs.Also(validateResourceTemplates(s.ResourceTemplates).ViaField("resourcetemplates"))
	errs = errs.Also(verifyParamDeclarations(s.Params, s.ResourceTemplates).ViaField("resourcetemplates"))
	return errs
}

func validateResourceTemplates(templates []TriggerResourceTemplate) (errs *apis.FieldError) {
	for i, trt := range templates {
		data := new(unstructured.Unstructured)
		if err := data.UnmarshalJSON(trt.Raw); err != nil {
			// a missing kind makes the unmarshalling throw an error
			errs = errs.Also(apis.ErrMissingField(fmt.Sprintf("[%d].kind", i)))
		}

		if data.GetAPIVersion() == "" {
			errs = errs.Also(apis.ErrMissingField(fmt.Sprintf("[%d].apiVersion", i)))
		}
	}
	return errs
}

// Verify every param in the ResourceTemplates is declared with a ParamSpec
func verifyParamDeclarations(params []ParamSpec, templates []TriggerResourceTemplate) *apis.FieldError {
	declaredParamNames := sets.NewString()
	for _, param := range params {
		declaredParamNames.Insert(param.Name)
	}
	for i, template := range templates {
		// Get all params in the template $(tt.params.NAME)
		templateParams := paramsRegexp.FindAllSubmatch(template.RawExtension.Raw, -1)
		for _, templateParam := range templateParams {
			templateParamName := string(templateParam[1])
			if !declaredParamNames.Has(templateParamName) {
				fieldErr := apis.ErrInvalidValue(
					fmt.Sprintf("undeclared param '$(tt.params.%s)'", templateParamName),
					fmt.Sprintf("[%d]", i),
				)
				fieldErr.Details = fmt.Sprintf("'$(tt.params.%s)' must be declared in spec.params", templateParamName)
				return fieldErr
			}
		}
	}

	return nil
}
