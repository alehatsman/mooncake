package config

import (
	"bufio"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Reader defines the interface for reading configuration and variables
type Reader interface {
	ReadConfig(path string) ([]Step, error)
	ReadVariables(path string) (map[string]interface{}, error)
}

// YAMLConfigReader implements Reader for YAML files
type YAMLConfigReader struct {
	// Can add dependencies here if needed (e.g., FileSystem interface)
}

// NewYAMLConfigReader creates a new YAMLConfigReader
func NewYAMLConfigReader() Reader {
	return &YAMLConfigReader{}
}

// ReadConfig reads configuration steps from a YAML file
// For backward compatibility, this method validates the config and returns
// an error if any validation errors are found
func (r *YAMLConfigReader) ReadConfig(path string) ([]Step, error) {
	steps, diagnostics, err := r.ReadConfigWithValidation(path)
	if err != nil {
		return nil, err
	}

	// Convert diagnostics to error for backward compatibility
	if len(diagnostics) > 0 && HasErrors(diagnostics) {
		return steps, &ValidationError{Diagnostics: diagnostics}
	}

	return steps, nil
}

// ReadConfigWithValidation reads configuration steps from a YAML file with full validation
// Returns steps, diagnostics (which may include warnings), and any parsing errors
func (r *YAMLConfigReader) ReadConfigWithValidation(path string) ([]Step, []Diagnostic, error) {
	// #nosec G304 -- User-specified config file path is intentional and required functionality
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err = f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close config file %s: %v\n", path, err)
		}
	}()

	// Parse YAML to yaml.Node to preserve source location information
	var rootNode yaml.Node
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&rootNode)
	if err != nil {
		return nil, nil, err
	}

	// Build location map from yaml.Node tree
	locationMap := buildLocationMap(&rootNode)

	// Unmarshal yaml.Node to []Step structs
	var config []Step
	err = rootNode.Decode(&config)
	if err != nil {
		return nil, nil, err
	}

	// Run schema validation
	validator, err := NewSchemaValidator()
	if err != nil {
		// If schema validation setup fails, fall back to basic validation
		// This ensures the system still works even if schema is broken
		return config, []Diagnostic{
			{
				FilePath: path,
				Line:     1,
				Column:   1,
				Message:  "schema validator initialization failed: " + err.Error(),
				Severity: "warning",
			},
		}, nil
	}

	diagnostics := validator.Validate(config, locationMap, path)

	return config, diagnostics, nil
}

// ReadVariables reads variables from a YAML file
func (r *YAMLConfigReader) ReadVariables(path string) (map[string]interface{}, error) {
	if path == "" {
		return make(map[string]interface{}), nil
	}

	// #nosec G304 -- User-specified variables file path is intentional and required functionality
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err = file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close variables file %s: %v\n", path, err)
		}
	}()

	reader := bufio.NewReader(file)

	variables := make(map[string]interface{})

	decoder := yaml.NewDecoder(reader)
	err = decoder.Decode(&variables)
	if err != nil {
		return nil, err
	}

	return variables, nil
}

// Package-level functions for backward compatibility
var defaultReader = NewYAMLConfigReader()

// ReadConfig is a convenience function using the default YAML reader
func ReadConfig(path string) ([]Step, error) {
	return defaultReader.ReadConfig(path)
}

// ReadConfigWithValidation is a convenience function using the default YAML reader
// Returns steps, diagnostics, and any parsing errors
func ReadConfigWithValidation(path string) ([]Step, []Diagnostic, error) {
	reader, ok := defaultReader.(*YAMLConfigReader)
	if !ok {
		panic("defaultReader is not a YAMLConfigReader")
	}
	return reader.ReadConfigWithValidation(path)
}

// ReadVariables is a convenience function using the default YAML reader
func ReadVariables(path string) (map[string]interface{}, error) {
	return defaultReader.ReadVariables(path)
}
