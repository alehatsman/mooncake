# Proposal: Schema Generation from Action Metadata

**Status**: ✅ Phases 1-3 Implemented (2026-02-09)
**Author**: Automation Discussion
**Date**: 2026-02-07
**Implementation**: commits 51232f3 (schema), [current] (openapi)
**Related**: Automated Documentation Generation, Platform Support

## Implementation Summary

**Phase 1-2: JSON Schema** ✅ Complete (2026-02-09)
- ✅ JSON Schema generation from action metadata
- ✅ Full validation support (oneOf, enums, patterns, ranges)
- ✅ Custom x- extensions (platforms, capabilities)
- ✅ CLI command: `mooncake schema generate`
- ✅ Auto-generated schema (1,853 lines vs 2,036 manual)
- ✅ 10 passing tests with 64.9% coverage
- ✅ Documentation updated

**Phase 3: OpenAPI 3.0** ✅ Complete (2026-02-09)
- ✅ OpenAPI 3.0.3 specification generation
- ✅ Conversion from JSON Schema to OpenAPI format
- ✅ Custom x- extensions preserved
- ✅ CLI: `mooncake schema generate --format openapi`
- ✅ 6 comprehensive tests for OpenAPI
- ✅ Documentation updated with use cases

**Location**:
- `internal/schemagen/` package (generator.go, types.go, openapi.go)
- `cmd/schema.go` (CLI command)

**Remaining**: Phase 4+ (TypeScript definitions) deferred to future

## Summary

Generate JSON Schema and OpenAPI specifications from action metadata to provide IDE autocomplete, validation, and API documentation.

## Motivation

### Current State

The JSON schema is manually maintained in `internal/config/schema.json`:

```json
{
  "definitions": {
    "shell": {
      "type": "object",
      "properties": {
        "cmd": {"type": "string"},
        "interpreter": {"type": "string"}
      },
      "required": ["cmd"]
    }
  }
}
```

### Problems

1. **Manual maintenance** - Every action field change requires schema update
2. **Out of sync risk** - Schema can drift from actual Go structs
3. **Duplication** - Information exists in Go structs and JSON schema
4. **No validation** - Can't verify schema matches code
5. **Missing features** - Platform-specific validation, conditional properties

## Proposed Solution

### Generate Schema from Multiple Sources

```
Action Metadata (actions.List())
    +
Go Struct Tags (config.Step)
    +
Custom Schema Hints
    ↓
JSON Schema Generator
    ↓
├── JSON Schema (for YAML validation)
├── OpenAPI 3.0 (for API docs)
└── TypeScript definitions (for web tools)
```

### Example: Service Action

**Input (Go code):**
```go
type ServiceAction struct {
    Name    string  `json:"name" yaml:"name" schema:"required,description=Service name"`
    State   string  `json:"state" yaml:"state" schema:"enum=started|stopped|restarted"`
    Enabled *bool   `json:"enabled" yaml:"enabled" schema:"description=Enable on boot"`
}

func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:               "service",
        Description:        "Manage system services",
        SupportedPlatforms: []string{"linux", "darwin", "windows"},
        // ...
    }
}
```

**Output (JSON Schema):**
```json
{
  "service": {
    "type": "object",
    "description": "Manage system services",
    "properties": {
      "name": {
        "type": "string",
        "description": "Service name"
      },
      "state": {
        "type": "string",
        "enum": ["started", "stopped", "restarted"]
      },
      "enabled": {
        "type": "boolean",
        "description": "Enable on boot"
      }
    },
    "required": ["name"],
    "x-platforms": ["linux", "darwin", "windows"],
    "x-requires-sudo": true
  }
}
```

## Implementation Options

### Option 1: Custom Generator (Recommended)

Build a Go program that:
1. Uses reflection to introspect Go structs
2. Reads struct tags for schema hints
3. Queries `actions.List()` for metadata
4. Generates JSON Schema with custom extensions

**Command:**
```bash
mooncake schema generate --output schema.json
mooncake schema generate --format openapi --output openapi.yaml
mooncake schema generate --format typescript --output types.d.ts
```

**Pros:**
- Full control over output
- Can add custom extensions (x-platforms, x-requires-sudo)
- Integrates with existing action registry
- Can generate multiple formats

**Cons:**
- Custom code to maintain
- Need to handle all Go types

### Option 2: Use Existing Library

Use `github.com/alecthomas/jsonschema` or similar:

```go
import "github.com/alecthomas/jsonschema"

type Step struct {
    Shell *ShellAction `json:"shell" jsonschema:"description=Execute shell commands"`
}

schema := jsonschema.Reflect(&Step{})
```

**Pros:**
- Well-tested library
- Less code to write
- Standard Go approach

**Cons:**
- Limited customization
- Can't easily integrate action metadata
- May not support all features we need

### Option 3: Hybrid Approach

Use library for basic schema generation, enhance with metadata:

```go
// Generate base schema from structs
baseSchema := jsonschema.Reflect(&config.Step{})

// Enhance with action metadata
for _, meta := range actions.List() {
    actionSchema := baseSchema.Definitions[meta.Name]
    actionSchema.Extensions["x-platforms"] = meta.SupportedPlatforms
    actionSchema.Extensions["x-requires-sudo"] = meta.RequiresSudo
}
```

**Pros:**
- Best of both worlds
- Less custom code
- Can add our enhancements

**Cons:**
- Dependency on external library
- Still need custom code for enhancements

## Detailed Design (Option 1)

### CLI Interface

```bash
# Generate JSON Schema
mooncake schema generate

# Generate with custom output
mooncake schema generate --output config-schema.json

# Generate OpenAPI spec
mooncake schema generate --format openapi --output openapi.yaml

# Generate TypeScript definitions
mooncake schema generate --format typescript --output mooncake.d.ts

# Validate existing schema
mooncake schema validate --schema schema.json
```

### Schema Extensions

Add custom fields for Mooncake-specific features:

```json
{
  "service": {
    "type": "object",
    "properties": { ... },

    // Standard JSON Schema
    "required": ["name"],

    // Mooncake extensions
    "x-platforms": ["linux", "darwin", "windows"],
    "x-requires-sudo": true,
    "x-implements-check": true,
    "x-category": "system",
    "x-dry-run": true,
    "x-examples": [
      {
        "name": "Start nginx",
        "service": {
          "name": "nginx",
          "state": "started"
        }
      }
    ]
  }
}
```

### Implementation Steps

#### Phase 1: Basic Schema Generation (Week 1)

1. **Create schema generator package** (`internal/schemagen/`)
   - `generator.go` - Main generator
   - `reflect.go` - Struct reflection
   - `writer.go` - Output writers

2. **Add CLI command** (`cmd/schema.go`)
   - `schema generate` subcommand
   - Format selection (json, yaml, openapi)

3. **Generate basic schemas**
   - Reflect over `config.Step` struct
   - Extract field types and tags
   - Write JSON Schema

#### Phase 2: Metadata Integration (Week 2)

4. **Integrate action metadata**
   - Query `actions.List()`
   - Add platform support info
   - Add capability flags (sudo, check)

5. **Add custom extensions**
   - `x-platforms`
   - `x-requires-sudo`
   - `x-implements-check`

#### Phase 3: Multi-Format Support (Week 3)

6. **OpenAPI generation**
   - Convert to OpenAPI 3.0 format
   - Add operation definitions
   - Include examples

7. **TypeScript definitions**
   - Generate `.d.ts` files
   - Type-safe config editing
   - IDE support

### Generated Formats

#### 1. JSON Schema

Used for YAML validation in editors:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Mooncake Configuration",
  "type": "object",
  "properties": {
    "name": {"type": "string"},
    "steps": {
      "type": "array",
      "items": {"$ref": "#/definitions/Step"}
    }
  },
  "definitions": {
    "Step": {
      "type": "object",
      "oneOf": [
        {"$ref": "#/definitions/ShellStep"},
        {"$ref": "#/definitions/ServiceStep"}
      ]
    }
  }
}
```

#### 2. OpenAPI 3.0

Used for API documentation and client generation:

```yaml
openapi: 3.0.0
info:
  title: Mooncake Configuration API
  version: 1.0.0

components:
  schemas:
    ServiceAction:
      type: object
      properties:
        name:
          type: string
          description: Service name
        state:
          type: string
          enum: [started, stopped, restarted]
      required: [name]
      x-platforms: [linux, darwin, windows]
```

#### 3. TypeScript Definitions

Used for web-based config editors:

```typescript
export interface ServiceAction {
  /** Service name */
  name: string;

  /** Service state */
  state?: "started" | "stopped" | "restarted";

  /** Enable on boot */
  enabled?: boolean;
}

export interface Step {
  name?: string;
  service?: ServiceAction;
  shell?: ShellAction;
  // ... other actions
}
```

## IDE Integration

### VSCode

With generated JSON Schema, users get:

**1. Autocomplete:**
```yaml
steps:
  - name: Start service
    service:  # <-- Ctrl+Space shows: name, state, enabled
      na      # <-- Autocomplete suggests "name"
```

**2. Validation:**
```yaml
steps:
  - service:
      name: nginx
      state: invalid  # ❌ Error: Must be one of: started, stopped, restarted
```

**3. Hover Documentation:**
```yaml
service:  # 📖 Hover shows: "Manage system services"
          #     Platforms: linux, darwin, windows
          #     Requires Sudo: Yes
```

### Configuration

**.vscode/settings.json:**
```json
{
  "yaml.schemas": {
    "schema.json": "*.yml"
  }
}
```

## Benefits

### For Users

1. **IDE autocomplete** - Faster config writing
2. **Inline validation** - Catch errors before running
3. **Documentation** - Hover to see action help
4. **Type safety** - Enum validation, required fields

### For Developers

1. **Single source of truth** - Schema from code
2. **No manual updates** - Schema always in sync
3. **Multi-format export** - JSON Schema, OpenAPI, TypeScript
4. **Validation in CI** - Catch schema drift

### For Project

1. **Better DX** - Professional IDE support
2. **API documentation** - Auto-generated from schema
3. **Client SDKs** - Generate from OpenAPI
4. **Web editors** - TypeScript types for web tools

## Testing Strategy

### Unit Tests

```go
func TestSchemaGeneration(t *testing.T) {
    // Generate schema
    schema := GenerateSchema()

    // Validate structure
    assert.NotNil(t, schema.Definitions["service"])
    assert.Equal(t, []string{"name"}, schema.Definitions["service"].Required)

    // Check extensions
    assert.Equal(t, []string{"linux", "darwin", "windows"},
                 schema.Definitions["service"].Extensions["x-platforms"])
}
```

### Integration Tests

```go
func TestSchemaValidatesConfig(t *testing.T) {
    // Generate schema
    schema := GenerateSchema()

    // Load test config
    config := LoadConfig("testdata/valid.yml")

    // Validate
    err := ValidateAgainstSchema(config, schema)
    assert.NoError(t, err)
}
```

### CI Validation

```bash
# Generate schema
make schema-generate

# Validate all example configs
for f in examples/*.yml; do
  mooncake validate --schema schema.json --config "$f"
done
```

## Migration Plan

### Week 1: Proof of Concept

- Implement basic schema generation
- Generate schema for 3 actions
- Validate against example configs

### Week 2: Full Implementation

- Generate schema for all 14 actions
- Add metadata integration
- Add custom extensions

### Week 3: Documentation & CI

- Document IDE setup (VSCode, IntelliJ)
- Add CI validation
- Publish schema to schema store

## Alternatives Considered

### A. Keep Manual Schema

**Rejected**: High maintenance, out-of-sync risk

### B. Generate from YAML Examples

Parse example configs to infer schema.

**Rejected**: Examples don't capture all features, no type info

### C. Use Protocol Buffers

Define actions in .proto files.

**Rejected**: Not Go-native, adds complexity, doesn't fit project style

## Success Metrics

1. **Zero schema drift** - CI catches any discrepancies
2. **IDE adoption** - 80%+ of users enable schema validation
3. **Reduced config errors** - 50% fewer validation errors
4. **Developer satisfaction** - Survey shows improved DX

## Future Extensions

### Phase 4+: Advanced Features

1. **Conditional schemas**
   - Different properties based on OS
   - Platform-specific validation
   - Example: `service.unit` only on Linux

2. **Schema marketplace**
   - Publish to schemastore.org
   - IDE automatically fetches schema
   - Version-specific schemas

3. **Web-based validator**
   - Validate YAML in browser
   - Real-time error highlighting
   - Suggestion engine

4. **Schema diff tool**
   - Show breaking changes between versions
   - Help users migrate configs
   - Automated upgrade suggestions

## References

- [JSON Schema Specification](https://json-schema.org/)
- [OpenAPI 3.0](https://swagger.io/specification/)
- [VSCode YAML Extension](https://github.com/redhat-developer/vscode-yaml)
- [TypeScript Handbook](https://www.typescriptlang.org/docs/)
- Similar implementations:
  - Kubernetes CRD schemas
  - GitHub Actions workflow schemas
  - Ansible playbook schemas

## Appendix: Example Generator Code

```go
package schemagen

import (
    "encoding/json"
    "reflect"
    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
)

type Schema struct {
    Schema      string                 `json:"$schema"`
    Title       string                 `json:"title"`
    Type        string                 `json:"type"`
    Definitions map[string]Definition  `json:"definitions"`
}

type Definition struct {
    Type        string                 `json:"type"`
    Description string                 `json:"description,omitempty"`
    Properties  map[string]Property    `json:"properties,omitempty"`
    Required    []string               `json:"required,omitempty"`
    Extensions  map[string]interface{} `json:"-"`
}

func GenerateSchema() *Schema {
    schema := &Schema{
        Schema:      "http://json-schema.org/draft-07/schema#",
        Title:       "Mooncake Configuration",
        Type:        "object",
        Definitions: make(map[string]Definition),
    }

    // Generate definitions from actions
    for _, meta := range actions.List() {
        def := generateDefinition(meta)
        schema.Definitions[meta.Name] = def
    }

    return schema
}

func generateDefinition(meta actions.ActionMetadata) Definition {
    def := Definition{
        Type:        "object",
        Description: meta.Description,
        Properties:  make(map[string]Property),
        Extensions:  make(map[string]interface{}),
    }

    // Add custom extensions
    if len(meta.SupportedPlatforms) > 0 {
        def.Extensions["x-platforms"] = meta.SupportedPlatforms
    }
    def.Extensions["x-requires-sudo"] = meta.RequiresSudo
    def.Extensions["x-implements-check"] = meta.ImplementsCheck

    return def
}
```

This generator can be extended to introspect Go structs and generate complete property definitions.
