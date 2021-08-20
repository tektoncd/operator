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

package config

import (
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	StableAPIFieldValue    = "stable"
	AlphaAPIFieldValue     = "alpha"
	enableAPIFieldsKey     = "enable-api-fields"
	DefaultEnableAPIFields = StableAPIFieldValue
)

// FeatureFlags holds the features configurations
// +k8s:deepcopy-gen=true
type FeatureFlags struct {
	// EnableAPIFields determines which gated features are enabled.
	// Acceptable values are "stable" or "alpha". Defaults to "stable"
	EnableAPIFields string
}

// GetFeatureFlagsConfigName returns the name of the configmap containing all
// feature flags.
func GetFeatureFlagsConfigName() string {
	if e := os.Getenv("CONFIG_FEATURE_FLAGS_NAME"); e != "" {
		return e
	}
	return "feature-flags-triggers"
}

// NewFeatureFlagsFromMap returns a Config given a map corresponding to a ConfigMap
func NewFeatureFlagsFromMap(cfgMap map[string]string) (*FeatureFlags, error) {
	tc := FeatureFlags{}
	var err error
	if tc.EnableAPIFields, err = getEnabledAPI(cfgMap); err != nil {
		return nil, err
	}
	return &tc, nil
}

// getEnabledAPI gets the "enable-api-fields" flag based on the content of a given map.
// If the feature gate is invalid or missing then an error is returned.
func getEnabledAPI(cfgMap map[string]string) (string, error) {
	value := DefaultEnableAPIFields
	if cfg, ok := cfgMap[enableAPIFieldsKey]; ok {
		value = strings.ToLower(cfg)
	}
	if value != AlphaAPIFieldValue && value != StableAPIFieldValue {
		return "", fmt.Errorf("invalid value for feature flag %q: %q", enableAPIFieldsKey, value)
	}
	return value, nil
}

// NewFeatureFlagsFromConfigMap returns a Config for the given configmap
func NewFeatureFlagsFromConfigMap(config *corev1.ConfigMap) (*FeatureFlags, error) {
	return NewFeatureFlagsFromMap(config.Data)
}
