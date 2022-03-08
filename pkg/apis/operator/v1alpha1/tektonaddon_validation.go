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

func (ta *TektonAddon) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if ta.GetName() != AddonResourceName {
		errMsg := fmt.Sprintf("metadata.name,  Only one instance of TektonAddon is allowed by name, %s", AddonResourceName)
		errs = errs.Also(apis.ErrInvalidValue(ta.GetName(), errMsg))
	}

	if ta.Spec.TargetNamespace == "" {
		errs = errs.Also(apis.ErrMissingField("spec.targetNamespace"))
	}

	if len(ta.Spec.Params) != 0 {
		errs = errs.Also(validateAddonParams(ta.Spec.Params, "spec.params"))
	}

	return errs
}

func validateAddonParams(params []Param, pathToParams string) *apis.FieldError {
	var errs *apis.FieldError

	for i, p := range params {
		paramValue, ok := AddonParams[p.Name]
		if !ok {
			errs = errs.Also(apis.ErrInvalidKeyName(p.Name, pathToParams))
			continue
		}
		if !isValueInArray(paramValue.Possible, p.Value) {
			path := pathToParams + "." + p.Name
			errs = errs.Also(apis.ErrInvalidArrayValue(p.Value, path, i))
		}
	}

	paramsMap := ParseParams(params)
	if (paramsMap[ClusterTasksParam] == "false") && (paramsMap[PipelineTemplatesParam] == "true") {
		errs = errs.Also(apis.ErrGeneric("pipelineTemplates cannot be true if clusterTask is false", pathToParams))
	}
	if (paramsMap[ClusterTasksParam] == "false") && (paramsMap[CommunityClusterTasks] == "true") {
		errs = errs.Also(apis.ErrGeneric("communityClusterTasks cannot be true if clusterTask is false", pathToParams))
	}

	return errs
}
