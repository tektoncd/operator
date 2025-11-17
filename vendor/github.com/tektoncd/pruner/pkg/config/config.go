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

package config

import (
	"context"
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"knative.dev/pkg/logging"
)

// following types are for internal use

// PrunerResourceType is a string type used to represent different types of resources that the pruner manages
type PrunerResourceType string

// PrunerFieldType is a string type used to represent different configuration types for pruner
type PrunerFieldType string

// EnforcedConfigLevel is a string type to manage the different override levels allowed for Pruner config
type EnforcedConfigLevel string

const (
	// PrunerResourceTypePipelineRun represents the resource type for a PipelineRun in the pruner.
	PrunerResourceTypePipelineRun PrunerResourceType = "pipelineRun"

	// PrunerResourceTypeTaskRun represents the resource type for a TaskRun in the pruner.
	PrunerResourceTypeTaskRun PrunerResourceType = "taskRun"

	// PrunerFieldTypeTTLSecondsAfterFinished represents the field type for the TTL (Time-to-Live) in seconds after the resource is finished.
	PrunerFieldTypeTTLSecondsAfterFinished PrunerFieldType = "ttlSecondsAfterFinished"

	// PrunerFieldTypeSuccessfulHistoryLimit represents the field type for the successful history limit of a resource.
	PrunerFieldTypeSuccessfulHistoryLimit PrunerFieldType = "successfulHistoryLimit"

	// PrunerFieldTypeFailedHistoryLimit represents the field type for the failed history limit of a resource.
	PrunerFieldTypeFailedHistoryLimit PrunerFieldType = "failedHistoryLimit"

	// EnforcedConfigLevelGlobal represents the cluster-wide config level for pruner.
	EnforcedConfigLevelGlobal EnforcedConfigLevel = "global"

	// EnforcedConfigLevelNamespace represents the namespace config level for pruner.
	EnforcedConfigLevelNamespace EnforcedConfigLevel = "namespace"

	// EnforcedConfigLevelResource represents the resource-level config for pruner.
	EnforcedConfigLevelResource EnforcedConfigLevel = "resource"
)

// ResourceSpec is used to hold the config of a specific resource
// Only used in namespace-level ConfigMaps (tekton-pruner-namespace-spec), NOT in global ConfigMaps
type ResourceSpec struct {
	Name         string         `yaml:"name"`               // Exact name of the parent Pipeline or Task
	Selector     []SelectorSpec `yaml:"selector,omitempty"` // Supports selection based on labels and annotations. If Name is given, Name takes precedence
	PrunerConfig `yaml:",inline"`
}

// SelectorSpec allows specifying selectors for matching resources like PipelineRun or TaskRun
// Only applicable in namespace-level ConfigMaps, NOT in global ConfigMaps
type SelectorSpec struct {
	// Match by labels AND annotations. If both are specified, BOTH must match (AND logic)
	MatchLabels      map[string]string `yaml:"matchLabels,omitempty"`
	MatchAnnotations map[string]string `yaml:"matchAnnotations,omitempty"`
}

// NamespaceSpec is used to hold the pruning config of a specific namespace and its resources
// Used in both global ConfigMap (tekton-pruner-default-spec) and namespace ConfigMap (tekton-pruner-namespace-spec)
// Selector support (PipelineRuns/TaskRuns arrays) ONLY works in namespace ConfigMaps
type NamespaceSpec struct {
	PrunerConfig `yaml:",inline"` // Root-level defaults
	PipelineRuns []ResourceSpec   `yaml:"pipelineRuns"` // Selector-based configs (namespace ConfigMap only)
	TaskRuns     []ResourceSpec   `yaml:"taskRuns"`     // Selector-based configs (namespace ConfigMap only)
}

// GlobalConfig represents the global ConfigMap (tekton-pruner-default-spec)
// Root-level fields are defaults; Namespaces map is for per-namespace defaults
// NOTE: Selector support (PipelineRuns/TaskRuns arrays) is IGNORED in global ConfigMap
type GlobalConfig struct {
	PrunerConfig `yaml:",inline"`         // Global root-level defaults
	Namespaces   map[string]NamespaceSpec `yaml:"namespaces"  json:"namespaces"` // Per-namespace defaults (selectors ignored)
}

// PrunerConfig used to hold the cluster-wide pruning config as well as namespace specific pruning config
type PrunerConfig struct {
	// EnforcedConfigLevel allowed values: global, namespace, resource (default: resource)
	EnforcedConfigLevel     *EnforcedConfigLevel `yaml:"enforcedConfigLevel" json:"enforcedConfigLevel"`
	TTLSecondsAfterFinished *int32               `yaml:"ttlSecondsAfterFinished" json:"ttlSecondsAfterFinished"`
	SuccessfulHistoryLimit  *int32               `yaml:"successfulHistoryLimit" json:"successfulHistoryLimit"`
	FailedHistoryLimit      *int32               `yaml:"failedHistoryLimit" json:"failedHistoryLimit"`
	HistoryLimit            *int32               `yaml:"historyLimit" json:"historyLimit"`
}

// prunerConfigStore defines the store structure to hold config from ConfigMap
type prunerConfigStore struct {
	mutex           sync.RWMutex
	globalConfig    GlobalConfig
	namespaceConfig map[string]NamespaceSpec // namespace -> NamespaceSpec
}

var (
	// PrunerConfigStore is the singleton instance to store pruner config
	PrunerConfigStore = prunerConfigStore{
		mutex:           sync.RWMutex{},
		namespaceConfig: make(map[string]NamespaceSpec),
	}
)

// loads config from configMap (global-config) should be called on startup and if there is a change detected on the ConfigMap
func (ps *prunerConfigStore) LoadGlobalConfig(ctx context.Context, configMap *corev1.ConfigMap) error {
	logger := logging.FromContext(ctx)
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Log the current state of globalConfig and namespacedConfig before updating
	logger.Debugw("Loading global config", "oldGlobalConfig", ps.globalConfig)

	globalConfig := &GlobalConfig{}
	if configMap.Data != nil && configMap.Data[PrunerGlobalConfigKey] != "" {
		err := yaml.Unmarshal([]byte(configMap.Data[PrunerGlobalConfigKey]), globalConfig)
		if err != nil {
			return err
		}
	}

	ps.globalConfig = *globalConfig

	if ps.globalConfig.Namespaces == nil {
		ps.globalConfig.Namespaces = map[string]NamespaceSpec{}
	}

	// Log the updated state of globalConfig and namespacedConfig after the update
	logger.Debugw("Updated global config", "newGlobalConfig", ps.globalConfig)

	return nil
}

// LoadNamespaceConfig loads config from namespace-level ConfigMap
func (ps *prunerConfigStore) LoadNamespaceConfig(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error {
	logger := logging.FromContext(ctx)
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Log the current state before updating
	logger.Debugw("Loading namespace config", "namespace", namespace, "oldConfig", ps.namespaceConfig[namespace])

	namespaceSpec := NamespaceSpec{}
	if configMap.Data != nil && configMap.Data[PrunerNamespaceConfigKey] != "" {
		err := yaml.Unmarshal([]byte(configMap.Data[PrunerNamespaceConfigKey]), &namespaceSpec)
		if err != nil {
			return err
		}
	}

	ps.namespaceConfig[namespace] = namespaceSpec

	// Log the updated state after the update
	logger.Debugw("Updated namespace config", "namespace", namespace, "newConfig", ps.namespaceConfig[namespace])

	return nil
}

// DeleteNamespaceConfig removes namespace-level config from the store
func (ps *prunerConfigStore) DeleteNamespaceConfig(ctx context.Context, namespace string) {
	logger := logging.FromContext(ctx)
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	logger.Debugw("Deleting namespace config", "namespace", namespace)
	delete(ps.namespaceConfig, namespace)
}

// loads config from configMap (global-config) should be called on startup and if there is a change detected on the ConfigMap
func (ps *prunerConfigStore) WorkerCount(ctx context.Context, configMap *corev1.ConfigMap) (count int, err error) {
	logger := logging.FromContext(ctx)

	// Log the current state of globalConfig and namespacedConfig before updating
	logger.Debugw("get worker count to concurrently cleanup namesapces", "nsCleanupConcurrentWorkerCount", configMap.Data["WorkerCountForNamespaceCleanup"])

	if configMap.Data != nil && configMap.Data["WorkerCountForNamespaceCleanup"] != "" {
		count, err = GetEnvValueAsInt("WorkerCountForNamespaceCleanup", DefaultWorkerCountForNamespaceCleanup)
		if err != nil {
			return 0, err
		}
	} else {
		count = DefaultWorkerCountForNamespaceCleanup
	}
	logger.Debugw("get worker count to concurrently cleanup namesapces", "nsCleanupConcurrentWorkerCount", count)
	return count, nil
}

// getFromPrunerConfigResourceLevelwithSelector retrieves resource-level configuration using selectors
// This function is used ONLY for namespace-level ConfigMaps (tekton-pruner-namespace-spec), NOT global ConfigMaps
// Selector matching logic:
// - If 'name' is provided, it has absolute precedence (returns nil if no match, no fallback)
// - Otherwise, checks selector arrays (PipelineRuns/TaskRuns) for matches
// - When both matchLabels AND matchAnnotations are specified, BOTH must match (AND logic)
func getFromPrunerConfigResourceLevelwithSelector(namespacesSpec map[string]NamespaceSpec, namespace, name string, selector SelectorSpec, resourceType PrunerResourceType, fieldType PrunerFieldType) (*int32, string) {
	prunerResourceSpec, found := namespacesSpec[namespace]
	if !found {
		return nil, "identifiedBy_global"
	}

	var resourceSpecs []ResourceSpec

	// Select the right resource specs based on the resource type
	switch resourceType {
	case PrunerResourceTypePipelineRun:
		resourceSpecs = prunerResourceSpec.PipelineRuns
	case PrunerResourceTypeTaskRun:
		resourceSpecs = prunerResourceSpec.TaskRuns
	}

	// First, check if name is provided, and use it to match exactly (absolute precedence)
	if name != "" {
		for _, resourceSpec := range resourceSpecs {
			if resourceSpec.Name == name {
				// Return the field value from the matched resourceSpec
				switch fieldType {
				case PrunerFieldTypeTTLSecondsAfterFinished:
					return resourceSpec.TTLSecondsAfterFinished, "identifiedBy_resource_name"
				case PrunerFieldTypeSuccessfulHistoryLimit:
					return resourceSpec.SuccessfulHistoryLimit, "identifiedBy_resource_name"
				case PrunerFieldTypeFailedHistoryLimit:
					return resourceSpec.FailedHistoryLimit, "identifiedBy_resource_name"
				}
			}
		}
		// Name was specified but no match found - continue to selector matching
	}

	// If name-based matching didn't succeed, proceed with selector matching
	if len(selector.MatchAnnotations) > 0 || len(selector.MatchLabels) > 0 {

		for _, resourceSpec := range resourceSpecs {
			// Check if the resourceSpec matches the provided selector by annotations AND labels
			for _, selectorSpec := range resourceSpec.Selector {
				// Both annotations and labels must match when both are specified (AND logic)
				// The ConfigMap's selectorSpec defines the required labels/annotations to match
				// The selector (from the PipelineRun/TaskRun) contains the actual labels/annotations
				annotationsMatch := true
				labelsMatch := true

				// If ConfigMap's selectorSpec has matchAnnotations, check if resource has all of them
				if len(selectorSpec.MatchAnnotations) > 0 {
					if len(selector.MatchAnnotations) == 0 {
						// ConfigMap requires annotations but resource has none - no match
						annotationsMatch = false
					} else {
						// Check if all ConfigMap's required annotations exist in resource
						for key, value := range selectorSpec.MatchAnnotations {
							if resourceAnnotationValue, exists := selector.MatchAnnotations[key]; !exists || resourceAnnotationValue != value {
								annotationsMatch = false
								break
							}
						}
					}
				}

				// If ConfigMap's selectorSpec has matchLabels, check if resource has all of them
				if len(selectorSpec.MatchLabels) > 0 {
					if len(selector.MatchLabels) == 0 {
						// ConfigMap requires labels but resource has none - no match
						labelsMatch = false
					} else {
						// Check if all ConfigMap's required labels exist in resource
						for key, value := range selectorSpec.MatchLabels {
							if resourceLabelValue, exists := selector.MatchLabels[key]; !exists || resourceLabelValue != value {
								labelsMatch = false
								break
							}
						}
					}
				}

				// Only return if BOTH match (AND logic)
				if annotationsMatch && labelsMatch {
					// Return the field value if selectors match
					switch fieldType {
					case PrunerFieldTypeTTLSecondsAfterFinished:
						return resourceSpec.TTLSecondsAfterFinished, "identifiedBy_resource_selector"
					case PrunerFieldTypeSuccessfulHistoryLimit:
						if resourceSpec.SuccessfulHistoryLimit != nil {
							return resourceSpec.SuccessfulHistoryLimit, "identifiedBy_resource_selector"
						} else {
							return resourceSpec.HistoryLimit, "identifiedBy_resource_selector"
						}
					case PrunerFieldTypeFailedHistoryLimit:
						if resourceSpec.FailedHistoryLimit != nil {
							return resourceSpec.FailedHistoryLimit, "identifiedBy_resource_selector"
						} else {
							return resourceSpec.HistoryLimit, "identifiedBy_resource_selector"
						}
					}
				}
			}
		}
	}

	// If no match found, return nil
	return nil, ""
}

// getResourceFieldData retrieves configuration field values based on enforcedConfigLevel
// Design principle: Selector support ONLY for namespace-level ConfigMaps, NOT global ConfigMaps
//
// Lookup hierarchy by enforcedConfigLevel:
//
// 1. EnforcedConfigLevelResource:
//   - Resource-level selector match (from global ConfigMap's Namespaces map)
//   - Namespace root-level (from global ConfigMap's Namespaces map)
//   - Global root-level defaults
//
// 2. EnforcedConfigLevelNamespace:
//   - Resource-level selector match (from namespace ConfigMap - NEW)
//   - Namespace root-level (from namespace ConfigMap)
//   - Namespace root-level (from global ConfigMap's Namespaces map)
//   - Global root-level defaults
//
// 3. EnforcedConfigLevelGlobal:
//   - Global root-level defaults ONLY (no selectors, no namespace lookup)
func getResourceFieldData(globalSpec GlobalConfig, namespaceConfigMap map[string]NamespaceSpec, namespace, name string, selector SelectorSpec, resourceType PrunerResourceType, fieldType PrunerFieldType, enforcedConfigLevel EnforcedConfigLevel) (*int32, string) {
	var fieldData *int32
	var identified_by string

	switch enforcedConfigLevel {
	case EnforcedConfigLevelResource:
		// First try resource level
		fieldData, identified_by = getFromPrunerConfigResourceLevelwithSelector(globalSpec.Namespaces, namespace, name, selector, resourceType, fieldType)
		if fieldData != nil {
			return fieldData, identified_by
		}
		// If no resource level config found, try namespace level
		spec, found := globalSpec.Namespaces[namespace]
		if found {
			switch fieldType {
			case PrunerFieldTypeTTLSecondsAfterFinished:
				fieldData = spec.TTLSecondsAfterFinished

			case PrunerFieldTypeSuccessfulHistoryLimit:
				if spec.SuccessfulHistoryLimit != nil {
					fieldData = spec.SuccessfulHistoryLimit
				} else {
					fieldData = spec.HistoryLimit
				}

			case PrunerFieldTypeFailedHistoryLimit:
				if spec.FailedHistoryLimit != nil {
					fieldData = spec.FailedHistoryLimit
				} else {
					fieldData = spec.HistoryLimit
				}
			}
			identified_by = "identified_by_ns"
		} else {
			// If no namespace level config found, try global level
			switch fieldType {
			case PrunerFieldTypeTTLSecondsAfterFinished:
				fieldData = globalSpec.TTLSecondsAfterFinished

			case PrunerFieldTypeSuccessfulHistoryLimit:
				if globalSpec.SuccessfulHistoryLimit != nil {
					fieldData = globalSpec.SuccessfulHistoryLimit
				} else {
					fieldData = globalSpec.HistoryLimit
				}

			case PrunerFieldTypeFailedHistoryLimit:
				if globalSpec.FailedHistoryLimit != nil {
					fieldData = globalSpec.FailedHistoryLimit
				} else {
					fieldData = globalSpec.HistoryLimit
				}
			}
			identified_by = "identified_by_global"
		}
		return fieldData, identified_by
	case EnforcedConfigLevelNamespace:
		// First check namespace-level ConfigMap (tekton-pruner-namespace-spec) for selector matches
		fieldData, identified_by = getFromPrunerConfigResourceLevelwithSelector(namespaceConfigMap, namespace, name, selector, resourceType, fieldType)
		if fieldData != nil {
			return fieldData, identified_by
		}

		// Then check namespace-level ConfigMap root-level fields
		nsSpec, found := namespaceConfigMap[namespace]
		if found {
			switch fieldType {
			case PrunerFieldTypeTTLSecondsAfterFinished:
				fieldData = nsSpec.TTLSecondsAfterFinished

			case PrunerFieldTypeSuccessfulHistoryLimit:
				if nsSpec.SuccessfulHistoryLimit != nil {
					fieldData = nsSpec.SuccessfulHistoryLimit
				} else {
					fieldData = nsSpec.HistoryLimit
				}

			case PrunerFieldTypeFailedHistoryLimit:
				if nsSpec.FailedHistoryLimit != nil {
					fieldData = nsSpec.FailedHistoryLimit
				} else {
					fieldData = nsSpec.HistoryLimit
				}
			}
			if fieldData != nil {
				identified_by = "identified_by_ns_configmap"
				return fieldData, identified_by
			}
		}

		// Fall back to global spec, namespace root level
		spec, found := globalSpec.Namespaces[namespace]
		if found {
			switch fieldType {
			case PrunerFieldTypeTTLSecondsAfterFinished:
				fieldData = spec.TTLSecondsAfterFinished

			case PrunerFieldTypeSuccessfulHistoryLimit:
				if spec.SuccessfulHistoryLimit != nil {
					fieldData = spec.SuccessfulHistoryLimit
				} else {
					fieldData = spec.HistoryLimit
				}

			case PrunerFieldTypeFailedHistoryLimit:
				if spec.FailedHistoryLimit != nil {
					fieldData = spec.FailedHistoryLimit
				} else {
					fieldData = spec.HistoryLimit
				}
			}
			identified_by = "identified_by_ns"
		} else {
			// If no namespace level config found, try global level
			switch fieldType {
			case PrunerFieldTypeTTLSecondsAfterFinished:
				fieldData = globalSpec.TTLSecondsAfterFinished

			case PrunerFieldTypeSuccessfulHistoryLimit:
				if globalSpec.SuccessfulHistoryLimit != nil {
					fieldData = globalSpec.SuccessfulHistoryLimit
				} else {
					fieldData = globalSpec.HistoryLimit
				}

			case PrunerFieldTypeFailedHistoryLimit:
				if globalSpec.FailedHistoryLimit != nil {
					fieldData = globalSpec.FailedHistoryLimit
				} else {
					fieldData = globalSpec.HistoryLimit
				}
			}
			identified_by = "identified_by_global"
		}
		return fieldData, identified_by

	case EnforcedConfigLevelGlobal:
		// get it from global spec, root level
		switch fieldType {
		case PrunerFieldTypeTTLSecondsAfterFinished:
			fieldData = globalSpec.TTLSecondsAfterFinished

		case PrunerFieldTypeSuccessfulHistoryLimit:
			if globalSpec.SuccessfulHistoryLimit != nil {
				fieldData = globalSpec.SuccessfulHistoryLimit
			} else {
				fieldData = globalSpec.HistoryLimit
			}

		case PrunerFieldTypeFailedHistoryLimit:
			if globalSpec.FailedHistoryLimit != nil {
				fieldData = globalSpec.FailedHistoryLimit
			} else {
				fieldData = globalSpec.HistoryLimit
			}
		}
		identified_by = "identified_by_global"
	}

	return fieldData, identified_by
}

func (ps *prunerConfigStore) GetEnforcedConfigLevelFromNamespaceSpec(namespacesSpec map[string]NamespaceSpec, namespace, name string, selector SelectorSpec, resourceType PrunerResourceType) *EnforcedConfigLevel {
	var enforcedConfigLevel *EnforcedConfigLevel

	namespaceSpec, found := ps.globalConfig.Namespaces[namespace]
	if !found {
		return nil
	}

	// Get the appropriate resource specs based on type
	var resourceSpecs []ResourceSpec
	switch resourceType {
	case PrunerResourceTypePipelineRun:
		resourceSpecs = namespaceSpec.PipelineRuns
	case PrunerResourceTypeTaskRun:
		resourceSpecs = namespaceSpec.TaskRuns
	}

	// Try to find resource level config first
	if name != "" && (len(selector.MatchAnnotations) == 0 && len(selector.MatchLabels) == 0) {
		// Search by exact name
		for _, resourceSpec := range resourceSpecs {
			if resourceSpec.Name == name {
				enforcedConfigLevel = resourceSpec.EnforcedConfigLevel
				if enforcedConfigLevel != nil {
					return enforcedConfigLevel
				}
				break
			}
		}
	} else if len(selector.MatchAnnotations) > 0 || len(selector.MatchLabels) > 0 {
		// Search by selectors
		for _, resourceSpec := range resourceSpecs {
			for _, selectorSpec := range resourceSpec.Selector {
				annotationsMatch := true
				labelsMatch := true

				// Check if ConfigMap's required annotations exist in the resource
				if len(selectorSpec.MatchAnnotations) > 0 {
					if len(selector.MatchAnnotations) == 0 {
						// ConfigMap requires annotations but resource has none - no match
						annotationsMatch = false
					} else {
						// Check if all ConfigMap's required annotations exist in resource
						for key, value := range selectorSpec.MatchAnnotations {
							if resourceAnnotationValue, exists := selector.MatchAnnotations[key]; !exists || resourceAnnotationValue != value {
								annotationsMatch = false
								break
							}
						}
					}
				}

				// Check if ConfigMap's required labels exist in the resource
				if len(selectorSpec.MatchLabels) > 0 {
					if len(selector.MatchLabels) == 0 {
						// ConfigMap requires labels but resource has none - no match
						labelsMatch = false
					} else {
						// Check if all ConfigMap's required labels exist in resource
						for key, value := range selectorSpec.MatchLabels {
							if resourceLabelValue, exists := selector.MatchLabels[key]; !exists || resourceLabelValue != value {
								labelsMatch = false
								break
							}
						}
					}
				}

				// Both annotations and labels must match (AND logic)
				if annotationsMatch && labelsMatch {
					enforcedConfigLevel = resourceSpec.EnforcedConfigLevel
					if enforcedConfigLevel != nil {
						return enforcedConfigLevel
					}
					break
				}
			}
		}
	}

	// If no resource level config found or it was nil, return namespace level
	return namespaceSpec.EnforcedConfigLevel
}

func (ps *prunerConfigStore) getEnforcedConfigLevel(namespace, name string, selector SelectorSpec, resourceType PrunerResourceType) EnforcedConfigLevel {
	var enforcedConfigLevel *EnforcedConfigLevel

	// get it from global spec (order: resource level, namespace root level)
	enforcedConfigLevel = ps.GetEnforcedConfigLevelFromNamespaceSpec(ps.globalConfig.Namespaces, namespace, name, selector, resourceType)
	if enforcedConfigLevel != nil {
		return *enforcedConfigLevel
	}

	// get it from global spec, root level
	enforcedConfigLevel = ps.globalConfig.EnforcedConfigLevel
	if enforcedConfigLevel != nil {
		return *enforcedConfigLevel
	}

	// default level, if no where specified
	return EnforcedConfigLevelResource
}

func (ps *prunerConfigStore) GetPipelineEnforcedConfigLevel(namespace, name string, selector SelectorSpec) EnforcedConfigLevel {
	return ps.getEnforcedConfigLevel(namespace, name, selector, PrunerResourceTypePipelineRun)
}

func (ps *prunerConfigStore) GetTaskEnforcedConfigLevel(namespace, name string, selector SelectorSpec) EnforcedConfigLevel {
	return ps.getEnforcedConfigLevel(namespace, name, selector, PrunerResourceTypeTaskRun)
}

func (ps *prunerConfigStore) GetPipelineTTLSecondsAfterFinished(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetPipelineEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, ps.namespaceConfig, namespace, name, selector, PrunerResourceTypePipelineRun, PrunerFieldTypeTTLSecondsAfterFinished, enforcedConfigLevel)
}

func (ps *prunerConfigStore) GetPipelineSuccessHistoryLimitCount(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetPipelineEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, ps.namespaceConfig, namespace, name, selector, PrunerResourceTypePipelineRun, PrunerFieldTypeSuccessfulHistoryLimit, enforcedConfigLevel)
}

func (ps *prunerConfigStore) GetPipelineFailedHistoryLimitCount(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetPipelineEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, ps.namespaceConfig, namespace, name, selector, PrunerResourceTypePipelineRun, PrunerFieldTypeFailedHistoryLimit, enforcedConfigLevel)
}

func (ps *prunerConfigStore) GetTaskTTLSecondsAfterFinished(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetTaskEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, ps.namespaceConfig, namespace, name, selector, PrunerResourceTypeTaskRun, PrunerFieldTypeTTLSecondsAfterFinished, enforcedConfigLevel)
}

func (ps *prunerConfigStore) GetTaskSuccessHistoryLimitCount(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetTaskEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, ps.namespaceConfig, namespace, name, selector, PrunerResourceTypeTaskRun, PrunerFieldTypeSuccessfulHistoryLimit, enforcedConfigLevel)
}

func (ps *prunerConfigStore) GetTaskFailedHistoryLimitCount(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetTaskEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, ps.namespaceConfig, namespace, name, selector, PrunerResourceTypeTaskRun, PrunerFieldTypeFailedHistoryLimit, enforcedConfigLevel)
}

func ValidateConfigMap(cm *corev1.ConfigMap) error {
	return ValidateConfigMapWithGlobal(cm, nil)
}

// ValidateConfigMapWithGlobal validates a ConfigMap with optional global config for limit enforcement
// If globalConfigMap is provided and cm is a namespace-level config, it validates that namespace
// limits do not exceed global limits
func ValidateConfigMapWithGlobal(cm *corev1.ConfigMap, globalConfigMap *corev1.ConfigMap) error {
	if cm.Data == nil {
		return nil
	}

	// Parse global config if validating a global ConfigMap
	var globalLimits *PrunerConfig
	if cm.Data[PrunerGlobalConfigKey] != "" {
		globalConfig := &GlobalConfig{}
		if err := yaml.Unmarshal([]byte(cm.Data[PrunerGlobalConfigKey]), globalConfig); err != nil {
			return fmt.Errorf("failed to parse global-config: %w", err)
		}
		if err := validatePrunerConfig(&globalConfig.PrunerConfig, "global-config", nil); err != nil {
			return err
		}
		// Validate nested namespace configs within global config
		// These are validated against the global limits
		for ns, nsSpec := range globalConfig.Namespaces {
			if err := validatePrunerConfig(&nsSpec.PrunerConfig, "global-config.namespaces."+ns, &globalConfig.PrunerConfig); err != nil {
				return err
			}

			// CRITICAL: Validate that global ConfigMap namespace sections do NOT contain selectors
			// Selectors are ONLY supported in namespace-level ConfigMaps (tekton-pruner-namespace-spec)
			for i, pr := range nsSpec.PipelineRuns {
				if len(pr.Selector) > 0 {
					return fmt.Errorf("global-config.namespaces.%s.pipelineRuns[%d]: selectors are NOT supported in global ConfigMap. Use namespace-level ConfigMap (tekton-pruner-namespace-spec) instead", ns, i)
				}
			}
			for i, tr := range nsSpec.TaskRuns {
				if len(tr.Selector) > 0 {
					return fmt.Errorf("global-config.namespaces.%s.taskRuns[%d]: selectors are NOT supported in global ConfigMap. Use namespace-level ConfigMap (tekton-pruner-namespace-spec) instead", ns, i)
				}
			}
		}
		return nil
	}

	// Parse and validate namespace config against global limits
	if cm.Data[PrunerNamespaceConfigKey] != "" {
		namespaceConfig := &NamespaceSpec{}
		if err := yaml.Unmarshal([]byte(cm.Data[PrunerNamespaceConfigKey]), namespaceConfig); err != nil {
			return fmt.Errorf("failed to parse ns-config: %w", err)
		}

		// Extract global limits if global config is provided
		if globalConfigMap != nil && globalConfigMap.Data != nil && globalConfigMap.Data[PrunerGlobalConfigKey] != "" {
			globalConfig := &GlobalConfig{}
			if err := yaml.Unmarshal([]byte(globalConfigMap.Data[PrunerGlobalConfigKey]), globalConfig); err != nil {
				// If we can't parse global config, just do basic validation
				return validatePrunerConfig(&namespaceConfig.PrunerConfig, "ns-config", nil)
			}
			globalLimits = &globalConfig.PrunerConfig
		}

		// Validate namespace config, enforcing global limits if available
		if err := validatePrunerConfig(&namespaceConfig.PrunerConfig, "ns-config", globalLimits); err != nil {
			return err
		}

		// Validate selector-based limits (sum of selectors must not exceed namespace/global limits)
		// Extract namespace name from ConfigMap metadata
		namespace := cm.Namespace
		var globalNamespaceSpec *NamespaceSpec
		if globalConfigMap != nil && globalConfigMap.Data != nil && globalConfigMap.Data[PrunerGlobalConfigKey] != "" {
			globalConfig := &GlobalConfig{}
			if err := yaml.Unmarshal([]byte(globalConfigMap.Data[PrunerGlobalConfigKey]), globalConfig); err == nil {
				if nsSpec, exists := globalConfig.Namespaces[namespace]; exists {
					globalNamespaceSpec = &nsSpec
				}
				// Pass both globalConfig and globalNamespaceSpec for 4-tier hierarchy
				if err := validateSelectorLimits(namespaceConfig, &globalConfig.PrunerConfig, globalNamespaceSpec, namespace); err != nil {
					return err
				}
			}
		} else {
			// No global config, validate with system maximum only
			if err := validateSelectorLimits(namespaceConfig, nil, nil, namespace); err != nil {
				return err
			}
		}
	}

	return nil
}

// validatePrunerConfig validates the fields of a PrunerConfig
// If globalConfig is provided, namespace-level settings are validated to not exceed global limits
// If globalConfig is nil and path indicates a namespace config, system maximums are enforced
func validatePrunerConfig(config *PrunerConfig, path string, globalConfig *PrunerConfig) error {
	if config == nil {
		return nil
	}

	// Determine if this is a namespace-level config validation (not a top-level global config)
	// Namespace configs can be:
	// - Standalone: path starts with "ns-config"
	// - Nested in global: path contains ".namespaces."
	isNamespaceConfig := strings.HasPrefix(path, "ns-config") || strings.Contains(path, ".namespaces.")

	// Validate EnforcedConfigLevel
	if config.EnforcedConfigLevel != nil {
		level := *config.EnforcedConfigLevel
		if level != EnforcedConfigLevelGlobal &&
			level != EnforcedConfigLevelNamespace &&
			level != EnforcedConfigLevelResource {
			return fmt.Errorf("%s: invalid enforcedConfigLevel '%s', must be one of: global, namespace, resource", path, level)
		}
	}

	// Validate TTLSecondsAfterFinished
	if config.TTLSecondsAfterFinished != nil {
		if *config.TTLSecondsAfterFinished < 0 {
			return fmt.Errorf("%s: ttlSecondsAfterFinished cannot be negative, got %d", path, *config.TTLSecondsAfterFinished)
		}
		// Namespace config cannot have longer TTL than global config
		if globalConfig != nil && globalConfig.TTLSecondsAfterFinished != nil {
			if *config.TTLSecondsAfterFinished > *globalConfig.TTLSecondsAfterFinished {
				return fmt.Errorf("%s: ttlSecondsAfterFinished (%d) cannot exceed global limit (%d)",
					path, *config.TTLSecondsAfterFinished, *globalConfig.TTLSecondsAfterFinished)
			}
		} else if isNamespaceConfig && (globalConfig == nil || globalConfig.TTLSecondsAfterFinished == nil) {
			// If this is a namespace config and no global limit is set, enforce system maximum
			if *config.TTLSecondsAfterFinished > MaxTTLSecondsAfterFinished {
				return fmt.Errorf("%s: ttlSecondsAfterFinished (%d) cannot exceed system maximum (%d seconds / 30 days)",
					path, *config.TTLSecondsAfterFinished, MaxTTLSecondsAfterFinished)
			}
		}
	}

	// Validate SuccessfulHistoryLimit
	if config.SuccessfulHistoryLimit != nil {
		if *config.SuccessfulHistoryLimit < 0 {
			return fmt.Errorf("%s: successfulHistoryLimit cannot be negative, got %d", path, *config.SuccessfulHistoryLimit)
		}
		// For namespace configs, determine the upper limit based on global config
		if isNamespaceConfig && globalConfig != nil {
			// Priority 1: Use global successfulHistoryLimit if set
			if globalConfig.SuccessfulHistoryLimit != nil {
				if *config.SuccessfulHistoryLimit > *globalConfig.SuccessfulHistoryLimit {
					return fmt.Errorf("%s: successfulHistoryLimit (%d) cannot exceed global limit (%d)",
						path, *config.SuccessfulHistoryLimit, *globalConfig.SuccessfulHistoryLimit)
				}
			} else if globalConfig.HistoryLimit != nil {
				// Priority 2: Use global historyLimit as fallback if no granular limit
				if *config.SuccessfulHistoryLimit > *globalConfig.HistoryLimit {
					return fmt.Errorf("%s: successfulHistoryLimit (%d) cannot exceed global historyLimit (%d)",
						path, *config.SuccessfulHistoryLimit, *globalConfig.HistoryLimit)
				}
			} else {
				// Priority 3: Use system maximum if global config exists but has no relevant limits
				if *config.SuccessfulHistoryLimit > MaxHistoryLimit {
					return fmt.Errorf("%s: successfulHistoryLimit (%d) cannot exceed system maximum (%d)",
						path, *config.SuccessfulHistoryLimit, MaxHistoryLimit)
				}
			}
		} else if isNamespaceConfig && globalConfig == nil {
			// Priority 3: Use system maximum if no global config at all
			if *config.SuccessfulHistoryLimit > MaxHistoryLimit {
				return fmt.Errorf("%s: successfulHistoryLimit (%d) cannot exceed system maximum (%d)",
					path, *config.SuccessfulHistoryLimit, MaxHistoryLimit)
			}
		}
	}

	// Validate FailedHistoryLimit
	if config.FailedHistoryLimit != nil {
		if *config.FailedHistoryLimit < 0 {
			return fmt.Errorf("%s: failedHistoryLimit cannot be negative, got %d", path, *config.FailedHistoryLimit)
		}
		// For namespace configs, determine the upper limit based on global config
		if isNamespaceConfig && globalConfig != nil {
			// Priority 1: Use global failedHistoryLimit if set
			if globalConfig.FailedHistoryLimit != nil {
				if *config.FailedHistoryLimit > *globalConfig.FailedHistoryLimit {
					return fmt.Errorf("%s: failedHistoryLimit (%d) cannot exceed global limit (%d)",
						path, *config.FailedHistoryLimit, *globalConfig.FailedHistoryLimit)
				}
			} else if globalConfig.HistoryLimit != nil {
				// Priority 2: Use global historyLimit as fallback if no granular limit
				if *config.FailedHistoryLimit > *globalConfig.HistoryLimit {
					return fmt.Errorf("%s: failedHistoryLimit (%d) cannot exceed global historyLimit (%d)",
						path, *config.FailedHistoryLimit, *globalConfig.HistoryLimit)
				}
			} else {
				// Priority 3: Use system maximum if global config exists but has no relevant limits
				if *config.FailedHistoryLimit > MaxHistoryLimit {
					return fmt.Errorf("%s: failedHistoryLimit (%d) cannot exceed system maximum (%d)",
						path, *config.FailedHistoryLimit, MaxHistoryLimit)
				}
			}
		} else if isNamespaceConfig && globalConfig == nil {
			// Priority 3: Use system maximum if no global config at all
			if *config.FailedHistoryLimit > MaxHistoryLimit {
				return fmt.Errorf("%s: failedHistoryLimit (%d) cannot exceed system maximum (%d)",
					path, *config.FailedHistoryLimit, MaxHistoryLimit)
			}
		}
	}

	// Validate HistoryLimit
	if config.HistoryLimit != nil {
		if *config.HistoryLimit < 0 {
			return fmt.Errorf("%s: historyLimit cannot be negative, got %d", path, *config.HistoryLimit)
		}
		// For namespace configs, validate against global historyLimit
		if isNamespaceConfig && globalConfig != nil && globalConfig.HistoryLimit != nil {
			if *config.HistoryLimit > *globalConfig.HistoryLimit {
				return fmt.Errorf("%s: historyLimit (%d) cannot exceed global limit (%d)",
					path, *config.HistoryLimit, *globalConfig.HistoryLimit)
			}
		} else if isNamespaceConfig && (globalConfig == nil || globalConfig.HistoryLimit == nil) {
			// Use system maximum if no global historyLimit is set
			if *config.HistoryLimit > MaxHistoryLimit {
				return fmt.Errorf("%s: historyLimit (%d) cannot exceed system maximum (%d)",
					path, *config.HistoryLimit, MaxHistoryLimit)
			}
		}
	}

	return nil
}

// validateSelectorLimits validates that the sum of selector-based limits does not exceed the allowed upper bound
// Uses a 4-tier hierarchy to determine the upper bound:
// 1. Namespace-level spec (in the same namespace config)
// 2. Global namespace override (from global.namespaces[namespace])
// 3. Global default spec
// 4. System maximum
func validateSelectorLimits(nsConfig *NamespaceSpec, globalConfig *PrunerConfig, globalNsSpec *NamespaceSpec, namespace string) error {
	if nsConfig == nil {
		return nil
	}

	// Validate PipelineRuns selectors
	if err := validateResourceSelectorLimits(nsConfig.PipelineRuns, &nsConfig.PrunerConfig, globalConfig, globalNsSpec, namespace, "pipelineRuns"); err != nil {
		return err
	}

	// Validate TaskRuns selectors
	if err := validateResourceSelectorLimits(nsConfig.TaskRuns, &nsConfig.PrunerConfig, globalConfig, globalNsSpec, namespace, "taskRuns"); err != nil {
		return err
	}

	return nil
}

// validateResourceSelectorLimits validates selector limits for a specific resource type (PipelineRuns or TaskRuns)
func validateResourceSelectorLimits(resources []ResourceSpec, nsConfig *PrunerConfig, globalConfig *PrunerConfig, globalNsSpec *NamespaceSpec, namespace, resourceType string) error {
	if len(resources) == 0 {
		return nil
	}

	// Calculate sum of selector-based limits for each limit type
	var sumSuccessful, sumFailed, sumHistory int32

	for i, resource := range resources {
		// Only count resources that have selectors (not name-based)
		if len(resource.Selector) > 0 {
			if resource.SuccessfulHistoryLimit != nil {
				sumSuccessful += *resource.SuccessfulHistoryLimit
			}
			if resource.FailedHistoryLimit != nil {
				sumFailed += *resource.FailedHistoryLimit
			}
			if resource.HistoryLimit != nil {
				sumHistory += *resource.HistoryLimit
			}
		}

		// Validate individual selector limits are non-negative
		if resource.SuccessfulHistoryLimit != nil && *resource.SuccessfulHistoryLimit < 0 {
			return fmt.Errorf("ns-config.%s[%d]: successfulHistoryLimit cannot be negative, got %d", resourceType, i, *resource.SuccessfulHistoryLimit)
		}
		if resource.FailedHistoryLimit != nil && *resource.FailedHistoryLimit < 0 {
			return fmt.Errorf("ns-config.%s[%d]: failedHistoryLimit cannot be negative, got %d", resourceType, i, *resource.FailedHistoryLimit)
		}
		if resource.HistoryLimit != nil && *resource.HistoryLimit < 0 {
			return fmt.Errorf("ns-config.%s[%d]: historyLimit cannot be negative, got %d", resourceType, i, *resource.HistoryLimit)
		}
	}

	// Validate successfulHistoryLimit sum
	if sumSuccessful > 0 {
		upperBound := determineUpperBound(nsConfig.SuccessfulHistoryLimit, nsConfig.HistoryLimit,
			globalNsSpec, globalConfig, "successfulHistoryLimit")
		if sumSuccessful > upperBound {
			return fmt.Errorf("namespace '%s' ns-config.%s: sum of selector successfulHistoryLimit (%d) cannot exceed upper bound (%d)",
				namespace, resourceType, sumSuccessful, upperBound)
		}
	}

	// Validate failedHistoryLimit sum
	if sumFailed > 0 {
		upperBound := determineUpperBound(nsConfig.FailedHistoryLimit, nsConfig.HistoryLimit,
			globalNsSpec, globalConfig, "failedHistoryLimit")
		if sumFailed > upperBound {
			return fmt.Errorf("namespace '%s' ns-config.%s: sum of selector failedHistoryLimit (%d) cannot exceed upper bound (%d)",
				namespace, resourceType, sumFailed, upperBound)
		}
	}

	// Validate historyLimit sum
	if sumHistory > 0 {
		upperBound := determineUpperBound(nsConfig.HistoryLimit, nil,
			globalNsSpec, globalConfig, "historyLimit")
		if sumHistory > upperBound {
			return fmt.Errorf("namespace '%s' ns-config.%s: sum of selector historyLimit (%d) cannot exceed upper bound (%d)",
				namespace, resourceType, sumHistory, upperBound)
		}
	}

	return nil
}

// determineUpperBound implements the 4-tier hierarchy to find the upper bound for selector validation
// limitType should be "successfulHistoryLimit", "failedHistoryLimit", or "historyLimit"
func determineUpperBound(nsGranularLimit, nsHistoryLimit *int32, globalNsSpec *NamespaceSpec, globalConfig *PrunerConfig, limitType string) int32 {
	// Level 1: Namespace-level spec (most specific)
	if nsGranularLimit != nil && limitType != "historyLimit" {
		return *nsGranularLimit
	}
	if limitType != "historyLimit" && nsHistoryLimit != nil {
		// For granular limits, fallback to namespace historyLimit if granular not set
		return *nsHistoryLimit
	}
	if limitType == "historyLimit" && nsHistoryLimit != nil {
		return *nsHistoryLimit
	}

	// Level 2: Global namespace override (from global.namespaces[namespace])
	if globalNsSpec != nil {
		switch limitType {
		case "successfulHistoryLimit":
			if globalNsSpec.SuccessfulHistoryLimit != nil {
				return *globalNsSpec.SuccessfulHistoryLimit
			}
			// Fallback to globalNsSpec.HistoryLimit
			if globalNsSpec.HistoryLimit != nil {
				return *globalNsSpec.HistoryLimit
			}
		case "failedHistoryLimit":
			if globalNsSpec.FailedHistoryLimit != nil {
				return *globalNsSpec.FailedHistoryLimit
			}
			// Fallback to globalNsSpec.HistoryLimit
			if globalNsSpec.HistoryLimit != nil {
				return *globalNsSpec.HistoryLimit
			}
		case "historyLimit":
			if globalNsSpec.HistoryLimit != nil {
				return *globalNsSpec.HistoryLimit
			}
		}
	}

	// Level 3: Global default spec
	if globalConfig != nil {
		switch limitType {
		case "successfulHistoryLimit":
			if globalConfig.SuccessfulHistoryLimit != nil {
				return *globalConfig.SuccessfulHistoryLimit
			}
			// Fallback to globalConfig.HistoryLimit
			if globalConfig.HistoryLimit != nil {
				return *globalConfig.HistoryLimit
			}
		case "failedHistoryLimit":
			if globalConfig.FailedHistoryLimit != nil {
				return *globalConfig.FailedHistoryLimit
			}
			// Fallback to globalConfig.HistoryLimit
			if globalConfig.HistoryLimit != nil {
				return *globalConfig.HistoryLimit
			}
		case "historyLimit":
			if globalConfig.HistoryLimit != nil {
				return *globalConfig.HistoryLimit
			}
		}
	}

	// Level 4: System maximum
	return int32(MaxHistoryLimit)
}
