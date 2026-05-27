package kernel

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/cmd/cmdutil"
	"github.com/alehatsman/mooncake/internal/config"
)

// ValidateCommand returns the `mooncake validate` cli.Command.
func ValidateCommand() *cli.Command {
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
	configPath, err := cmdutil.ResolveConfigPath(c)
	if err != nil {
		if cmdutil.PrintNoConfigHintAndExit(err, "validate") {
			return nil
		}
		return err
	}
	format := c.String("format")

	// F052: return cli.ExitCoder errors instead of calling os.Exit.
	// Kernel verbs are embeddable (MCP, SDK, agent loop); a hard
	// exit would kill the host process. urfave/cli reads the
	// ExitCoder's code and exits the CLI process with it, so
	// operator UX is identical.
	_, diagnostics, err := config.ReadConfigWithValidation(configPath)
	if err != nil {
		return cli.Exit(fmt.Sprintf("Error reading config: %v", err), exitCodeRuntimeError)
	}

	hasErrors := config.HasErrors(diagnostics)

	if format == outputFormatJSON {
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
			return cli.Exit(fmt.Sprintf("Error encoding JSON: %v", err), exitCodeRuntimeError)
		}
	} else {
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

	if hasErrors {
		// Diagnostics already printed above; silent ExitCoder
		// carries only the exit code.
		return cli.Exit("", exitCodeValidationError)
	}

	return nil
}
