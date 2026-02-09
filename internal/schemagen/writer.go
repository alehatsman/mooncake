package schemagen

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Writer handles writing schemas in different formats.
type Writer struct {
	format string
}

// NewWriter creates a new writer for the specified format.
func NewWriter(format string) *Writer {
	return &Writer{format: format}
}

// Write writes the schema to the given writer.
func (w *Writer) Write(schema *Schema, out io.Writer) error {
	switch w.format {
	case "json":
		return w.writeJSON(schema, out)
	case "yaml":
		return w.writeYAML(schema, out)
	default:
		return fmt.Errorf("unsupported format: %s", w.format)
	}
}

// writeJSON writes the schema as JSON.
func (w *Writer) writeJSON(schema *Schema, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(schema)
}

// writeYAML writes the schema as YAML.
func (w *Writer) writeYAML(schema *Schema, out io.Writer) error {
	encoder := yaml.NewEncoder(out)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(schema)
}

// WriteToFile writes the schema to a file.
func (w *Writer) WriteToFile(schema *Schema, filename string) error {
	// This will be used by the CLI command
	// For now, we'll implement it in the cmd package
	return fmt.Errorf("not implemented: use CLI command instead")
}
