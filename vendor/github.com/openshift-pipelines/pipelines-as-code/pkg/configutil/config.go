package configutil

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

func ValidateAndAssignValues(logger *zap.SugaredLogger, configData map[string]string, configStruct any, customValidations map[string]func(string) error, logUpdates bool) error {
	structValue := reflect.ValueOf(configStruct).Elem()
	structType := reflect.TypeOf(configStruct).Elem()

	var errors []error

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldName := field.Name

		jsonTag := field.Tag.Get("json")
		// Skip field which doesn't have json tag
		if jsonTag == "" {
			continue
		}

		// Read value from ConfigMap
		fieldValue := configData[strings.ToLower(jsonTag)]

		// If value is missing in ConfigMap, use default value from struct tag
		if fieldValue == "" {
			fieldValue = field.Tag.Get("default")
		}

		fieldValueKind := field.Type.Kind()

		//nolint
		switch fieldValueKind {
		case reflect.String:
			// if fieldvalue is empty, skip validation and set the field as empty string
			if validator, ok := customValidations[fieldName]; ok && fieldValue != "" {
				if err := validator(fieldValue); err != nil {
					errors = append(errors, fmt.Errorf("custom validation failed for field %s: %w", fieldName, err))
					continue
				}
			}
			oldValue := structValue.FieldByName(fieldName).String()
			if oldValue != fieldValue && logUpdates {
				logger.Infof("updating value for field %s: from '%s' to '%s'", fieldName, oldValue, fieldValue)
			}
			structValue.FieldByName(fieldName).SetString(fieldValue)

		case reflect.Bool:
			// if fieldvalue is empty, set the field as false
			if fieldValue == "" {
				fieldValue = "false"
			}
			newValue, err := strconv.ParseBool(fieldValue)
			if err != nil {
				errors = append(errors, fmt.Errorf("invalid value for bool field %s: %w", fieldName, err))
				continue
			}
			oldValue := structValue.FieldByName(fieldName).Bool()
			if oldValue != newValue && logUpdates {
				logger.Infof("updating value for field %s: from '%v' to '%v'", fieldName, oldValue, newValue)
			}
			structValue.FieldByName(fieldName).SetBool(newValue)

		case reflect.Int:
			// if fieldvalue is empty, skip validation and set the field as 0
			if validator, ok := customValidations[fieldName]; ok && fieldValue != "" {
				if err := validator(fieldValue); err != nil {
					errors = append(errors, fmt.Errorf("custom validation failed for field %s: %w", fieldName, err))
					continue
				}
			}
			if fieldValue == "" {
				fieldValue = "0"
			}
			newValue, err := strconv.ParseInt(fieldValue, 10, 64)
			if err != nil {
				errors = append(errors, fmt.Errorf("invalid value for int field %s: %w", fieldName, err))
				continue
			}
			oldValue := structValue.FieldByName(fieldName).Int()
			if oldValue != newValue && logUpdates {
				logger.Infof("updating value for field %s: from '%d' to '%d'", fieldName, oldValue, newValue)
			}
			structValue.FieldByName(fieldName).SetInt(newValue)

		default:
			// Skip unsupported field types
			continue
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %v", errors)
	}

	return nil
}
