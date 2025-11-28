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
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/yaml"
)

// Validate performs comprehensive validation on TektonPruner using the tektoncd/pruner webhook validation
func (tp *TektonPruner) Validate(ctx context.Context) (errs *apis.FieldError) {
	// Skip validation if the resource is being deleted
	if apis.IsInDelete(ctx) {
		return nil
	}

	// Validate that only one instance of TektonPruner exists with the correct name
	if tp.GetName() != TektonPrunerResourceName {
		errMsg := fmt.Sprintf("metadata.name, Only one instance of TektonPruner is allowed by name, %s", TektonPrunerResourceName)
		errs = errs.Also(apis.ErrInvalidValue(tp.GetName(), errMsg))
	}

	// Validate common spec fields (targetNamespace, etc.)
	errs = errs.Also(tp.Spec.CommonSpec.validate("spec"))

	// Disallow updating the targetNamespace
	if apis.IsInUpdate(ctx) {
		existingTP := apis.GetBaseline(ctx).(*TektonPruner)
		if existingTP.Spec.GetTargetNamespace() != tp.Spec.GetTargetNamespace() {
			errs = errs.Also(apis.ErrGeneric("Doesn't allow to update targetNamespace, delete existing TektonPruner and create the updated TektonPruner", "spec.targetNamespace"))
		}
	}

	// Validate the TektonPrunerConfig using tektoncd/pruner webhook validation
	errs = errs.Also(tp.Spec.Pruner.validate("spec"))

	// Validate additional options
	errs = errs.Also(tp.Spec.Options.validate("spec.options"))

	return errs
}

// validate validates the Pruner configuration by leveraging tektoncd/pruner webhook validation
// This ensures consistency with the upstream pruner validation logic
func (p *Pruner) validate(path string) *apis.FieldError {
	var errs *apis.FieldError

	// If pruner is disabled, no validation is required
	if p.IsDisabled() {
		return errs
	}

	// Validate the TektonPrunerConfig using tektoncd/pruner's validation functions
	errs = errs.Also(p.TektonPrunerConfig.validate(path))

	return errs
}

// validate validates the TektonPrunerConfig by calling tektoncd/pruner's validation functions
// This delegates validation to the upstream pruner package to avoid code duplication
func (tpc *TektonPrunerConfig) validate(path string) *apis.FieldError {
	var errs *apis.FieldError

	// Create a ConfigMap object to leverage tektoncd/pruner's validation functions
	// This allows us to reuse the comprehensive validation logic from the pruner package
	cm, err := tpc.toConfigMap()
	if err != nil {
		errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("failed to convert TektonPrunerConfig to ConfigMap: %v", err), path+".global-config"))
		return errs
	}

	// Use tektoncd/pruner's ValidateConfigMap function
	// This function performs comprehensive validation including:
	// - EnforcedConfigLevel validation (global, namespace, resource)
	// - TTLSecondsAfterFinished validation (non-negative, within limits)
	// - SuccessfulHistoryLimit validation (non-negative, within limits)
	// - FailedHistoryLimit validation (non-negative, within limits)
	// - HistoryLimit validation (non-negative, within limits)
	// - Namespace-level config validation (if global limits are set)
	// - Selector validation (ensures selectors are only in namespace-level ConfigMaps)
	if err := config.ValidateConfigMap(cm); err != nil {
		errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("pruner config validation failed: %v", err), path+".global-config"))
	}

	return errs
}

// toConfigMap converts TektonPrunerConfig to a corev1.ConfigMap for validation
// This is a helper function to leverage tektoncd/pruner's ConfigMap-based validation
func (tpc *TektonPrunerConfig) toConfigMap() (*corev1.ConfigMap, error) {
	// Marshal the GlobalConfig to YAML
	data, err := yaml.Marshal(tpc.GlobalConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GlobalConfig to YAML: %w", err)
	}

	// Create a ConfigMap with the global-config data
	cm := &corev1.ConfigMap{
		Data: map[string]string{
			config.PrunerGlobalConfigKey: string(data),
		},
	}

	return cm, nil
}

// validatePrunerConfigInTektonConfig validates the Pruner configuration in TektonConfig
// This is used by TektonConfig validation to ensure pruner settings are valid
func (p *Pruner) validateInTektonConfig(path string) *apis.FieldError {
	var errs *apis.FieldError

	// If TektonPruner (event-based) is disabled, no validation is required
	if p.IsDisabled() {
		return errs
	}

	// Validate the TektonPrunerConfig using tektoncd/pruner's validation functions
	errs = errs.Also(p.TektonPrunerConfig.validate(path + ".tektonpruner"))

	return errs
}
