// Package main provides the mooncake CLI application.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/explain"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func run(c *cli.Context) error {
	// Check if running from plan
	fromPlan := c.String("from-plan")
	if fromPlan != "" {
		return runFromPlan(c, fromPlan)
	}

	raw := c.Bool("raw")
	dryRun := c.Bool("dry-run")
	logLevel := c.String("log-level")

	var log logger.Logger

	// Force raw mode for dry-run
	if dryRun {
		raw = true
	}

	// Use animated TUI by default if supported, unless --raw is specified
	if !raw && logger.IsTUISupported() {
		tuiLogger, err := logger.NewTUILogger(logger.InfoLevel)
		if err != nil {
			// Fallback to console logger if TUI initialization fails
			log = logger.NewLogger(logger.InfoLevel)
		} else {
			log = tuiLogger
			tuiLogger.Start()
			defer tuiLogger.Stop()
		}
	} else {
		// Use console logger for raw output or when TUI is not supported
		log = logger.NewLogger(logger.InfoLevel)
	}

	if err := log.SetLogLevelStr(logLevel); err != nil {
		return err
	}

	// Parse tags from comma-separated string
	var tags []string
	tagsStr := c.String("tags")
	if tagsStr != "" {
		for _, tag := range strings.Split(tagsStr, ",") {
			trimmed := strings.TrimSpace(tag)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	// Validate password input methods (mutual exclusion)
	passwordMethods := 0
	if c.String("sudo-pass") != "" {
		passwordMethods++
	}
	if c.Bool("ask-become-pass") {
		passwordMethods++
	}
	if c.String("sudo-pass-file") != "" {
		passwordMethods++
	}

	if passwordMethods > 1 {
		return fmt.Errorf("only one password method can be specified (--sudo-pass, --ask-become-pass, --sudo-pass-file)")
	}

	// Security warning for --sudo-pass
	if c.String("sudo-pass") != "" && !c.Bool("insecure-sudo-pass") {
		return fmt.Errorf("--sudo-pass requires --insecure-sudo-pass flag (WARNING: password will be visible in shell history and process list)")
	}

	return executor.Start(executor.StartConfig{
		ConfigFilePath:   c.String("config"),
		VarsFilePath:     c.String("vars"),
		SudoPass:         c.String("sudo-pass"),
		SudoPassFile:     c.String("sudo-pass-file"),
		AskBecomePass:    c.Bool("ask-become-pass"),
		InsecureSudoPass: c.Bool("insecure-sudo-pass"),
		Tags:             tags,
		DryRun:           dryRun,
	}, log)
}

func runFromPlan(c *cli.Context, planPath string) error {
	// Load plan from file
	planData, err := plan.LoadPlanFromFile(planPath)
	if err != nil {
		return fmt.Errorf("failed to load plan: %w", err)
	}

	// Setup logger
	raw := c.Bool("raw")
	dryRun := c.Bool("dry-run")
	logLevel := c.String("log-level")

	var log logger.Logger

	// Force raw mode for dry-run
	if dryRun {
		raw = true
	}

	// Use animated TUI by default if supported, unless --raw is specified
	if !raw && logger.IsTUISupported() {
		tuiLogger, err := logger.NewTUILogger(logger.InfoLevel)
		if err != nil {
			// Fallback to console logger if TUI initialization fails
			log = logger.NewLogger(logger.InfoLevel)
		} else {
			log = tuiLogger
			tuiLogger.Start()
			defer tuiLogger.Stop()
		}
	} else {
		// Use console logger for raw output or when TUI is not supported
		log = logger.NewLogger(logger.InfoLevel)
	}

	if err := log.SetLogLevelStr(logLevel); err != nil {
		return err
	}

	// Execute plan
	return executor.ExecutePlan(planData, c.String("sudo-pass"), dryRun, log)
}

func explainCommand(_ *cli.Context) error {
	// Collect system facts
	f := facts.Collect()

	// Display facts
	explain.DisplayFacts(f)

	return nil
}

func planCommand(c *cli.Context) error {
	configPath := c.String("config")
	varsPath := c.String("vars")
	outputPath := c.String("output")
	format := c.String("format")
	showOrigins := c.Bool("show-origins")

	// Parse tags
	var tags []string
	tagsStr := c.String("tags")
	if tagsStr != "" {
		for _, tag := range strings.Split(tagsStr, ",") {
			trimmed := strings.TrimSpace(tag)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	// Load variables if specified
	var variables map[string]interface{}
	if varsPath != "" {
		vars, err := config.ReadVariables(varsPath)
		if err != nil {
			return fmt.Errorf("failed to read variables: %w", err)
		}
		variables = vars
	} else {
		variables = make(map[string]interface{})
	}

	// Build plan (planner will inject system facts automatically)
	planner := plan.NewPlanner()
	planData, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: configPath,
		Variables:  variables,
		Tags:       tags,
	})
	if err != nil {
		return fmt.Errorf("failed to build plan: %w", err)
	}

	// Save to file if output path specified
	if outputPath != "" {
		if err := plan.SavePlanToFile(planData, outputPath); err != nil {
			return fmt.Errorf("failed to save plan: %w", err)
		}
		fmt.Printf("Plan saved to %s\n", outputPath)
		return nil
	}

	// Format and display plan
	switch format {
	case "json":
		return formatPlanJSON(planData)
	case "yaml":
		return formatPlanYAML(planData)
	case "text":
		return formatPlanText(planData, showOrigins)
	default:
		return fmt.Errorf("unsupported format: %s (use text, json, or yaml)", format)
	}
}

func formatPlanJSON(p *plan.Plan) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(p)
}

func formatPlanYAML(p *plan.Plan) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer func() {
		_ = encoder.Close()
	}()
	return encoder.Encode(p)
}

func formatPlanText(p *plan.Plan, showOrigins bool) error {
	fmt.Printf("Plan: %s\n", p.RootFile)
	fmt.Printf("Generated: %s\n", p.GeneratedAt.Format("2006-01-02 15:04:05"))
	if len(p.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(p.Tags, ", "))
	}
	fmt.Printf("Steps: %d\n\n", len(p.Steps))

	for i, step := range p.Steps {
		fmt.Printf("[%d] %s (ID: %s)\n", i+1, step.Name, step.ID)

		// Determine action type
		actionType := "unknown"
		if step.Shell != nil {
			actionType = "shell"
		} else if step.File != nil {
			actionType = "file"
		} else if step.Template != nil {
			actionType = "template"
		} else if step.Vars != nil {
			actionType = "vars"
		} else if step.IncludeVars != nil {
			actionType = "include_vars"
		}
		fmt.Printf("    Action: %s\n", actionType)

		if step.Skipped {
			fmt.Printf("    Status: SKIPPED (tags)\n")
		}

		if len(step.Tags) > 0 {
			fmt.Printf("    Tags: %s\n", strings.Join(step.Tags, ", "))
		}

		if showOrigins && step.Origin != nil {
			fmt.Printf("    Origin: %s:%d:%d\n", step.Origin.FilePath, step.Origin.Line, step.Origin.Column)
			if len(step.Origin.IncludeChain) > 0 {
				fmt.Printf("    Chain: %s\n", strings.Join(step.Origin.IncludeChain, " -> "))
			}
		}

		if step.LoopContext != nil {
			fmt.Printf("    Loop: %s[%d] (first=%v, last=%v)\n",
				step.LoopContext.Type, step.LoopContext.Index,
				step.LoopContext.First, step.LoopContext.Last)
		}

		fmt.Println()
	}

	return nil
}

func validateCommand(c *cli.Context) error {
	configPath := c.String("config")
	format := c.String("format")

	// Read and validate configuration
	_, diagnostics, err := config.ReadConfigWithValidation(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(3) // Runtime error
	}

	// Check for validation errors
	hasErrors := config.HasErrors(diagnostics)

	// Output diagnostics
	if format == "json" {
		// JSON output
		type ValidationResult struct {
			Valid       bool                  `json:"valid"`
			Diagnostics []config.Diagnostic   `json:"diagnostics,omitempty"`
		}
		result := ValidationResult{
			Valid:       !hasErrors,
			Diagnostics: diagnostics,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(3)
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
		os.Exit(2) // Validation error
	}

	return nil
}

func createApp() *cli.App {
	app := &cli.App{
		Name:                 "mooncake",
		Usage:                "Space fighters provisioning tool, Chookity!",
		EnableBashCompletion: true,

		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Run a space fighter",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "config",
						Aliases:  []string{"c"},
						Usage:    "Path to configuration file",
					},
					&cli.StringFlag{
						Name:    "vars",
						Aliases: []string{"v"},
						Usage:   "Path to variables file",
					},
					&cli.StringFlag{
						Name:    "log-level",
						Aliases: []string{"l"},
						Value:   "info",
						Usage:   "Log level (debug, info, error)",
					},
					&cli.StringFlag{
						Name:    "sudo-pass",
						Aliases: []string{"s"},
						Usage:   "Sudo password for steps with become: true (requires --insecure-sudo-pass)",
					},
					&cli.BoolFlag{
						Name:    "ask-become-pass",
						Aliases: []string{"K"},
						Usage:   "Prompt for sudo password interactively (recommended)",
					},
					&cli.StringFlag{
						Name:  "sudo-pass-file",
						Usage: "Read sudo password from file (must have 0600 permissions)",
					},
					&cli.BoolFlag{
						Name:  "insecure-sudo-pass",
						Usage: "Allow --sudo-pass flag (WARNING: password visible in shell history)",
					},
					&cli.StringFlag{
						Name:    "tags",
						Aliases: []string{"t"},
						Usage:   "Filter steps by tags (comma-separated)",
					},
					&cli.BoolFlag{
						Name:    "raw",
						Aliases: []string{"r"},
						Value:   false,
						Usage:   "Disable animated TUI and use raw console output",
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Value: false,
						Usage: "Preview what would be executed without making changes",
					},
					&cli.StringFlag{
						Name:  "from-plan",
						Usage: "Execute from saved plan file (JSON or YAML)",
					},
				},
				Action: run,
			},
			{
				Name:  "plan",
				Usage: "Generate and display execution plan",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "config",
						Aliases:  []string{"c"},
						Required: true,
						Usage:    "Path to configuration file",
					},
					&cli.StringFlag{
						Name:    "vars",
						Aliases: []string{"v"},
						Usage:   "Path to variables file",
					},
					&cli.StringFlag{
						Name:    "tags",
						Aliases: []string{"t"},
						Usage:   "Filter steps by tags (comma-separated)",
					},
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Value:   "text",
						Usage:   "Output format: text, json, or yaml",
					},
					&cli.BoolFlag{
						Name:  "show-origins",
						Value: false,
						Usage: "Show origin file:line:col for each step",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Save plan to file (format determined by extension: .json, .yaml, .yml)",
					},
				},
				Action: planCommand,
			},
			{
				Name:    "explain",
				Aliases: []string{"info"},
				Usage:   "Explain machine state - show system information",
				Action:  explainCommand,
			},
			{
				Name:  "validate",
				Usage: "Validate configuration file without executing",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "config",
						Aliases:  []string{"c"},
						Required: true,
						Usage:    "Path to configuration file",
					},
					&cli.StringFlag{
						Name:    "vars",
						Aliases: []string{"v"},
						Usage:   "Path to variables file",
					},
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Value:   "text",
						Usage:   "Output format: text or json",
					},
				},
				Action: validateCommand,
			},
		},
	}

	return app
}

func main() {
	app := createApp()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
