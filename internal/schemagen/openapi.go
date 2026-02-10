package schemagen

// OpenAPISpec represents an OpenAPI 3.0 specification document.
type OpenAPISpec struct {
	OpenAPI    string                `json:"openapi" yaml:"openapi"`
	Info       OpenAPIInfo           `json:"info" yaml:"info"`
	Servers    []OpenAPIServer       `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths      map[string]OpenAPIPath `json:"paths,omitempty" yaml:"paths,omitempty"`
	Components OpenAPIComponents     `json:"components" yaml:"components"`
}

// OpenAPIInfo contains API metadata.
type OpenAPIInfo struct {
	Title       string         `json:"title" yaml:"title"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string         `json:"version" yaml:"version"`
	Contact     *OpenAPIContact `json:"contact,omitempty" yaml:"contact,omitempty"`
	License     *OpenAPILicense `json:"license,omitempty" yaml:"license,omitempty"`
}

// OpenAPIContact contains contact information.
type OpenAPIContact struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	URL   string `json:"url,omitempty" yaml:"url,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
}

// OpenAPILicense contains license information.
type OpenAPILicense struct {
	Name string `json:"name" yaml:"name"`
	URL  string `json:"url,omitempty" yaml:"url,omitempty"`
}

// OpenAPIServer describes a server.
type OpenAPIServer struct {
	URL         string `json:"url" yaml:"url"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// OpenAPIPath represents API path operations.
type OpenAPIPath struct {
	Summary     string          `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty"`
	Get         *OpenAPIOperation `json:"get,omitempty" yaml:"get,omitempty"`
	Post        *OpenAPIOperation `json:"post,omitempty" yaml:"post,omitempty"`
}

// OpenAPIOperation describes a single API operation.
type OpenAPIOperation struct {
	Summary     string                    `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string                    `json:"description,omitempty" yaml:"description,omitempty"`
	OperationID string                    `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Tags        []string                  `json:"tags,omitempty" yaml:"tags,omitempty"`
	RequestBody *OpenAPIRequestBody       `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse `json:"responses" yaml:"responses"`
}

// OpenAPIRequestBody describes a request body.
type OpenAPIRequestBody struct {
	Description string                       `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool                         `json:"required,omitempty" yaml:"required,omitempty"`
	Content     map[string]OpenAPIMediaType  `json:"content" yaml:"content"`
}

// OpenAPIResponse describes a response.
type OpenAPIResponse struct {
	Description string                       `json:"description" yaml:"description"`
	Content     map[string]OpenAPIMediaType  `json:"content,omitempty" yaml:"content,omitempty"`
}

// OpenAPIMediaType describes a media type.
type OpenAPIMediaType struct {
	Schema   *OpenAPISchema    `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example  interface{}       `json:"example,omitempty" yaml:"example,omitempty"`
	Examples map[string]OpenAPIExample `json:"examples,omitempty" yaml:"examples,omitempty"`
}

// OpenAPIExample represents an example.
type OpenAPIExample struct {
	Summary     string      `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Value       interface{} `json:"value,omitempty" yaml:"value,omitempty"`
}

// OpenAPIComponents contains reusable components.
type OpenAPIComponents struct {
	Schemas map[string]*OpenAPISchema `json:"schemas,omitempty" yaml:"schemas,omitempty"`
}

// OpenAPISchema represents a schema (simplified from JSON Schema).
// OpenAPI 3.0 uses a subset of JSON Schema with some modifications.
type OpenAPISchema struct {
	Type        string                   `json:"type,omitempty" yaml:"type,omitempty"`
	Description string                   `json:"description,omitempty" yaml:"description,omitempty"`
	Ref         string                   `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Properties  map[string]*OpenAPISchema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Items       *OpenAPISchema           `json:"items,omitempty" yaml:"items,omitempty"`
	Required    []string                 `json:"required,omitempty" yaml:"required,omitempty"`
	Enum        []interface{}            `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default     interface{}              `json:"default,omitempty" yaml:"default,omitempty"`
	Example     interface{}              `json:"example,omitempty" yaml:"example,omitempty"`

	// OneOf, AnyOf, AllOf support
	OneOf []*OpenAPISchema `json:"oneOf,omitempty" yaml:"oneOf,omitempty"`
	AnyOf []*OpenAPISchema `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
	AllOf []*OpenAPISchema `json:"allOf,omitempty" yaml:"allOf,omitempty"`
	Not   *OpenAPISchema   `json:"not,omitempty" yaml:"not,omitempty"`

	// Validation
	Minimum          *float64 `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MinLength        *int     `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength        *int     `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Pattern          string   `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Format           string   `json:"format,omitempty" yaml:"format,omitempty"`

	AdditionalProperties *bool `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`

	// Custom extensions (x- prefixed fields are allowed in OpenAPI)
	XPlatforms       []string `json:"x-platforms,omitempty" yaml:"x-platforms,omitempty"`
	XRequiresSudo    bool     `json:"x-requires-sudo,omitempty" yaml:"x-requires-sudo,omitempty"`
	XImplementsCheck bool     `json:"x-implements-check,omitempty" yaml:"x-implements-check,omitempty"`
	XCategory        string   `json:"x-category,omitempty" yaml:"x-category,omitempty"`
	XSupportsDryRun  bool     `json:"x-supports-dry-run,omitempty" yaml:"x-supports-dry-run,omitempty"`
	XSupportsBecome  bool     `json:"x-supports-become,omitempty" yaml:"x-supports-become,omitempty"`
	XVersion         string   `json:"x-version,omitempty" yaml:"x-version,omitempty"`
	XEmitsEvents     []string `json:"x-emits-events,omitempty" yaml:"x-emits-events,omitempty"`
}

// ConvertToOpenAPI converts a JSON Schema to OpenAPI 3.0 format.
func (s *Schema) ConvertToOpenAPI() *OpenAPISpec {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:       s.Title,
			Description: s.Description,
			Version:     "0.3.0", // Mooncake version
			Contact: &OpenAPIContact{
				Name: "Mooncake Project",
				URL:  "https://github.com/alehatsman/mooncake",
			},
			License: &OpenAPILicense{
				Name: "MIT",
				URL:  "https://opensource.org/licenses/MIT",
			},
		},
		Components: OpenAPIComponents{
			Schemas: make(map[string]*OpenAPISchema),
		},
	}

	// Convert all definitions to OpenAPI schemas
	for name, def := range s.Definitions {
		spec.Components.Schemas[name] = convertDefinitionToOpenAPISchema(def)
	}

	// Add root schema for the config (supports both array and RunConfig formats)
	if len(s.OneOf) > 0 {
		// Schema uses oneOf for multiple formats
		oneOfSchemas := make([]*OpenAPISchema, 0, len(s.OneOf))
		for _, constraint := range s.OneOf {
			schema := &OpenAPISchema{}
			if constraint.Type != "" {
				schema.Type = constraint.Type
				if constraint.Items != nil {
					schema.Items = &OpenAPISchema{
						Ref: convertRef(constraint.Items.Ref),
					}
				}
			} else if constraint.Ref != "" {
				schema.Ref = convertRef(constraint.Ref)
			}
			oneOfSchemas = append(oneOfSchemas, schema)
		}
		spec.Components.Schemas["config"] = &OpenAPISchema{
			Description: "Configuration can be either an array of steps (old format) or a RunConfig object (new format)",
			OneOf:       oneOfSchemas,
		}
	} else if s.Type != "" {
		// Legacy: schema has a single type
		spec.Components.Schemas["config"] = &OpenAPISchema{
			Type:        s.Type,
			Description: "Array of configuration steps",
			Items: &OpenAPISchema{
				Ref: convertRef(s.Items.Ref),
			},
		}
	}

	return spec
}

// convertDefinitionToOpenAPISchema converts a JSON Schema Definition to OpenAPI Schema.
func convertDefinitionToOpenAPISchema(def *Definition) *OpenAPISchema {
	schema := &OpenAPISchema{
		Type:                 def.Type,
		Description:          def.Description,
		Properties:           make(map[string]*OpenAPISchema),
		Required:             def.Required,
		AdditionalProperties: def.AdditionalProperties,
	}

	// Convert properties
	for name, prop := range def.Properties {
		schema.Properties[name] = convertPropertyToOpenAPISchema(prop)
	}

	// Convert oneOf constraints
	if len(def.OneOf) > 0 {
		schema.OneOf = make([]*OpenAPISchema, len(def.OneOf))
		for i, constraint := range def.OneOf {
			schema.OneOf[i] = &OpenAPISchema{
				Required: constraint.Required,
			}
			if constraint.Not != nil {
				schema.OneOf[i].Not = &OpenAPISchema{
					AnyOf: make([]*OpenAPISchema, len(constraint.Not.AnyOf)),
				}
				for j, notConstraint := range constraint.Not.AnyOf {
					schema.OneOf[i].Not.AnyOf[j] = &OpenAPISchema{
						Required: notConstraint.Required,
					}
				}
			}
		}
	}

	// Convert custom extensions
	schema.XPlatforms = def.XPlatforms
	schema.XRequiresSudo = def.XRequiresSudo
	schema.XImplementsCheck = def.XImplementsCheck
	schema.XCategory = def.XCategory
	schema.XSupportsDryRun = def.XSupportsDryRun
	schema.XSupportsBecome = def.XSupportsBecome
	schema.XVersion = def.XVersion
	schema.XEmitsEvents = def.XEmitsEvents

	return schema
}

// convertPropertyToOpenAPISchema converts a JSON Schema Property to OpenAPI Schema.
func convertPropertyToOpenAPISchema(prop *Property) *OpenAPISchema {
	schema := &OpenAPISchema{
		Type:                 prop.Type,
		Description:          prop.Description,
		Ref:                  convertRef(prop.Ref),
		Enum:                 prop.Enum,
		Default:              prop.Default,
		Example:              prop.Example,
		Minimum:              prop.Minimum,
		Maximum:              prop.Maximum,
		MinLength:            prop.MinLength,
		MaxLength:            prop.MaxLength,
		Pattern:              prop.Pattern,
		Format:               prop.Format,
		Required:             prop.Required,
		AdditionalProperties: prop.AdditionalProps,
	}

	// Convert items for arrays
	if prop.Items != nil {
		schema.Items = convertPropertyToOpenAPISchema(prop.Items)
	}

	// Convert nested properties for objects
	if len(prop.Properties) > 0 {
		schema.Properties = make(map[string]*OpenAPISchema)
		for name, nestedProp := range prop.Properties {
			schema.Properties[name] = convertPropertyToOpenAPISchema(nestedProp)
		}
	}

	// Convert oneOf
	if len(prop.OneOf) > 0 {
		schema.OneOf = make([]*OpenAPISchema, len(prop.OneOf))
		for i, oneOfProp := range prop.OneOf {
			schema.OneOf[i] = convertPropertyToOpenAPISchema(oneOfProp)
		}
	}

	return schema
}

// convertRef converts JSON Schema $ref to OpenAPI $ref format.
// JSON Schema: #/definitions/step
// OpenAPI:     #/components/schemas/step
func convertRef(ref string) string {
	if ref == "" {
		return ""
	}
	// Replace #/definitions/ with #/components/schemas/
	if len(ref) > 14 && ref[:14] == "#/definitions/" {
		return "#/components/schemas/" + ref[14:]
	}
	return ref
}
