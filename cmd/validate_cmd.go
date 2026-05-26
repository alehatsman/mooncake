package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/config"
)

// validateCommand returns the `mooncake validate` cli.Command.
func validateCommand() *cli.Command {
	return &cli.Command{
		Name:  "validate",
		Usage: "Validate configuration file without executing",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to configuration file (default: ./mooncake.yml or ./mooncake/main.yml)",
			},
			&cli.StringSliceFlag{
				Name:    "vars",
				Aliases: []string{"v"},
				Usage:   "Path to a variables file. Repeat to layer multiple files; later wins on key collision.",
			},
			&cli.StringFlag{
				// MT-68: accept --output-format too so users
				// who learned the verb on `apply` don't get a
				// "flag not defined" error on `validate`.
				Name:    "format",
				Aliases: []string{"f", "output-format"},
				Value:   "text",
				Usage:   "Output format: text or json",
			},
		},
		Action: validateAction,
	}
}

func validateAction(c *cli.Context) error {
	configPath, err := resolveConfigPath(c)
	if err != nil {
		if printNoConfigHintAndExit(err, "validate") {
			return nil
		}
		return err
	}
	format := c.String("format")

	// Read and validate configuration
	_, diagnostics, err := config.ReadConfigWithValidation(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(exitCodeRuntimeError)
	}

	// Check for validation errors
	hasErrors := config.HasErrors(diagnostics)

	// Output diagnostics
	if format == outputFormatJSON {
		// JSON output
		type ValidationResult struct {
			Valid       bool                `json:"valid"`
			Diagnostics []config.Diagnostic `json:"diagnostics,omitempty"`
		}
		result := ValidationResult{
			Valid:       !hasErrors,
			Diagnostics: diagnostics,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(exitCodeRuntimeError)
		}
	} else {
		// Text output
		if len(diagnostics) > 0 {
			fmt.Println(config.FormatDiagnosticsWithContext(diagnostics))
		}

		if hasErrors {
			fmt.Println("\n❌ Validation failed")
		} else if len(diagnostics) > 0 {
			fmt.Println("\n⚠️  Validation passed with warnings")
		} else {
			fmt.Println("✓ Configuration is valid")
		}
	}

	// Exit with appropriate code
	if hasErrors {
		os.Exit(exitCodeValidationError)
	}

	return nil
}
