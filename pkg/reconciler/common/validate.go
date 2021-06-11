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

package common

import (
	"context"
	"fmt"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/logging"
)

// ValidateParamsAndSetDefault validates the params and their values with the component params passed
// If the params from spec is missing some params then this will add them with default values
// If there is need to check any special conditions, a func can be passed to validate the same params
// which will be called before setting the default values
func ValidateParamsAndSetDefault(ctx context.Context, params *[]v1alpha1.Param,
	componentParams map[string]v1alpha1.ParamValue, validateCond func(params *[]v1alpha1.Param) error) (bool, error) {

	updated := false
	logger := logging.FromContext(ctx)

	// Validate all params passed are valid and have valid values
	for _, p := range *params {
		pv, ok := componentParams[p.Name]
		if !ok {
			logger.Error("invalid param: %s")
			return updated, fmt.Errorf("invalid param : %s", p.Name)
		}
		if !isParamValueValid(p.Value, pv.Possible) {
			msg := fmt.Sprintf("invalid value (%s) for param: %s", p.Value, p.Name)
			logger.Error(msg)
			return updated, fmt.Errorf(msg)
		}
	}

	err := validateCond(params)
	if err != nil {
		return false, err
	}

	// Parse params and convert in a map
	specParams := ParseParams(*params)

	// If a param is not passed, add the param with default value
	for d := range componentParams {
		_, ok := specParams[d]
		if !ok {
			*params = append(*params,
				v1alpha1.Param{
					Name:  d,
					Value: componentParams[d].Default,
				})
			updated = true
		}
	}
	return updated, nil
}

// ParseParams returns the params passed in a map
func ParseParams(params []v1alpha1.Param) map[string]string {
	paramsMap := map[string]string{}
	for _, p := range params {
		paramsMap[p.Name] = p.Value
	}
	return paramsMap
}

func isParamValueValid(value string, possible []string) bool {
	for _, v := range possible {
		if v == value {
			return true
		}
	}
	return false
}
