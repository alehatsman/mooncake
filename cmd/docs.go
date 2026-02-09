package main

import (
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/docgen"
	"github.com/urfave/cli/v2"
)

// docsCommand creates the docs command with subcommands.
func docsCommand() *cli.Command {
	return &cli.Command{
		Name:  "docs",
		Usage: "Generate documentation from action metadata",
		Description: `Generate documentation from action metadata to keep docs in sync with code.

Supports multiple sections:
  - platform-matrix:   Platform support table for all actions
  - capabilities:      Action capabilities table (dry-run, become, etc.)
  - action-summary:    Detailed action summaries grouped by category
  - preset-examples:   Examples from actual preset files (validates syntax)
  - schema:            YAML schema from Go struct definitions
  - all:               Generate all sections (except preset-examples)

Examples:
  mooncake docs generate --section platform-matrix
  mooncake docs generate --section all --output docs-next/generated/actions.md
  mooncake docs generate --section preset-examples --presets-dir ./presets
  mooncake docs generate --section schema`,
		Subcommands: []*cli.Command{
			{
				Name:  "generate",
				Usage: "Generate documentation sections",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "section",
						Aliases: []string{"s"},
						Value:   "all",
						Usage:   "Section to generate (platform-matrix, capabilities, action-summary, preset-examples, schema, all)",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output file (default: stdout)",
					},
					&cli.StringFlag{
						Name:  "presets-dir",
						Value: "presets",
						Usage: "Directory containing preset files (for preset-examples section)",
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Preview output without writing files",
					},
				},
				Action: generateDocsAction,
			},
		},
	}
}

// generateDocsAction handles the docs generate command.
func generateDocsAction(c *cli.Context) error {
	section := c.String("section")
	output := c.String("output")
	presetsDir := c.String("presets-dir")
	dryRun := c.Bool("dry-run")

	// Get version from app context
	version := c.App.Version
	if version == "" {
		version = "dev"
	}

	// Create generator
	generator := docgen.NewGenerator(version)

	// Determine output writer
	var writer *os.File
	var err error

	if output == "" || dryRun {
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

	// Generate documentation
	if err := generator.GenerateSection(section, writer, presetsDir); err != nil {
		return fmt.Errorf("failed to generate documentation: %w", err)
	}

	// Print success message if writing to file
	if output != "" && !dryRun {
		fmt.Fprintf(os.Stderr, "✓ Generated %s documentation to %s\n", section, output)
	}

	return nil
}
