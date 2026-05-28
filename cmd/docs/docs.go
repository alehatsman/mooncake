// Package docs implements the `mooncake docs` CLI — generate
// documentation from action metadata.
package docs

import (
	"bytes"
	"fmt"
	"os"
	"regexp"

	"github.com/alehatsman/mooncake/internal/docgen"
	"github.com/urfave/cli/v2"
)

// generatedHeaderLine matches the `<!-- Version: ... | Generated: ... -->`
// preamble so we can compare generator output to disk while ignoring the
// timestamp/version, which would otherwise force a write on every run.
var generatedHeaderLine = regexp.MustCompile(`(?m)^<!-- Version: .* \| Generated: .* -->\r?\n`)

func docContentEqual(a, b []byte) bool {
	return bytes.Equal(
		generatedHeaderLine.ReplaceAll(a, nil),
		generatedHeaderLine.ReplaceAll(b, nil),
	)
}

// Command creates the docs command with subcommands.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "docs",
		Usage: "Generate documentation from action metadata",
		Description: `Generate documentation from action metadata to keep docs in sync with code.

Single-section emitters (--output is a single markdown file):
  - platform-matrix:    Platform support table for all actions
  - capabilities:       Action capabilities table (dry-run, become, etc.)
  - action-summary:     Detailed action summaries grouped by category
  - action-properties:  Properties tables from schema.json (auto-generated)
  - preset-examples:    Examples from actual preset files (validates syntax)
  - schema:             YAML schema from Go struct definitions
  - all:                Concatenated platform-matrix + capabilities + action-summary + schema

Multi-file emitter (--output is a DIRECTORY):
  - all-into-dir:       Per-action cards + schema + properties + matrices + preset
                        examples, written into the output dir as a structured tree
                        ready for MkDocs (mkdocs reads dist/docs/) and llms.txt
                        agent consumption.

Examples:
  mooncake docs generate --section platform-matrix
  mooncake docs generate --section all-into-dir --output dist/docs
  mooncake docs generate --section action-properties --output dist/docs/properties.md
  mooncake docs generate --section preset-examples --presets-dir ./presets`,
		Subcommands: []*cli.Command{
			{
				Name:  "generate",
				Usage: "Generate documentation sections",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "section",
						Aliases: []string{"s"},
						Value:   "all",
						Usage:   "Section to generate (platform-matrix, capabilities, action-summary, action-properties, preset-examples, schema, all, all-into-dir)",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output file (single-section) or directory (all-into-dir); default: stdout",
					},
					&cli.StringFlag{
						Name:  "presets-dir",
						Value: "presets",
						Usage: "Directory containing preset files (for preset-examples and all-into-dir sections)",
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
	appVersion := c.App.Version
	if appVersion == "" {
		appVersion = "dev"
	}

	generator := docgen.NewGenerator(appVersion)

	// The "all-into-dir" section is the multi-file dispatcher used by
	// `task docs-generate`. It writes a directory tree (one file per
	// topic) rather than a single markdown file.
	if section == "all-into-dir" {
		if output == "" {
			return fmt.Errorf("--output (target directory) is required for section all-into-dir")
		}
		if dryRun {
			fmt.Fprintf(os.Stderr, "dry-run: would regenerate %s\n", output)
			return nil
		}
		written, err := generator.GenerateDist(docgen.DistOptions{
			OutDir:     output,
			PresetsDir: presetsDir,
			CLIRoot:    c.App,
			EnableAPI:  true,
		})
		if err != nil {
			return fmt.Errorf("failed to generate dist tree: %w", err)
		}
		fmt.Fprintf(os.Stderr, "✓ Generated %d files into %s\n", len(written), output)
		return nil
	}

	if output == "" || dryRun {
		return generator.GenerateSection(section, os.Stdout, presetsDir)
	}

	var buf bytes.Buffer
	if err := generator.GenerateSection(section, &buf, presetsDir); err != nil {
		return fmt.Errorf("failed to generate documentation: %w", err)
	}

	mode := os.FileMode(0o644)
	if info, err := os.Stat(output); err == nil {
		if docContent, readErr := os.ReadFile(output); readErr == nil && docContentEqual(docContent, buf.Bytes()) {
			fmt.Fprintf(os.Stderr, "✓ %s already up to date (%s)\n", section, output)
			return nil
		}
		mode = info.Mode().Perm()
	}

	if err := os.WriteFile(output, buf.Bytes(), mode); err != nil { // #nosec G304 -- output path provided by user via CLI flag
		return fmt.Errorf("failed to write output file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "✓ Generated %s documentation to %s\n", section, output)
	return nil
}
