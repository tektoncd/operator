/*
Copyright 2025 The Tekton Authors

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

	"github.com/tektoncd/pruner/pkg/config"
	"knative.dev/pkg/apis"
)

// Validate performs comprehensive validation on TektonPruner
func (tp *TektonPruner) Validate(ctx context.Context) (errs *apis.FieldError) {
	// Skip validation when deleting
	if apis.IsInDelete(ctx) {
		return nil
	}

	// Validate that only one instance exists with the correct name
	if tp.GetName() != PrunerResourceName {
		errMsg := fmt.Sprintf("metadata.name, Only one instance of TektonPruner is allowed by name, %s", PrunerResourceName)
		errs = errs.Also(apis.ErrInvalidValue(tp.GetName(), errMsg))
	}

	// Execute common spec validations
	errs = errs.Also(tp.Spec.CommonSpec.validate("spec"))

	// Validate pruner configuration using direct struct validation
	errs = errs.Also(tp.Spec.Pruner.validate("spec.pruner"))

	return errs
}

// validate validates the Pruner configuration using direct struct validation
// This ensures consistency with the upstream pruner validation logic
// This method is used by both TektonPruner.Validate() and TektonConfig.Validate()
func (p *Pruner) validate(path string) *apis.FieldError {
	// Skip validation if pruner is disabled
	if p.IsDisabled() {
		return nil
	}

	// Use the new ValidateGlobalConfig function from pruner package
	// This validates the GlobalConfig struct directly without ConfigMap conversion
	// This is the recommended approach for operator CRDs as documented in pruner PR #57
	if err := config.ValidateGlobalConfig(&p.TektonPrunerConfig.GlobalConfig); err != nil {
		return apis.ErrGeneric(
			fmt.Sprintf("pruner config validation failed: %v", err),
			path+".global-config",
		)
	}

	return nil
}
