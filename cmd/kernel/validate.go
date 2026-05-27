package kernel

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/cmd/cmdutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/plan"
)

// runStrictTemplateCheck builds a plan for configPath and returns
// the list of unresolved root identifiers. Used by `validate` to
// surface template typos that the parse-level config validator
// can't see. Returns nil + nil if the plan build itself fails for
// non-template reasons — those errors are surfaced upstream.
func runStrictTemplateCheck(configPath string, varsPaths []string) ([]plan.UnresolvedRef, error) {
	variables := make(map[string]interface{})
	for _, varsPath := range varsPaths {
		if varsPath == "" {
			continue
		}
		vars, err := config.ReadVariables(varsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read variables from %s: %w", varsPath, err)
		}
		for k, v := range vars {
			variables[k] = v
		}
	}
	planner, err := plan.NewPlanner()
	if err != nil {
		return nil, err
	}
	p, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: configPath,
		Variables:  variables,
	})
	if err != nil {
		return nil, fmt.Errorf("build plan: %w", err)
	}
	return p.UnresolvedTemplates, nil
}

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

	// Strict-template scan: only run when parse-level diagnostics
	// are clean. A malformed YAML or unknown action would make the
	// planner crash on something the user already needs to fix.
	var unresolved []plan.UnresolvedRef
	if !hasErrors {
		var checkErr error
		unresolved, checkErr = runStrictTemplateCheck(configPath, c.StringSlice("vars"))
		if checkErr != nil {
			return cli.Exit(fmt.Sprintf("Error during strict-template check: %v", checkErr), exitCodeRuntimeError)
		}
	}
	hasUnresolved := len(unresolved) > 0

	if format == outputFormatJSON {
		type ValidationResult struct {
			Valid               bool                 `json:"valid"`
			Diagnostics         []config.Diagnostic  `json:"diagnostics,omitempty"`
			UnresolvedTemplates []plan.UnresolvedRef `json:"unresolved_templates,omitempty"`
		}
		result := ValidationResult{
			Valid:               !hasErrors && !hasUnresolved,
			Diagnostics:         diagnostics,
			UnresolvedTemplates: unresolved,
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
		if hasUnresolved {
			fmt.Print(FormatUnresolvedTemplates(unresolved))
		}

		if hasErrors || hasUnresolved {
			fmt.Println("\n❌ Validation failed")
		} else if len(diagnostics) > 0 {
			fmt.Println("\n⚠️  Validation passed with warnings")
		} else {
			fmt.Println("✓ Configuration is valid")
		}
	}

	if hasErrors || hasUnresolved {
		// Diagnostics already printed above; silent ExitCoder
		// carries only the exit code.
		return cli.Exit("", exitCodeValidationError)
	}

	return nil
}
