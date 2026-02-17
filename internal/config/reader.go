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
		closeErr := f.Close()
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "failed to close config file %s: %v\n", path, closeErr)
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

		// Attach source locations from locationMap
		attachSourceLocations(steps, locationMap, "")

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

		// Attach source locations from locationMap
		attachSourceLocations(runConfig.Steps, locationMap, "/steps")

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

	diagnostics := validator.Validate(parsedConfig, locationMap, path)

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

// attachSourceLocations populates SourceLocation for each step from the locationMap.
// This captures the exact line number where each step is defined in the YAML source.
// The basePath parameter is the JSON pointer prefix:
//   - "" for old format (plain array)
//   - "/steps" for new format (RunConfig with steps field)
func attachSourceLocations(steps []Step, locationMap *LocationMap, basePath string) {
	for i := range steps {
		// Build JSON pointer path for this step
		// e.g., "/0", "/1" for old format or "/steps/0", "/steps/1" for new format
		stepPath := formatArrayPath(basePath, i)

		// Get position from locationMap
		pos := locationMap.Get(stepPath)
		if pos.Line > 0 {
			steps[i].SourceLocation = &Position{
				Line:   pos.Line,
				Column: pos.Column,
			}
		}
	}
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
		closeErr := file.Close()
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "failed to close variables file %s: %v\n", path, closeErr)
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
		return nil, nil, fmt.Errorf("internal error: defaultReader is not a YAMLConfigReader")
	}
	return reader.ReadConfigWithValidation(path)
}

// ReadVariables is a convenience function using the default YAML reader
func ReadVariables(path string) (map[string]interface{}, error) {
	return defaultReader.ReadVariables(path)
}
