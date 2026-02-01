// Package plan provides plan generation and persistence for mooncake configurations.
package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SavePlanToFile saves a plan to a file in JSON or YAML format
func SavePlanToFile(p *Plan, filePath string) (err error) {
	ext := filepath.Ext(filePath)

	file, err := os.Create(filePath) //nolint:gosec // filePath is user-provided CLI argument
	if err != nil {
		return fmt.Errorf("failed to create plan file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close plan file: %w", closeErr)
		}
	}()

	switch ext {
	case ".json":
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(p); err != nil {
			return fmt.Errorf("failed to encode plan as JSON: %w", err)
		}
	case ".yaml", ".yml":
		encoder := yaml.NewEncoder(file)
		encoder.SetIndent(2)
		if err := encoder.Encode(p); err != nil {
			return fmt.Errorf("failed to encode plan as YAML: %w", err)
		}
	default:
		return fmt.Errorf("unsupported file format: %s (use .json, .yaml, or .yml)", ext)
	}

	return nil
}

// LoadPlanFromFile loads a plan from a JSON or YAML file
func LoadPlanFromFile(filePath string) (*Plan, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is user-provided CLI argument
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	ext := filepath.Ext(filePath)
	plan := &Plan{}

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, plan); err != nil {
			return nil, fmt.Errorf("failed to decode JSON plan: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, plan); err != nil {
			return nil, fmt.Errorf("failed to decode YAML plan: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s (use .json, .yaml, or .yml)", ext)
	}

	return plan, nil
}
