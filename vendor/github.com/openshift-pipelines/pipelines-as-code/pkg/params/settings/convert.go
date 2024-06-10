package settings

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

func ConvertPacStructToConfigMap(settings *Settings) map[string]string {
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
				data.Range(func(key, value interface{}) bool {
					catalogData := value.(HubCatalog)
					if key == "default" {
						config[HubURLKey] = catalogData.URL
						config[HubCatalogNameKey] = catalogData.Name
						return true
					}
					config[fmt.Sprintf("%s-%s-%s", "catalog", catalogData.Index, "id")] = key.(string)
					config[fmt.Sprintf("%s-%s-%s", "catalog", catalogData.Index, "name")] = catalogData.Name
					config[fmt.Sprintf("%s-%s-%s", "catalog", catalogData.Index, "url")] = catalogData.URL
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
