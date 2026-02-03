/*
Copyright 2022 The Tekton Authors

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
	"reflect"
	"strconv"
	"strings"
	"sync"

	pacSettings "github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

func (pac *OpenShiftPipelinesAsCode) SetDefaults(ctx context.Context) {
	if pac.Spec.PACSettings.Settings == nil {
		pac.Spec.PACSettings.Settings = map[string]string{}
	}
	if pac.Spec.PACSettings.AdditionalPACControllers == nil {
		pac.Spec.PACSettings.AdditionalPACControllers = map[string]AdditionalPACControllerConfig{}
	}
	logger := logging.FromContext(ctx)
	pac.Spec.PACSettings.setPACDefaults(logger)
}

// settingsEqual compares two Settings maps for equality
func settingsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || v != bv {
			return false
		}
	}
	return true
}

func (set *PACSettings) setPACDefaults(logger *zap.SugaredLogger) {
	if set.Settings == nil {
		set.Settings = map[string]string{}
	}
	defaultPacSettings := pacSettings.Settings{}

	err := pacSettings.SyncConfig(logger, &defaultPacSettings, set.Settings, map[string]func(string) error{})
	if err != nil {
		logger.Error("error on applying default PAC settings", err)
	}

	// Remove tektonhub catalog to only keep artifacthub
	defaultPacSettings.HubCatalogs.Delete("tektonhub")

	// Only reassign Settings map if values actually changed
	newSettings := ConvertPacStructToConfigMap(&defaultPacSettings)
	if !settingsEqual(set.Settings, newSettings) {
		set.Settings = newSettings
	}
	setAdditionalPACControllerDefault(set.AdditionalPACControllers)
}

// Set the default values for additional PAc controller resources
func setAdditionalPACControllerDefault(additionalPACController map[string]AdditionalPACControllerConfig) {
	for name, additionalPACInfo := range additionalPACController {
		if additionalPACInfo.Enable == nil {
			additionalPACInfo.Enable = ptr.Bool(true)
		}
		if additionalPACInfo.ConfigMapName == "" {
			additionalPACInfo.ConfigMapName = fmt.Sprintf("%s-pipelines-as-code-configmap", name)
		}
		if additionalPACInfo.SecretName == "" {
			additionalPACInfo.SecretName = fmt.Sprintf("%s-pipelines-as-code-secret", name)
		}
		additionalPACController[name] = additionalPACInfo
	}
}

func ConvertPacStructToConfigMap(settings *pacSettings.Settings) map[string]string {
	config := map[string]string{}
	if settings == nil {
		return config
	}
	structValue := reflect.ValueOf(settings).Elem()
	structType := reflect.TypeOf(settings).Elem()

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldName := field.Name

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		key := strings.ToLower(jsonTag)
		element := structValue.FieldByName(fieldName)
		if !element.IsValid() {
			continue
		}

		//nolint
		switch field.Type.Kind() {
		case reflect.String:
			config[key] = element.String()
		case reflect.Bool:
			config[key] = strconv.FormatBool(element.Bool())
		case reflect.Int:
			config[key] = strconv.FormatInt(element.Int(), 10)
		case reflect.Ptr:
			// for hub catalogs map
			if key == "" {
				data := element.Interface().(*sync.Map)
				data.Range(func(key, value any) bool {
					catalogData := value.(pacSettings.HubCatalog)
					if key == "default" {
						config[pacSettings.HubURLKey] = catalogData.URL
						config[pacSettings.HubCatalogTypeKey] = catalogData.Type
						if catalogData.Name != "" {
							config[pacSettings.HubCatalogNameKey] = catalogData.Name
						}
						return true
					}
					config[fmt.Sprintf("%s-%s-%s", "catalog", catalogData.Index, "id")] = key.(string)
					config[fmt.Sprintf("%s-%s-%s", "catalog", catalogData.Index, "name")] = catalogData.Name
					config[fmt.Sprintf("%s-%s-%s", "catalog", catalogData.Index, "url")] = catalogData.URL
					config[fmt.Sprintf("%s-%s-%s", "catalog", catalogData.Index, "type")] = catalogData.Type
					return true
				})
			}
		default:
			// Skip unsupported field types
			continue
		}
	}

	return config
}
