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
		// Name was specified but no match found - no fallback to selectors (field isolation)
		return nil, ""
	}

	// If name is not provided, proceed with selector matching
	if len(selector.MatchAnnotations) > 0 || len(selector.MatchLabels) > 0 {

		for _, resourceSpec := range resourceSpecs {
			// Check if the resourceSpec matches the provided selector by annotations AND labels
			for _, selectorSpec := range resourceSpec.Selector {
				// Both annotations and labels must match when both are specified (AND logic)
				// If ResourceSpec has both, selector must also provide both
				annotationsMatch := true
				labelsMatch := true

				// If ResourceSpec has annotations, check if selector provides matching annotations
				if len(selectorSpec.MatchAnnotations) > 0 {
					if len(selector.MatchAnnotations) == 0 {
						// ResourceSpec has annotations but selector doesn't - no match
						annotationsMatch = false
					} else {
						// Check if all selector annotations match
						for key, value := range selector.MatchAnnotations {
							if resourceAnnotationValue, exists := selectorSpec.MatchAnnotations[key]; !exists || resourceAnnotationValue != value {
								annotationsMatch = false
								break
							}
						}
					}
				}

				// If ResourceSpec has labels, check if selector provides matching labels
				if len(selectorSpec.MatchLabels) > 0 {
					if len(selector.MatchLabels) == 0 {
						// ResourceSpec has labels but selector doesn't - no match
						labelsMatch = false
					} else {
						// Check if all selector labels match
						for key, value := range selector.MatchLabels {
							if resourceLabelValue, exists := selectorSpec.MatchLabels[key]; !exists || resourceLabelValue != value {
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
				// Try annotation matching first
				if len(selector.MatchAnnotations) > 0 {
					match := true
					for key, value := range selector.MatchAnnotations {
						if resourceAnnotationValue, exists := selectorSpec.MatchAnnotations[key]; !exists || resourceAnnotationValue != value {
							match = false
							break
						}
					}
					if match {
						enforcedConfigLevel = resourceSpec.EnforcedConfigLevel
						if enforcedConfigLevel != nil {
							return enforcedConfigLevel
						}
						break
					}
				}

				// Try label matching if no annotation match
				if len(selector.MatchLabels) > 0 {
					match := true
					for key, value := range selector.MatchLabels {
						if resourceLabelValue, exists := selectorSpec.MatchLabels[key]; !exists || resourceLabelValue != value {
							match = false
							break
						}
					}
					if match {
						enforcedConfigLevel = resourceSpec.EnforcedConfigLevel
						if enforcedConfigLevel != nil {
							return enforcedConfigLevel
						}
						break
					}
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
			return fmt.Errorf("failed to parse namespace-config: %w", err)
		}

		// Extract global limits if global config is provided
		if globalConfigMap != nil && globalConfigMap.Data != nil && globalConfigMap.Data[PrunerGlobalConfigKey] != "" {
			globalConfig := &GlobalConfig{}
			if err := yaml.Unmarshal([]byte(globalConfigMap.Data[PrunerGlobalConfigKey]), globalConfig); err != nil {
				// If we can't parse global config, just do basic validation
				return validatePrunerConfig(&namespaceConfig.PrunerConfig, "namespace-config", nil)
			}
			globalLimits = &globalConfig.PrunerConfig
		}

		// Validate namespace config, enforcing global limits if available
		if err := validatePrunerConfig(&namespaceConfig.PrunerConfig, "namespace-config", globalLimits); err != nil {
			return err
		}
	}

	return nil
}

// validatePrunerConfig validates the fields of a PrunerConfig
// If globalConfig is provided, namespace-level settings are validated to not exceed global limits
func validatePrunerConfig(config *PrunerConfig, path string, globalConfig *PrunerConfig) error {
	if config == nil {
		return nil
	}

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
		} else if globalConfig == nil || globalConfig.TTLSecondsAfterFinished == nil {
			// If no global limit is set, enforce system maximum
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
		// Namespace config cannot retain more successful runs than global config
		if globalConfig != nil && globalConfig.SuccessfulHistoryLimit != nil {
			if *config.SuccessfulHistoryLimit > *globalConfig.SuccessfulHistoryLimit {
				return fmt.Errorf("%s: successfulHistoryLimit (%d) cannot exceed global limit (%d)",
					path, *config.SuccessfulHistoryLimit, *globalConfig.SuccessfulHistoryLimit)
			}
		} else if globalConfig == nil || globalConfig.SuccessfulHistoryLimit == nil {
			// If no global limit is set, enforce system maximum
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
		// Namespace config cannot retain more failed runs than global config
		if globalConfig != nil && globalConfig.FailedHistoryLimit != nil {
			if *config.FailedHistoryLimit > *globalConfig.FailedHistoryLimit {
				return fmt.Errorf("%s: failedHistoryLimit (%d) cannot exceed global limit (%d)",
					path, *config.FailedHistoryLimit, *globalConfig.FailedHistoryLimit)
			}
		} else if globalConfig == nil || globalConfig.FailedHistoryLimit == nil {
			// If no global limit is set, enforce system maximum
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
		// Namespace config cannot retain more runs than global config
		if globalConfig != nil && globalConfig.HistoryLimit != nil {
			if *config.HistoryLimit > *globalConfig.HistoryLimit {
				return fmt.Errorf("%s: historyLimit (%d) cannot exceed global limit (%d)",
					path, *config.HistoryLimit, *globalConfig.HistoryLimit)
			}
		} else if globalConfig == nil || globalConfig.HistoryLimit == nil {
			// If no global limit is set, enforce system maximum
			if *config.HistoryLimit > MaxHistoryLimit {
				return fmt.Errorf("%s: historyLimit (%d) cannot exceed system maximum (%d)",
					path, *config.HistoryLimit, MaxHistoryLimit)
			}
		}
	}

	return nil
}
