package config

import (
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// error_messages.go - User-friendly error message formatting
//
// This file contains all the formatXxxError() functions that convert technical
// JSON Schema validation errors into user-friendly, actionable messages.
//
// The formatting is context-aware based on:
// - Error keyword (pattern, enum, type, required, etc.)
// - Instance path (which field failed)
// - Error message content
//
// Example transformations:
//   "does not match pattern '^0[0-7]{3}$'" → "Invalid file mode. Must be in octal format: '0644', '0755'"
//   "value must be one of 'file', 'directory', 'absent'" → "Invalid file state. Must be one of: 'file', 'directory', or 'absent'"

// formatValidationErrorMessage formats a validation error message based on the error type
func formatValidationErrorMessage(err *jsonschema.ValidationError) string {
	// Extract the keyword that failed
	keyword := extractKeyword(err.KeywordLocation)

	switch keyword {
	case "required":
		return formatRequiredError(err.Message, err.InstanceLocation)

	case "additionalProperties":
		return formatAdditionalPropertiesError(err.Message, err.InstanceLocation)

	case "type":
		return formatTypeError(err.Message)

	case "enum":
		return formatEnumError(err.Message, err.InstanceLocation)

	case "pattern":
		return formatPatternError(err.Message, err.InstanceLocation)

	case "oneOf":
		// Special handling - will be done in formatOneOfError
		return ""

	case "minLength":
		return formatMinLengthError(err.Message)

	case "maxLength":
		return formatMaxLengthError(err.Message)

	case "minimum":
		return formatMinimumError(err.Message)

	case "maximum":
		return formatMaximumError(err.Message)

	case "format":
		return formatFormatError(err.Message, err.InstanceLocation)

	case "minItems":
		return "List must have at least the minimum number of items"

	case "maxItems":
		return "List has too many items. Reduce the number of items"

	case "uniqueItems":
		return "List contains duplicate items. Each item must be unique"

	default:
		// Capitalize first letter for consistency
		if len(err.Message) > 0 {
			return strings.ToUpper(err.Message[:1]) + err.Message[1:]
		}
		return err.Message
	}
}

// formatTypeError creates a friendly message for type validation errors
func formatTypeError(message string) string {
	// Parse the type error to be more specific
	if strings.Contains(message, "expected string") {
		if strings.Contains(message, "got number") {
			return "Expected text value, but got a number. Wrap it in quotes (e.g., \"123\" instead of 123)"
		}
		if strings.Contains(message, "got boolean") {
			return "Expected text value, but got true/false. Wrap it in quotes if intended as text"
		}
		if strings.Contains(message, "got object") {
			return "Expected text value, but got an object/map. Check field structure"
		}
		if strings.Contains(message, "got array") {
			return "Expected text value, but got a list. Check field structure"
		}
		return "Expected text value (string), but got a different type"
	}

	if strings.Contains(message, "expected boolean") {
		return "Expected true or false (boolean), not text or number"
	}

	if strings.Contains(message, "expected object") {
		return "Expected an object with fields (like 'path:', 'state:'), but got a simple value"
	}

	if strings.Contains(message, "expected array") {
		return "Expected a list of items, but got a single value"
	}

	if strings.Contains(message, "expected number") {
		return "Expected a number, but got text or another type"
	}

	// Capitalize first letter for consistency
	if strings.Contains(message, "expected") {
		return strings.Replace(message, "expected", "Expected", 1)
	}

	return message
}

// formatEnumError creates a friendly message for enum validation errors
func formatEnumError(message string, instancePath string) string {
	// Check if this is a file state error
	if strings.Contains(instancePath, "/state") {
		return "Invalid file state. Must be one of: 'file', 'directory', 'absent', 'touch', 'link', 'hardlink', or 'perms'"
	}

	// Generic enum error - make it more readable
	if strings.Contains(message, "must be one of") {
		// Extract the allowed values
		return strings.Replace(message, "value must be one of", "Invalid value. Must be one of", 1)
	}

	return message
}

// formatPatternError creates a friendly message for pattern validation errors
func formatPatternError(message string, instancePath string) string {
	// File mode pattern
	if strings.Contains(instancePath, "/mode") {
		return "Invalid file mode. Must be in octal format: '0' followed by 3 octal digits (e.g., '0644', '0755')"
	}

	// Shell command pattern (if we add validation for it in the future)
	if strings.Contains(instancePath, "/shell") {
		return "Invalid shell command format"
	}

	// Path pattern (if we add validation for it)
	if strings.Contains(instancePath, "/path") || strings.Contains(instancePath, "/src") || strings.Contains(instancePath, "/dest") {
		return "Invalid path format. Check for proper file path syntax"
	}

	// Generic pattern error - try to make it more readable
	if strings.Contains(message, "does not match pattern") {
		// Remove the regex pattern from message as it's not user-friendly
		parts := strings.Split(message, "pattern")
		if len(parts) > 0 {
			return "Invalid format. Check the value syntax"
		}
		return "Invalid format. Value doesn't match expected pattern"
	}

	return message
}

// formatRequiredError creates a friendly message for missing required fields
func formatRequiredError(message string, _ string) string {
	// Extract the missing property name
	missingField := extractMissingProperty(message)
	if missingField == "" {
		return "Missing required field"
	}

	// Context-specific messages based on the field
	switch missingField {
	case "src":
		return "Missing required field 'src'. Template needs a source file path (e.g., src: ./template.j2)"
	case "dest":
		return "Missing required field 'dest'. Template needs a destination path (e.g., dest: /etc/config.conf)"
	case "path":
		return "Missing required field 'path'. File action needs a file or directory path (e.g., path: /tmp/myfile)"
	case "shell":
		return "Missing required field 'shell'. Provide a shell command to execute"
	case "template":
		return "Missing required field 'template'. Provide template configuration with src and dest"
	case "file":
		return "Missing required field 'file'. Provide file configuration with path"
	default:
		return fmt.Sprintf("Missing required field '%s'. This field must be specified", missingField)
	}
}

// formatAdditionalPropertiesError creates a friendly message for unknown fields
func formatAdditionalPropertiesError(message string, _ string) string {
	// Extract the additional property name
	additionalField := extractAdditionalProperty(message)
	if additionalField == "" {
		return "Unknown field found. Check for typos or unsupported fields"
	}

	// Common typos and suggestions
	suggestions := map[string]string{
		"command":     "Did you mean 'shell'?",
		"cmd":         "Did you mean 'shell'?",
		"run":         "Did you mean 'shell'?",
		"execute":     "Did you mean 'shell'?",
		"source":      "Did you mean 'src'?",
		"destination": "Did you mean 'dest'?",
		"target":      "Did you mean 'dest'?",
		"output":      "Did you mean 'dest'?",
		"directory":   "Did you mean 'state: directory' under 'file'?",
		"folder":      "Did you mean 'state: directory' under 'file'?",
		"condition":   "Did you mean 'when'?",
		"if":          "Did you mean 'when'?",
		"sudo":        "Did you mean 'become'?",
		"root":        "Did you mean 'become: true'?",
		"tag":         "Did you mean 'tags'?",
		"loop":        "Did you mean 'with_items' or 'with_filetree'?",
		"foreach":     "Did you mean 'with_items'?",
		"var":         "Did you mean 'vars'?",
		"variables":   "Did you mean 'vars'?",
		"variable":    "Did you mean 'vars'?",
	}

	if suggestion, ok := suggestions[strings.ToLower(additionalField)]; ok {
		return fmt.Sprintf("Unknown field '%s'. %s", additionalField, suggestion)
	}

	// Generic message
	return fmt.Sprintf("Unknown field '%s'. Check spelling or remove this field", additionalField)
}

// formatOneOfError formats a oneOf validation error (mutually exclusive actions)
func formatOneOfError(err *jsonschema.ValidationError) string {
	// For oneOf errors related to steps, provide a clear actionable message

	// Check if this is a "no action" vs "multiple actions" case
	// by looking at the causes
	hasRequiredFailure := false
	hasNotFailure := false

	for _, cause := range err.Causes {
		if strings.Contains(cause.Message, "required property") {
			hasRequiredFailure = true
		}
		if strings.Contains(cause.KeywordLocation, "/not") {
			hasNotFailure = true
		}
	}

	// If all causes are "required" failures, it means no action is present
	if hasRequiredFailure && !hasNotFailure {
		return "Step has no action. Each step must have exactly ONE of: shell, template, file, file_replace, copy, service, assert, preset, print, include, include_vars, vars, repo_search, or repo_tree"
	}

	// If we have "not" failures, it means multiple actions are present
	if hasNotFailure {
		return "Step has multiple actions. Only ONE action is allowed per step. Choose either: shell, template, file, file_replace, copy, service, assert, preset, print, include, include_vars, vars, repo_search, or repo_tree"
	}

	// Generic fallback
	return "Step must have exactly one action (shell, template, file, file_replace, copy, service, assert, preset, print, include, include_vars, vars, repo_search, or repo_tree)"
}

// formatMinLengthError creates a friendly message for string too short errors
func formatMinLengthError(message string) string {
	// Extract minimum length if possible
	if strings.Contains(message, "minimum") {
		return strings.Replace(message, "length must be", "Value is too short. Must be at least", 1)
	}
	return "Value is too short. Increase the length"
}

// formatMaxLengthError creates a friendly message for string too long errors
func formatMaxLengthError(message string) string {
	if strings.Contains(message, "maximum") {
		return strings.Replace(message, "length must be", "Value is too long. Must be at most", 1)
	}
	return "Value is too long. Reduce the length"
}

// formatMinimumError creates a friendly message for number too small errors
func formatMinimumError(message string) string {
	if strings.Contains(message, "must be") {
		return strings.Replace(message, "must be", "Value is too small. Must be at least", 1)
	}
	return "Value is too small. Increase the number"
}

// formatMaximumError creates a friendly message for number too large errors
func formatMaximumError(message string) string {
	if strings.Contains(message, "must be") {
		return strings.Replace(message, "must be", "Value is too large. Must be at most", 1)
	}
	return "Value is too large. Reduce the number"
}

// formatFormatError creates a friendly message for format validation errors
func formatFormatError(message string, _ string) string {
	// Detect the format type
	if strings.Contains(message, "email") {
		return "Invalid email format. Must be like: user@example.com"
	}
	if strings.Contains(message, "uri") || strings.Contains(message, "url") {
		return "Invalid URL format. Must be like: https://example.com/path"
	}
	if strings.Contains(message, "date") {
		return "Invalid date format. Check date syntax"
	}
	if strings.Contains(message, "time") {
		return "Invalid time format. Check time syntax"
	}
	if strings.Contains(message, "ipv4") {
		return "Invalid IPv4 address. Must be like: 192.168.1.1"
	}
	if strings.Contains(message, "ipv6") {
		return "Invalid IPv6 address format"
	}
	if strings.Contains(message, "hostname") {
		return "Invalid hostname. Must be like: example.com"
	}

	// Generic format error
	return "Invalid format. Check the value syntax"
}

// extractKeyword extracts the keyword from a keyword location JSON pointer
func extractKeyword(keywordLocation string) string {
	parts := strings.Split(keywordLocation, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// extractMissingProperty extracts the property name from a "required" error message
func extractMissingProperty(message string) string {
	// Example: "missing properties: 'src'"
	if idx := strings.Index(message, "'"); idx != -1 {
		if endIdx := strings.Index(message[idx+1:], "'"); endIdx != -1 {
			return message[idx+1 : idx+1+endIdx]
		}
	}
	return ""
}

// extractAdditionalProperty extracts the property name from an "additionalProperties" error message
func extractAdditionalProperty(message string) string {
	// Example: "additionalProperties 'foo' not allowed"
	if idx := strings.Index(message, "'"); idx != -1 {
		if endIdx := strings.Index(message[idx+1:], "'"); endIdx != -1 {
			return message[idx+1 : idx+1+endIdx]
		}
	}
	return ""
}
