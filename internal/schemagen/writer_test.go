package schemagen

import (
	"bytes"
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWriterJSON(t *testing.T) {
	schema := &Schema{
		SchemaURI: "http://json-schema.org/draft-07/schema#",
		Title:     "Test Schema",
		Type:      "object",
		Definitions: map[string]*Definition{
			"test": {
				Type:        "object",
				Description: "Test definition",
				Properties:  map[string]*Property{},
			},
		},
	}

	var buf bytes.Buffer
	writer := NewWriter("json")
	err := writer.Write(schema, &buf)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	if result["$schema"] != schema.SchemaURI {
		t.Errorf("Schema URI mismatch")
	}
}

func TestWriterYAML(t *testing.T) {
	schema := &Schema{
		SchemaURI: "http://json-schema.org/draft-07/schema#",
		Title:     "Test Schema",
		Type:      "object",
		Definitions: map[string]*Definition{
			"test": {
				Type:        "object",
				Description: "Test definition",
				Properties:  map[string]*Property{},
			},
		},
	}

	var buf bytes.Buffer
	writer := NewWriter("yaml")
	err := writer.Write(schema, &buf)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Verify it's valid YAML
	var result map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Output is not valid YAML: %v", err)
	}

	if result["$schema"] != schema.SchemaURI {
		t.Errorf("Schema URI mismatch in YAML output")
	}
}

func TestWriterUnsupportedFormat(t *testing.T) {
	schema := &Schema{
		SchemaURI: "http://json-schema.org/draft-07/schema#",
		Title:     "Test Schema",
		Type:      "object",
	}

	var buf bytes.Buffer
	writer := NewWriter("invalid")
	err := writer.Write(schema, &buf)
	if err == nil {
		t.Error("Expected error for unsupported format, got nil")
	}
}
