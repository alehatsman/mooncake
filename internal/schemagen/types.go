// Package schemagen provides JSON Schema generation from action metadata.
//
// This package generates JSON Schema, OpenAPI specifications, and TypeScript
// definitions from mooncake's action registry and Go struct definitions.
//
// The generator uses multiple sources:
//  1. Action metadata from actions.List() (platforms, capabilities)
//  2. Go struct tags from config structs (types, descriptions)
//  3. Custom schema hints for complex validations
//
// Usage:
//
//	schema := schemagen.Generate()
//	json, _ := schema.MarshalJSON()
package schemagen

// Schema represents a complete JSON Schema document.
type Schema struct {
	SchemaURI   string                 `json:"$schema" yaml:"$schema"`
	ID          string                 `json:"$id,omitempty" yaml:"$id,omitempty"`
	Title       string                 `json:"title" yaml:"title"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Type        string                 `json:"type,omitempty" yaml:"type,omitempty"`
	Items       *SchemaRef             `json:"items,omitempty" yaml:"items,omitempty"`
	OneOf       []*OneOfConstraint     `json:"oneOf,omitempty" yaml:"oneOf,omitempty"`
	Definitions map[string]*Definition `json:"definitions,omitempty" yaml:"definitions,omitempty"`
}

// Definition represents a schema definition (typically for an action).
type Definition struct {
	Type        string                 `json:"type" yaml:"type"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Properties  map[string]*Property   `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required    []string               `json:"required,omitempty" yaml:"required,omitempty"`
	OneOf       []*OneOfConstraint     `json:"oneOf,omitempty" yaml:"oneOf,omitempty"`
	AnyOf       []*SchemaRef           `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
	AllOf       []*SchemaRef           `json:"allOf,omitempty" yaml:"allOf,omitempty"`

	// additionalProperties controls whether unknown properties are allowed
	AdditionalProperties *bool `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`

	// Custom extensions for mooncake
	XPlatforms       []string `json:"x-platforms,omitempty" yaml:"x-platforms,omitempty"`
	XRequiresSudo    bool     `json:"x-requires-sudo,omitempty" yaml:"x-requires-sudo,omitempty"`
	XImplementsCheck bool     `json:"x-implements-check,omitempty" yaml:"x-implements-check,omitempty"`
	XCategory        string   `json:"x-category,omitempty" yaml:"x-category,omitempty"`
	XSupportsDryRun  bool     `json:"x-supports-dry-run,omitempty" yaml:"x-supports-dry-run,omitempty"`
	XSupportsBecome  bool     `json:"x-supports-become,omitempty" yaml:"x-supports-become,omitempty"`
	XVersion         string   `json:"x-version,omitempty" yaml:"x-version,omitempty"`
	XEmitsEvents     []string `json:"x-emits-events,omitempty" yaml:"x-emits-events,omitempty"`
}

// OneOfConstraint represents a oneOf constraint with required and not clauses.
// It can also represent complete schema alternatives (for root-level oneOf).
type OneOfConstraint struct {
	// For mutual exclusion constraints at step level
	Required   []string              `json:"required,omitempty" yaml:"required,omitempty"`
	Properties map[string]*Property  `json:"properties,omitempty" yaml:"properties,omitempty"`
	Not        *NotConstraint        `json:"not,omitempty" yaml:"not,omitempty"`

	// For root-level oneOf alternatives (complete schemas)
	Type        string      `json:"type,omitempty" yaml:"type,omitempty"`
	Items       *SchemaRef  `json:"items,omitempty" yaml:"items,omitempty"`
	Ref         string      `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
}

// NotConstraint represents a "not" constraint with anyOf clauses.
type NotConstraint struct {
	AnyOf []*RequiredConstraint `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
}

// Property represents a schema property (field in an action struct).
type Property struct {
	Type        string                 `json:"type,omitempty" yaml:"type,omitempty"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Ref         string                 `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default     interface{}            `json:"default,omitempty" yaml:"default,omitempty"`
	Items       *Property              `json:"items,omitempty" yaml:"items,omitempty"`
	Properties  map[string]*Property   `json:"properties,omitempty" yaml:"properties,omitempty"`
	OneOf       []*Property            `json:"oneOf,omitempty" yaml:"oneOf,omitempty"`
	Required    []string               `json:"required,omitempty" yaml:"required,omitempty"`

	// Validation
	Minimum          *float64 `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MinLength        *int     `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength        *int     `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Pattern          string   `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Format           string   `json:"format,omitempty" yaml:"format,omitempty"`

	// Additional metadata
	Example              interface{} `json:"example,omitempty" yaml:"example,omitempty"`
	AdditionalProps      *bool       `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
}

// RequiredConstraint represents a simple required constraint.
type RequiredConstraint struct {
	Required []string `json:"required" yaml:"required"`
}

// SchemaRef represents a reference to another schema definition.
type SchemaRef struct {
	Ref         string `json:"$ref,omitempty"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

// GeneratorOptions configures schema generation behavior.
type GeneratorOptions struct {
	// IncludeExamples adds example values to properties
	IncludeExamples bool

	// IncludeExtensions adds custom x- extensions
	IncludeExtensions bool

	// StrictValidation generates stricter validation rules
	StrictValidation bool

	// OutputFormat specifies the output format (json, yaml, openapi, typescript)
	OutputFormat string
}
