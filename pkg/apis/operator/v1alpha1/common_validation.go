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
	"fmt"

	"knative.dev/pkg/apis"
)

func (ta *CommonSpec) validate(path string) *apis.FieldError {
	var errs *apis.FieldError
	targetNamespacePath := fmt.Sprintf("%s.targetNamespace", path)
	if ta.GetTargetNamespace() == "" {
		errs = errs.Also(apis.ErrMissingField(targetNamespacePath))
	} else if IsOpenShiftPlatform() {
		// "openshift-operators" namespace restricted in openshift environment
		if ta.GetTargetNamespace() == "openshift-operators" {
			errs = errs.Also(apis.ErrInvalidValue(ta.GetTargetNamespace(), targetNamespacePath, "'openshift-operators' namespace is not allowed"))
		}
	}
	return errs
}
