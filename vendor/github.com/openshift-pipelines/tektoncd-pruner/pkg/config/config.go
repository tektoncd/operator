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
type ResourceSpec struct {
	// EnforcedConfigLevel allowed values: global, namespace, resource (default: resource)
	Name                    string               `yaml:"name"`               // Exact name of the parent Pipeline or Task
	Selector                []SelectorSpec       `yaml:"selector,omitempty"` // Supports selection based on labels and annotations. If Name is given, Name taskes precedence
	EnforcedConfigLevel     *EnforcedConfigLevel `yaml:"enforcedConfigLevel"`
	TTLSecondsAfterFinished *int32               `yaml:"ttlSecondsAfterFinished"`
	SuccessfulHistoryLimit  *int32               `yaml:"successfulHistoryLimit"`
	FailedHistoryLimit      *int32               `yaml:"failedHistoryLimit"`
	HistoryLimit            *int32               `yaml:"historyLimit"`
}

// SelectorSpec allows specifying selectors for matching resources like PipelineRun or TaskRun
type SelectorSpec struct {
	// Match by labels or Annotations. If both are specified, Annotations will take priority.
	MatchLabels      map[string]string `yaml:"matchLabels,omitempty"`
	MatchAnnotations map[string]string `yaml:"matchAnnotations,omitempty"`
}

// NamespaceSpec is used to hold the pruning config of a specific namespace and its resources
type NamespaceSpec struct {
	// EnforcedConfigLevel allowed values: global, namespace, resource (default: resource)
	EnforcedConfigLevel     *EnforcedConfigLevel `yaml:"enforcedConfigLevel"`
	TTLSecondsAfterFinished *int32               `yaml:"ttlSecondsAfterFinished"`
	SuccessfulHistoryLimit  *int32               `yaml:"successfulHistoryLimit"`
	FailedHistoryLimit      *int32               `yaml:"failedHistoryLimit"`
	HistoryLimit            *int32               `yaml:"historyLimit"`
	PipelineRuns            []ResourceSpec       `yaml:"pipelineRuns"`
	TaskRuns                []ResourceSpec       `yaml:"taskRuns"`
}

// PrunerConfig used to hold the cluster-wide pruning config as well as namespace specific pruning config
type PrunerConfig struct {
	// EnforcedConfigLevel allowed values: global, namespace, resource (default: resource)
	EnforcedConfigLevel     *EnforcedConfigLevel     `yaml:"enforcedConfigLevel"`
	TTLSecondsAfterFinished *int32                   `yaml:"ttlSecondsAfterFinished"`
	SuccessfulHistoryLimit  *int32                   `yaml:"successfulHistoryLimit"`
	FailedHistoryLimit      *int32                   `yaml:"failedHistoryLimit"`
	HistoryLimit            *int32                   `yaml:"historyLimit"`
	Namespaces              map[string]NamespaceSpec `yaml:"namespaces"`
}

// prunerConfigStore defines the store structure to hold config from ConfigMap
type prunerConfigStore struct {
	mutex        sync.RWMutex
	globalConfig PrunerConfig
}

var (
	// PrunerConfigStore is the singleton instance to store pruner config
	PrunerConfigStore = prunerConfigStore{mutex: sync.RWMutex{}}
)

// loads config from configMap (global-config) should be called on startup and if there is a change detected on the ConfigMap
func (ps *prunerConfigStore) LoadGlobalConfig(ctx context.Context, configMap *corev1.ConfigMap) error {
	logger := logging.FromContext(ctx)
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Log the current state of globalConfig and namespacedConfig before updating
	logger.Debugw("Loading global config", "oldGlobalConfig", ps.globalConfig)

	globalConfig := &PrunerConfig{}
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

	// First, check if name is provided, and use it to match exactly
	if name != "" && (len(selector.MatchAnnotations) == 0 || len(selector.MatchLabels) == 0) {
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
	} else if len(selector.MatchAnnotations) > 0 || len(selector.MatchLabels) > 0 {
		// If name is not provided, we proceed with selector matching

		for _, resourceSpec := range resourceSpecs {
			// Check if the resourceSpec matches the provided selector by annotations or labels
			for _, selectorSpec := range resourceSpec.Selector {
				// Match by annotations if provided in the selector
				if len(selector.MatchAnnotations) > 0 {
					match := true
					for key, value := range selector.MatchAnnotations {
						if resourceAnnotationValue, exists := selectorSpec.MatchAnnotations[key]; !exists || resourceAnnotationValue != value {
							match = false
							break
						}
					}
					if match {
						// Return the field value if annotations match
						switch fieldType {
						case PrunerFieldTypeTTLSecondsAfterFinished:
							return resourceSpec.TTLSecondsAfterFinished, "identifiedBy_resource_ann"
						case PrunerFieldTypeSuccessfulHistoryLimit:
							if resourceSpec.SuccessfulHistoryLimit != nil {
								return resourceSpec.SuccessfulHistoryLimit, "identifiedBy_resource_ann"
							} else {
								return resourceSpec.HistoryLimit, "identifiedBy_resource_ann"
							}
						case PrunerFieldTypeFailedHistoryLimit:
							if resourceSpec.SuccessfulHistoryLimit != nil {
								return resourceSpec.FailedHistoryLimit, "identifiedBy_resource_ann"
							} else {
								return resourceSpec.HistoryLimit, "identifiedBy_resource_ann"
							}
						}
					}
				}
				// Match by labels if provided in the selector
				if len(selector.MatchLabels) > 0 {
					match := true
					for key, value := range selector.MatchLabels {
						if resourceLabelValue, exists := selectorSpec.MatchLabels[key]; !exists || resourceLabelValue != value {
							match = false
							break
						}
					}
					if match {
						// Return the field value if labels match
						switch fieldType {
						case PrunerFieldTypeTTLSecondsAfterFinished:
							return resourceSpec.TTLSecondsAfterFinished, "identifiedBy_resource_label"
						case PrunerFieldTypeSuccessfulHistoryLimit:
							if resourceSpec.SuccessfulHistoryLimit != nil {
								return resourceSpec.SuccessfulHistoryLimit, "identifiedBy_resource_label"
							} else {
								return resourceSpec.HistoryLimit, "identifiedBy_resource_label"
							}
						case PrunerFieldTypeFailedHistoryLimit:
							if resourceSpec.SuccessfulHistoryLimit != nil {
								return resourceSpec.FailedHistoryLimit, "identifiedBy_resource_label"
							} else {
								return resourceSpec.HistoryLimit, "identifiedBy_resource_label"
							}
						}
					}
				}
			}
		}
	}

	// If no match found, return nil
	return nil, ""
}

func getResourceFieldData(globalSpec PrunerConfig, namespace, name string, selector SelectorSpec, resourceType PrunerResourceType, fieldType PrunerFieldType, enforcedConfigLevel EnforcedConfigLevel) (*int32, string) {
	var fieldData *int32
	var identified_by string

	switch enforcedConfigLevel {
	case EnforcedConfigLevelResource:
		// get from resource level
		fieldData, identified_by = getFromPrunerConfigResourceLevelwithSelector(globalSpec.Namespaces, namespace, name, selector, resourceType, fieldType)
		fallthrough

	case EnforcedConfigLevelNamespace:
		if fieldData == nil {
			// get it from global spec, namespace root level
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
			}
			identified_by = "identified_by_ns"
		}
		fallthrough

	case EnforcedConfigLevelGlobal:
		if fieldData == nil {
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
		}
		identified_by = "identified_by_global"

	}

	return fieldData, identified_by
}

func (ps *prunerConfigStore) GetEnforcedConfigLevelFromNamespaceSpec(namespacesSpec map[string]NamespaceSpec, namespace, name string, selector SelectorSpec, resourceType PrunerResourceType) *EnforcedConfigLevel {
	var enforcedConfigLevel *EnforcedConfigLevel
	var resourceSpecs []ResourceSpec
	var namespaceSpec NamespaceSpec
	var found bool

	namespaceSpec, found = ps.globalConfig.Namespaces[namespace]
	if found {
		switch resourceType {
		case PrunerResourceTypePipelineRun:
			resourceSpecs = namespaceSpec.PipelineRuns

		case PrunerResourceTypeTaskRun:
			resourceSpecs = namespaceSpec.TaskRuns
		}

		if name != "" {
			for _, resourceSpec := range resourceSpecs {
				if resourceSpec.Name == name {
					// if found on resource level
					enforcedConfigLevel = resourceSpec.EnforcedConfigLevel
					if enforcedConfigLevel != nil {
						return enforcedConfigLevel
					}
					break
				}
			}
		} else if len(selector.MatchAnnotations) > 0 || len(selector.MatchLabels) > 0 {
			// If name is not provided, we proceed with selector matching

			for _, resourceSpec := range resourceSpecs {
				// Check if the resourceSpec matches the provided selector by annotations or labels
				for _, selectorSpec := range resourceSpec.Selector {
					// Match by annotations if provided in the selector
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
					// Match by labels if provided in the selector
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
		// get it from namespace root level
		enforcedConfigLevel = namespaceSpec.EnforcedConfigLevel
		if enforcedConfigLevel != nil {
			return enforcedConfigLevel
		}
	}
	return nil //yet to include a default enforced config global option. For now,only if it set in teh config map this would work
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
	return getResourceFieldData(ps.globalConfig, namespace, name, selector, PrunerResourceTypePipelineRun, PrunerFieldTypeTTLSecondsAfterFinished, enforcedConfigLevel)
}

func (ps *prunerConfigStore) GetPipelineSuccessHistoryLimitCount(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetPipelineEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, namespace, name, selector, PrunerResourceTypePipelineRun, PrunerFieldTypeSuccessfulHistoryLimit, enforcedConfigLevel)
}

func (ps *prunerConfigStore) GetPipelineFailedHistoryLimitCount(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetPipelineEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, namespace, name, selector, PrunerResourceTypePipelineRun, PrunerFieldTypeFailedHistoryLimit, enforcedConfigLevel)
}

func (ps *prunerConfigStore) GetTaskTTLSecondsAfterFinished(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetTaskEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, namespace, name, selector, PrunerResourceTypeTaskRun, PrunerFieldTypeTTLSecondsAfterFinished, enforcedConfigLevel)
}

func (ps *prunerConfigStore) GetTaskSuccessHistoryLimitCount(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetTaskEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, namespace, name, selector, PrunerResourceTypeTaskRun, PrunerFieldTypeSuccessfulHistoryLimit, enforcedConfigLevel)
}

func (ps *prunerConfigStore) GetTaskFailedHistoryLimitCount(namespace, name string, selector SelectorSpec) (*int32, string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	enforcedConfigLevel := ps.GetTaskEnforcedConfigLevel(namespace, name, selector)
	return getResourceFieldData(ps.globalConfig, namespace, name, selector, PrunerResourceTypeTaskRun, PrunerFieldTypeFailedHistoryLimit, enforcedConfigLevel)
}
