package config

import (
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestExtractKeyword(t *testing.T) {
	tests := []struct {
		name            string
		keywordLocation string
		expected        string
	}{
		{
			name:            "simple keyword",
			keywordLocation: "#/required",
			expected:        "required",
		},
		{
			name:            "nested keyword",
			keywordLocation: "#/properties/name/type",
			expected:        "type",
		},
		{
			name:            "deep nested keyword",
			keywordLocation: "#/items/properties/shell/minLength",
			expected:        "minLength",
		},
		{
			name:            "empty location",
			keywordLocation: "",
			expected:        "",
		},
		{
			name:            "just hash",
			keywordLocation: "#",
			expected:        "#",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractKeyword(tt.keywordLocation)
			if result != tt.expected {
				t.Errorf("extractKeyword(%q) = %q, want %q", tt.keywordLocation, result, tt.expected)
			}
		})
	}
}

func TestExtractMissingProperty(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "single property",
			message:  "missing properties: 'src'",
			expected: "src",
		},
		{
			name:     "dest property",
			message:  "missing properties: 'dest'",
			expected: "dest",
		},
		{
			name:     "shell property",
			message:  "missing properties: 'shell'",
			expected: "shell",
		},
		{
			name:     "path property",
			message:  "missing properties: 'path'",
			expected: "path",
		},
		{
			name:     "no quotes",
			message:  "missing properties: src",
			expected: "",
		},
		{
			name:     "empty message",
			message:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMissingProperty(tt.message)
			if result != tt.expected {
				t.Errorf("extractMissingProperty(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestExtractAdditionalProperty(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "command typo",
			message:  "additionalProperties 'command' not allowed",
			expected: "command",
		},
		{
			name:     "cmd typo",
			message:  "additionalProperties 'cmd' not allowed",
			expected: "cmd",
		},
		{
			name:     "source typo",
			message:  "additionalProperties 'source' not allowed",
			expected: "source",
		},
		{
			name:     "no quotes",
			message:  "additionalProperties command not allowed",
			expected: "",
		},
		{
			name:     "empty message",
			message:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAdditionalProperty(tt.message)
			if result != tt.expected {
				t.Errorf("extractAdditionalProperty(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestFormatTypeError(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "string expected got number",
			message:  "expected string, got number",
			expected: "Expected text value, but got a number. Wrap it in quotes (e.g., \"123\" instead of 123)",
		},
		{
			name:     "string expected got boolean",
			message:  "expected string, got boolean",
			expected: "Expected text value, but got true/false. Wrap it in quotes if intended as text",
		},
		{
			name:     "string expected got object",
			message:  "expected string, got object",
			expected: "Expected text value, but got an object/map. Check field structure",
		},
		{
			name:     "string expected got array",
			message:  "expected string, got array",
			expected: "Expected text value, but got a list. Check field structure",
		},
		{
			name:     "boolean expected",
			message:  "expected boolean, got string",
			expected: "Expected true or false (boolean), not text or number",
		},
		{
			name:     "object expected",
			message:  "expected object, got string",
			expected: "Expected an object with fields (like 'path:', 'state:'), but got a simple value",
		},
		{
			name:     "array expected",
			message:  "expected array, got string",
			expected: "Expected a list of items, but got a single value",
		},
		{
			name:     "number expected",
			message:  "expected number, got string",
			expected: "Expected a number, but got text or another type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTypeError(tt.message)
			if result != tt.expected {
				t.Errorf("formatTypeError(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestFormatEnumError(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		instancePath string
		expected     string
	}{
		{
			name:         "file state error",
			message:      "value must be one of 'file', 'directory', 'absent'",
			instancePath: "/steps/0/file/state",
			expected:     "Invalid file state. Must be one of: 'file', 'directory', 'absent', 'touch', 'link', 'hardlink', or 'perms'",
		},
		{
			name:         "generic enum error",
			message:      "value must be one of 'option1', 'option2'",
			instancePath: "/some/path",
			expected:     "Invalid value. Must be one of 'option1', 'option2'",
		},
		{
			name:         "other message format",
			message:      "invalid enum value",
			instancePath: "/some/path",
			expected:     "invalid enum value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatEnumError(tt.message, tt.instancePath)
			if result != tt.expected {
				t.Errorf("formatEnumError(%q, %q) = %q, want %q", tt.message, tt.instancePath, result, tt.expected)
			}
		})
	}
}

func TestFormatPatternError(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		instancePath string
		expected     string
	}{
		{
			name:         "file mode error",
			message:      "does not match pattern '^0[0-7]{3}$'",
			instancePath: "/steps/0/file/mode",
			expected:     "Invalid file mode. Must be in octal format: '0' followed by 3 octal digits (e.g., '0644', '0755')",
		},
		{
			name:         "shell command error",
			message:      "does not match pattern",
			instancePath: "/steps/0/shell",
			expected:     "Invalid shell command format",
		},
		{
			name:         "path error",
			message:      "does not match pattern",
			instancePath: "/steps/0/file/path",
			expected:     "Invalid path format. Check for proper file path syntax",
		},
		{
			name:         "generic pattern error",
			message:      "does not match pattern '^[a-z]+$'",
			instancePath: "/some/field",
			expected:     "Invalid format. Check the value syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPatternError(tt.message, tt.instancePath)
			if result != tt.expected {
				t.Errorf("formatPatternError(%q, %q) = %q, want %q", tt.message, tt.instancePath, result, tt.expected)
			}
		})
	}
}

func TestFormatRequiredError(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "missing src",
			message:  "missing properties: 'src'",
			expected: "Missing required field 'src'. Template needs a source file path (e.g., src: ./template.j2)",
		},
		{
			name:     "missing dest",
			message:  "missing properties: 'dest'",
			expected: "Missing required field 'dest'. Template needs a destination path (e.g., dest: /etc/config.conf)",
		},
		{
			name:     "missing path",
			message:  "missing properties: 'path'",
			expected: "Missing required field 'path'. File action needs a file or directory path (e.g., path: /tmp/myfile)",
		},
		{
			name:     "missing shell",
			message:  "missing properties: 'shell'",
			expected: "Missing required field 'shell'. Provide a shell command to execute",
		},
		{
			name:     "missing template",
			message:  "missing properties: 'template'",
			expected: "Missing required field 'template'. Provide template configuration with src and dest",
		},
		{
			name:     "missing file",
			message:  "missing properties: 'file'",
			expected: "Missing required field 'file'. Provide file configuration with path",
		},
		{
			name:     "missing unknown field",
			message:  "missing properties: 'unknown'",
			expected: "Missing required field 'unknown'. This field must be specified",
		},
		{
			name:     "no property name",
			message:  "missing properties",
			expected: "Missing required field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRequiredError(tt.message, "")
			if result != tt.expected {
				t.Errorf("formatRequiredError(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestFormatAdditionalPropertiesError(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "command typo",
			message:  "additionalProperties 'command' not allowed",
			expected: "Unknown field 'command'. Did you mean 'shell'?",
		},
		{
			name:     "cmd typo",
			message:  "additionalProperties 'cmd' not allowed",
			expected: "Unknown field 'cmd'. Did you mean 'shell'?",
		},
		{
			name:     "source typo",
			message:  "additionalProperties 'source' not allowed",
			expected: "Unknown field 'source'. Did you mean 'src'?",
		},
		{
			name:     "destination typo",
			message:  "additionalProperties 'destination' not allowed",
			expected: "Unknown field 'destination'. Did you mean 'dest'?",
		},
		{
			name:     "condition typo",
			message:  "additionalProperties 'condition' not allowed",
			expected: "Unknown field 'condition'. Did you mean 'when'?",
		},
		{
			name:     "sudo typo",
			message:  "additionalProperties 'sudo' not allowed",
			expected: "Unknown field 'sudo'. Did you mean 'become'?",
		},
		{
			name:     "unknown field",
			message:  "additionalProperties 'randomfield' not allowed",
			expected: "Unknown field 'randomfield'. Check spelling or remove this field",
		},
		{
			name:     "no property name",
			message:  "additionalProperties not allowed",
			expected: "Unknown field found. Check for typos or unsupported fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAdditionalPropertiesError(tt.message, "")
			if result != tt.expected {
				t.Errorf("formatAdditionalPropertiesError(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestFormatMinLengthError(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "with minimum",
			message:  "length must be >= 5 characters",
			expected: "Value is too short. Increase the length",
		},
		{
			name:     "without minimum",
			message:  "string too short",
			expected: "Value is too short. Increase the length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMinLengthError(tt.message)
			if result != tt.expected {
				t.Errorf("formatMinLengthError(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestFormatMaxLengthError(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "with maximum",
			message:  "length must be <= 100 characters",
			expected: "Value is too long. Reduce the length",
		},
		{
			name:     "without maximum",
			message:  "string too long",
			expected: "Value is too long. Reduce the length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMaxLengthError(tt.message)
			if result != tt.expected {
				t.Errorf("formatMaxLengthError(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestFormatMinimumError(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "with must be",
			message:  "must be >= 5",
			expected: "Value is too small. Must be at least >= 5",
		},
		{
			name:     "without must be",
			message:  "value too small",
			expected: "Value is too small. Increase the number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMinimumError(tt.message)
			if result != tt.expected {
				t.Errorf("formatMinimumError(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestFormatMaximumError(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "with must be",
			message:  "must be <= 100",
			expected: "Value is too large. Must be at most <= 100",
		},
		{
			name:     "without must be",
			message:  "value too large",
			expected: "Value is too large. Reduce the number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMaximumError(tt.message)
			if result != tt.expected {
				t.Errorf("formatMaximumError(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestFormatFormatError(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "email format",
			message:  "invalid email format",
			expected: "Invalid email format. Must be like: user@example.com",
		},
		{
			name:     "uri format",
			message:  "invalid uri format",
			expected: "Invalid URL format. Must be like: https://example.com/path",
		},
		{
			name:     "url format",
			message:  "invalid url format",
			expected: "Invalid URL format. Must be like: https://example.com/path",
		},
		{
			name:     "date format",
			message:  "invalid date format",
			expected: "Invalid date format. Check date syntax",
		},
		{
			name:     "time format",
			message:  "invalid time format",
			expected: "Invalid time format. Check time syntax",
		},
		{
			name:     "ipv4 format",
			message:  "invalid ipv4 address",
			expected: "Invalid IPv4 address. Must be like: 192.168.1.1",
		},
		{
			name:     "ipv6 format",
			message:  "invalid ipv6 address",
			expected: "Invalid IPv6 address format",
		},
		{
			name:     "hostname format",
			message:  "invalid hostname",
			expected: "Invalid hostname. Must be like: example.com",
		},
		{
			name:     "generic format",
			message:  "invalid format",
			expected: "Invalid format. Check the value syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFormatError(tt.message, "")
			if result != tt.expected {
				t.Errorf("formatFormatError(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestFormatOneOfError(t *testing.T) {
	tests := []struct {
		name     string
		err      *jsonschema.ValidationError
		expected string
	}{
		{
			name: "no action present",
			err: &jsonschema.ValidationError{
				Causes: []*jsonschema.ValidationError{
					{Message: "missing required property 'shell'"},
					{Message: "missing required property 'file'"},
				},
			},
			expected: "Step has no action. Each step must have exactly ONE of: shell, template, file, copy, service, include, include_vars, or vars",
		},
		{
			name: "multiple actions present",
			err: &jsonschema.ValidationError{
				Causes: []*jsonschema.ValidationError{
					{KeywordLocation: "#/oneOf/0/not"},
					{KeywordLocation: "#/oneOf/1/not"},
				},
			},
			expected: "Step has multiple actions. Only ONE action is allowed per step. Choose either: shell, template, file, copy, service, include, include_vars, or vars",
		},
		{
			name: "generic oneOf error",
			err: &jsonschema.ValidationError{
				Causes: []*jsonschema.ValidationError{},
			},
			expected: "Step must have exactly one action (shell, template, file, copy, service, include, include_vars, or vars)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatOneOfError(tt.err)
			if result != tt.expected {
				t.Errorf("formatOneOfError() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatValidationErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      *jsonschema.ValidationError
		expected string
	}{
		{
			name: "required error",
			err: &jsonschema.ValidationError{
				KeywordLocation:  "#/required",
				Message:          "missing properties: 'shell'",
				InstanceLocation: "/steps/0",
			},
			expected: "Missing required field 'shell'. Provide a shell command to execute",
		},
		{
			name: "type error",
			err: &jsonschema.ValidationError{
				KeywordLocation:  "#/type",
				Message:          "expected string, got number",
				InstanceLocation: "/steps/0/name",
			},
			expected: "Expected text value, but got a number. Wrap it in quotes (e.g., \"123\" instead of 123)",
		},
		{
			name: "enum error",
			err: &jsonschema.ValidationError{
				KeywordLocation:  "#/enum",
				Message:          "value must be one of 'file', 'directory', 'absent'",
				InstanceLocation: "/steps/0/file/state",
			},
			expected: "Invalid file state. Must be one of: 'file', 'directory', 'absent', 'touch', 'link', 'hardlink', or 'perms'",
		},
		{
			name: "minItems error",
			err: &jsonschema.ValidationError{
				KeywordLocation:  "#/minItems",
				Message:          "array must have at least 1 item",
				InstanceLocation: "/steps",
			},
			expected: "List must have at least the minimum number of items",
		},
		{
			name: "maxItems error",
			err: &jsonschema.ValidationError{
				KeywordLocation:  "#/maxItems",
				Message:          "array must have at most 10 items",
				InstanceLocation: "/steps",
			},
			expected: "List has too many items. Reduce the number of items",
		},
		{
			name: "uniqueItems error",
			err: &jsonschema.ValidationError{
				KeywordLocation:  "#/uniqueItems",
				Message:          "array items must be unique",
				InstanceLocation: "/tags",
			},
			expected: "List contains duplicate items. Each item must be unique",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValidationErrorMessage(tt.err)
			if result != tt.expected {
				t.Errorf("formatValidationErrorMessage() = %q, want %q", result, tt.expected)
			}
		})
	}
}
