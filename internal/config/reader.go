package config

import (
	"bufio"
	"os"

	"gopkg.in/yaml.v3"
)

// ConfigReader defines the interface for reading configuration and variables
type ConfigReader interface {
	ReadConfig(path string) ([]Step, error)
	ReadVariables(path string) (map[string]interface{}, error)
}

// YAMLConfigReader implements ConfigReader for YAML files
type YAMLConfigReader struct {
	// Can add dependencies here if needed (e.g., FileSystem interface)
}

// NewYAMLConfigReader creates a new YAMLConfigReader
func NewYAMLConfigReader() ConfigReader {
	return &YAMLConfigReader{}
}

// ReadConfig reads configuration steps from a YAML file
func (r *YAMLConfigReader) ReadConfig(path string) ([]Step, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	config := make([]Step, 0)

	decoder := yaml.NewDecoder(f)

	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// ReadVariables reads variables from a YAML file
func (r *YAMLConfigReader) ReadVariables(path string) (map[string]interface{}, error) {
	if path == "" {
		return make(map[string]interface{}), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

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

// ReadVariables is a convenience function using the default YAML reader
func ReadVariables(path string) (map[string]interface{}, error) {
	return defaultReader.ReadVariables(path)
}
