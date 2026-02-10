package config

import (
	"bufio"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Reader defines the interface for reading configuration and variables
type Reader interface {
	ReadConfig(path string) (*ParsedConfig, error)
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
func (r *YAMLConfigReader) ReadConfig(path string) (*ParsedConfig, error) {
	parsedConfig, diagnostics, err := r.ReadConfigWithValidation(path)
	if err != nil {
		return nil, err
	}

	// Convert diagnostics to error for backward compatibility
	if len(diagnostics) > 0 && HasErrors(diagnostics) {
		return parsedConfig, &ValidationError{Diagnostics: diagnostics}
	}

	return parsedConfig, nil
}

// ReadConfigWithValidation reads configuration steps from a YAML file with full validation
// Returns parsed config (with steps, global vars, version), diagnostics (which may include warnings), and any parsing errors
func (r *YAMLConfigReader) ReadConfigWithValidation(path string) (*ParsedConfig, []Diagnostic, error) {
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

	// Parse config - supports both old format (array) and new format (object with steps)
	var parsedConfig *ParsedConfig
	if isArrayFormat(&rootNode) {
		// Old format: plain array of steps
		var steps []Step
		err = rootNode.Decode(&steps)
		if err != nil {
			return nil, nil, err
		}
		parsedConfig = &ParsedConfig{
			Steps:      steps,
			GlobalVars: make(map[string]interface{}),
			Version:    "",
		}
	} else {
		// New format: RunConfig structure with version, vars, and steps
		var runConfig RunConfig
		err = rootNode.Decode(&runConfig)
		if err != nil {
			return nil, nil, err
		}
		// Initialize GlobalVars to empty map if nil
		globalVars := runConfig.Vars
		if globalVars == nil {
			globalVars = make(map[string]interface{})
		}
		parsedConfig = &ParsedConfig{
			Steps:      runConfig.Steps,
			GlobalVars: globalVars,
			Version:    runConfig.Version,
		}
	}

	// Run schema validation
	validator, err := NewSchemaValidator()
	if err != nil {
		// If schema validation setup fails, fall back to basic validation
		// This ensures the system still works even if schema is broken
		return parsedConfig, []Diagnostic{
			{
				FilePath: path,
				Line:     1,
				Column:   1,
				Message:  "schema validator initialization failed: " + err.Error(),
				Severity: "warning",
			},
		}, nil
	}

	diagnostics := validator.Validate(parsedConfig.Steps, locationMap, path)

	// Validate template syntax in all templatable fields
	templateValidator := NewTemplateValidator()
	templateDiagnostics := templateValidator.ValidateSteps(parsedConfig.Steps, locationMap, path)
	diagnostics = append(diagnostics, templateDiagnostics...)

	return parsedConfig, diagnostics, nil
}

// isArrayFormat checks if the YAML root node represents an array (old format)
// or an object (new RunConfig format)
func isArrayFormat(node *yaml.Node) bool {
	// The root node is a document node, so we need to check its content
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		firstNode := node.Content[0]
		return firstNode.Kind == yaml.SequenceNode
	}
	return node.Kind == yaml.SequenceNode
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
func ReadConfig(path string) (*ParsedConfig, error) {
	return defaultReader.ReadConfig(path)
}

// ReadConfigWithValidation is a convenience function using the default YAML reader
// Returns parsed config (with steps, global vars, version), diagnostics, and any parsing errors
func ReadConfigWithValidation(path string) (*ParsedConfig, []Diagnostic, error) {
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
