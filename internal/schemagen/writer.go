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

// WriteOpenAPI writes an OpenAPI spec to the given writer.
func (w *Writer) WriteOpenAPI(spec *OpenAPISpec, out io.Writer) error {
	switch w.format {
	case "json", "openapi": // openapi defaults to JSON
		return w.writeOpenAPIJSON(spec, out)
	case "yaml":
		return w.writeOpenAPIYAML(spec, out)
	default:
		return fmt.Errorf("unsupported format: %s", w.format)
	}
}

// WriteTypeScript writes TypeScript definitions to the given writer.
func (w *Writer) WriteTypeScript(tsContent string, out io.Writer) error {
	_, err := out.Write([]byte(tsContent))
	return err
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
	defer func() {
		_ = encoder.Close() // Ignore close error as we're already returning encode error
	}()
	return encoder.Encode(schema)
}

// writeOpenAPIJSON writes the OpenAPI spec as JSON.
func (w *Writer) writeOpenAPIJSON(spec *OpenAPISpec, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(spec)
}

// writeOpenAPIYAML writes the OpenAPI spec as YAML.
func (w *Writer) writeOpenAPIYAML(spec *OpenAPISpec, out io.Writer) error {
	encoder := yaml.NewEncoder(out)
	encoder.SetIndent(2)
	defer func() {
		_ = encoder.Close() // Ignore close error as we're already returning encode error
	}()
	return encoder.Encode(spec)
}
