package main

import (
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/schemagen"
	"github.com/urfave/cli/v2"
)

// schemaCommand creates the schema command with subcommands.
func schemaCommand() *cli.Command {
	return &cli.Command{
		Name:  "schema",
		Usage: "Generate JSON Schema and OpenAPI specifications from action metadata",
		Description: `Generate JSON Schema, OpenAPI specifications, and TypeScript definitions
from mooncake's action registry and Go struct definitions.

This ensures the schema is always in sync with the code and provides
IDE autocomplete, validation, and API documentation.

Formats:
  - json:       JSON Schema for YAML validation (default)
  - yaml:       JSON Schema in YAML format
  - openapi:    OpenAPI 3.0 specification
  - typescript: TypeScript definitions (.d.ts)

Examples:
  mooncake schema generate
  mooncake schema generate --output schema.json
  mooncake schema generate --format yaml --output schema.yml
  mooncake schema generate --format openapi --output openapi.json
  mooncake schema generate --format typescript --output mooncake.d.ts
  mooncake schema validate --schema schema.json --config config.yml`,
		Subcommands: []*cli.Command{
			{
				Name:  "generate",
				Usage: "Generate schema from action metadata",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Value:   "json",
						Usage:   "Output format (json, yaml, openapi, typescript)",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output file (default: stdout)",
					},
					&cli.BoolFlag{
						Name:  "extensions",
						Value: true,
						Usage: "Include custom x- extensions (platforms, capabilities)",
					},
					&cli.BoolFlag{
						Name:  "examples",
						Usage: "Include example values in schema",
					},
					&cli.BoolFlag{
						Name:  "strict",
						Value: true,
						Usage: "Generate stricter validation rules (oneOf, additionalProperties)",
					},
				},
				Action: generateSchemaAction,
			},
			{
				Name:  "validate",
				Usage: "Validate existing schema against current code",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "schema",
						Aliases:  []string{"s"},
						Required: true,
						Usage:    "Schema file to validate",
					},
				},
				Action: validateSchemaAction,
			},
		},
	}
}

// generateSchemaAction handles the schema generate command.
func generateSchemaAction(c *cli.Context) error {
	format := c.String("format")
	output := c.String("output")
	includeExtensions := c.Bool("extensions")
	includeExamples := c.Bool("examples")
	strictValidation := c.Bool("strict")

	// Validate format
	validFormats := map[string]bool{
		"json":       true,
		"yaml":       true,
		"openapi":    true,
		"typescript": true,
	}

	if supported, ok := validFormats[format]; !ok {
		return fmt.Errorf("unknown format: %s (supported: json, yaml, openapi)", format)
	} else if !supported {
		return fmt.Errorf("format %s is not yet implemented (coming soon)", format)
	}

	// Create generator
	opts := schemagen.GeneratorOptions{
		IncludeExamples:   includeExamples,
		IncludeExtensions: includeExtensions,
		StrictValidation:  strictValidation,
		OutputFormat:      format,
	}
	generator := schemagen.NewGenerator(opts)

	// Determine output writer
	var writer *os.File
	var err error
	if output == "" {
		writer = os.Stdout
	} else {
		writer, err = os.Create(output) // #nosec G304 -- output path provided by user via CLI flag
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer func() {
			if closeErr := writer.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to close output file: %v\n", closeErr)
			}
		}()
	}

	// Generate and write based on format
	if format == "openapi" {
		// Generate OpenAPI spec
		spec, err := generator.GenerateOpenAPI()
		if err != nil {
			return fmt.Errorf("failed to generate OpenAPI spec: %w", err)
		}

		// Write OpenAPI
		schemaWriter := schemagen.NewWriter("json") // OpenAPI defaults to JSON
		if err := schemaWriter.WriteOpenAPI(spec, writer); err != nil {
			return fmt.Errorf("failed to write OpenAPI spec: %w", err)
		}

		// Print success message if writing to file
		if output != "" {
			fmt.Fprintf(os.Stderr, "✓ Generated OpenAPI 3.0 spec to %s\n", output)
			fmt.Fprintf(os.Stderr, "  Use with Swagger UI, ReDoc, or openapi-generator\n")
		}
	} else if format == "typescript" {
		// Generate TypeScript definitions
		tsContent, err := generator.GenerateTypeScript()
		if err != nil {
			return fmt.Errorf("failed to generate TypeScript definitions: %w", err)
		}

		// Write TypeScript
		schemaWriter := schemagen.NewWriter("typescript")
		if err := schemaWriter.WriteTypeScript(tsContent, writer); err != nil {
			return fmt.Errorf("failed to write TypeScript definitions: %w", err)
		}

		// Print success message if writing to file
		if output != "" {
			fmt.Fprintf(os.Stderr, "✓ Generated TypeScript definitions to %s\n", output)
			fmt.Fprintf(os.Stderr, "  Use with TypeScript editors for autocomplete and type checking\n")
		}
	} else {
		// Generate JSON Schema
		schema, err := generator.Generate()
		if err != nil {
			return fmt.Errorf("failed to generate schema: %w", err)
		}

		// Write schema
		schemaWriter := schemagen.NewWriter(format)
		if err := schemaWriter.Write(schema, writer); err != nil {
			return fmt.Errorf("failed to write schema: %w", err)
		}

		// Print success message if writing to file
		if output != "" {
			fmt.Fprintf(os.Stderr, "✓ Generated %s schema to %s\n", format, output)
			fmt.Fprintf(os.Stderr, "  Configure your editor to use this schema for YAML validation\n")
		}
	}

	return nil
}

// validateSchemaAction handles the schema validate command.
func validateSchemaAction(c *cli.Context) error {
	schemaPath := c.String("schema")

	// Read existing schema
	schemaData, err := os.ReadFile(schemaPath) // #nosec G304 -- schema path provided by user via CLI flag
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Generate current schema
	opts := schemagen.GeneratorOptions{
		IncludeExtensions: true,
		IncludeExamples:   false,
		StrictValidation:  false,
		OutputFormat:      "json",
	}
	generator := schemagen.NewGenerator(opts)
	currentSchema, err := generator.Generate()
	if err != nil {
		return fmt.Errorf("failed to generate current schema: %w", err)
	}

	// Marshal current schema
	currentData, err := currentSchema.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal current schema: %w", err)
	}

	// Compare schemas (simple byte comparison for now)
	if string(schemaData) == string(currentData) {
		fmt.Println("✓ Schema is up to date")
		return nil
	}

	// Schema differs
	fmt.Println("✗ Schema is out of date")
	fmt.Println("  Run 'mooncake schema generate' to update")
	os.Exit(1)
	return nil
}
