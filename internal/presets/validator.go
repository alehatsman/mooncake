package presets

import (
	"fmt"
	"reflect"

	"github.com/alehatsman/mooncake/internal/config"
)

// ValidateParameters validates user-provided parameters against preset parameter definitions.
// It checks required parameters, validates types, checks enum constraints, and applies defaults.
// Returns a validated parameter map ready for use in template expansion.
func ValidateParameters(definition *config.PresetDefinition, userParams map[string]interface{}) (map[string]interface{}, error) {
	if definition == nil {
		return nil, fmt.Errorf("preset definition is nil")
	}

	validated := make(map[string]interface{})

	// Check all defined parameters
	for paramName, paramDef := range definition.Parameters {
		userValue, provided := userParams[paramName]

		// Check required parameters
		if paramDef.Required && !provided {
			return nil, fmt.Errorf("required parameter '%s' not provided", paramName)
		}

		// Use default if not provided
		if !provided {
			if paramDef.Default != nil {
				validated[paramName] = paramDef.Default
			}
			continue
		}

		// Validate type
		if err := validateType(paramName, userValue, paramDef.Type); err != nil {
			return nil, err
		}

		// Validate enum constraint
		if len(paramDef.Enum) > 0 {
			if err := validateEnum(paramName, userValue, paramDef.Enum); err != nil {
				return nil, err
			}
		}

		validated[paramName] = userValue
	}

	// Check for unknown parameters
	for userParam := range userParams {
		if _, defined := definition.Parameters[userParam]; !defined {
			return nil, fmt.Errorf("unknown parameter '%s' (preset '%s' does not define this parameter)", userParam, definition.Name)
		}
	}

	return validated, nil
}

// validateType checks if a value matches the expected type.
func validateType(paramName string, value interface{}, expectedType string) error {
	actualType := getValueType(value)

	switch expectedType {
	case "string":
		if actualType != "string" {
			return fmt.Errorf("parameter '%s' must be a string, got %s", paramName, actualType)
		}
	case "bool":
		if actualType != "bool" {
			return fmt.Errorf("parameter '%s' must be a boolean, got %s", paramName, actualType)
		}
	case "array":
		if actualType != "array" {
			return fmt.Errorf("parameter '%s' must be an array, got %s", paramName, actualType)
		}
	case "object":
		if actualType != "object" {
			return fmt.Errorf("parameter '%s' must be an object, got %s", paramName, actualType)
		}
	default:
		return fmt.Errorf("parameter '%s' has unknown type definition: %s", paramName, expectedType)
	}

	return nil
}

// validateEnum checks if a value is in the allowed enum values.
func validateEnum(paramName string, value interface{}, enumValues []interface{}) error {
	for _, allowed := range enumValues {
		if reflect.DeepEqual(value, allowed) {
			return nil
		}
	}

	return fmt.Errorf("parameter '%s' has invalid value: got %v, allowed values: %v", paramName, value, enumValues)
}

// getValueType returns a string representation of the value's type.
func getValueType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return "number"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}
