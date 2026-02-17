// Package main provides the mooncake CLI application.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/agent"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/explain"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
	_ "github.com/alehatsman/mooncake/internal/register" // Register action handlers
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

const (
	outputFormatJSON = "json"
	outputFormatText = "text"
	outputFormatYAML = "yaml"

	// Artifact default limits
	defaultMaxOutputBytes = 1048576 // 1MB
	defaultMaxOutputLines = 1000

	// YAML formatting
	yamlIndentSpaces = 2

	// Exit codes
	exitCodeValidationError = 2 // Configuration validation failed
	exitCodeRuntimeError    = 3 // Runtime error during execution
)

// parseTags parses a comma-separated tag string into a slice of trimmed tags
func parseTags(tagsStr string) []string {
	if tagsStr == "" {
		return nil
	}

	var tags []string
	for _, tag := range strings.Split(tagsStr, ",") {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}


func run(c *cli.Context) error {
	// Check if running from plan
	fromPlan := c.String("from-plan")
	if fromPlan != "" {
		return runFromPlan(c, fromPlan)
	}

	raw := c.Bool("raw")
	dryRun := c.Bool("dry-run")
	logLevel := c.String("log-level")
	outputFormat := c.String("output-format")

	// Force raw mode for dry-run
	if dryRun {
		raw = true
	}

	// Validate output format
	if outputFormat != outputFormatText && outputFormat != outputFormatJSON {
		return fmt.Errorf("invalid output-format: %s (must be 'text' or 'json')", outputFormat)
	}

	// JSON format requires raw mode
	if outputFormat == outputFormatJSON && !raw {
		return fmt.Errorf("--output-format json requires --raw flag")
	}

	// Parse tags from comma-separated string
	tags := parseTags(c.String("tags"))

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

	// Collect facts early if facts-json requested
	factsJSONPath := c.String("facts-json")
	if factsJSONPath != "" {
		systemFacts := facts.Collect()
		if err := writeFactsJSON(systemFacts, factsJSONPath); err != nil {
			log.Printf("Warning: failed to write facts JSON: %v", err)
			// Non-fatal, continue execution
		}
	}

	// Always use event-driven architecture
	// Create event publisher
	publisher := events.NewPublisher()
	defer publisher.Close()

	// Parse log level for subscriber
	level := logger.InfoLevel
	switch logLevel {
	case "debug":
		level = logger.DebugLevel
	case "error":
		level = logger.ErrorLevel
	}

	// Create appropriate subscriber based on mode
	if !raw && logger.IsTUISupported() {
		// Use TUI subscriber for animated display
		tuiSubscriber, err := logger.NewTUISubscriber(level)
		if err != nil {
			// Fallback to console subscriber if TUI initialization fails
			subscriber := logger.NewConsoleSubscriber(level, outputFormat)
			publisher.Subscribe(subscriber)
		} else {
			tuiSubscriber.Start()
			defer tuiSubscriber.Stop()
			publisher.Subscribe(tuiSubscriber)
		}
	} else {
		// Use console subscriber for raw/JSON output
		subscriber := logger.NewConsoleSubscriber(level, outputFormat)
		publisher.Subscribe(subscriber)
	}

	// Create a minimal logger for internal use (errors, etc.)
	internalLog := logger.NewLogger(level)

	// Execute with event publisher
	return executor.Start(executor.StartConfig{
		ConfigFilePath:   c.String("config"),
		VarsFilePath:     c.String("vars"),
		SudoPass:         c.String("sudo-pass"),
		SudoPassFile:     c.String("sudo-pass-file"),
		AskBecomePass:    c.Bool("ask-become-pass"),
		InsecureSudoPass: c.Bool("insecure-sudo-pass"),
		Tags:             tags,
		DryRun:           dryRun,

		// Artifact configuration
		ArtifactsDir:      c.String("artifacts-dir"),
		CaptureFullOutput: c.Bool("capture-full-output"),
		MaxOutputBytes:    c.Int("max-output-bytes"),
		MaxOutputLines:    c.Int("max-output-lines"),
	}, internalLog, publisher)
}

func runFromPlan(c *cli.Context, planPath string) error {
	// Load plan from file
	planData, err := plan.LoadPlanFromFile(planPath)
	if err != nil {
		return fmt.Errorf("failed to load plan: %w", err)
	}

	// Setup logger
	dryRun := c.Bool("dry-run")
	logLevel := c.String("log-level")

	// Always use event-driven architecture
	publisher := events.NewPublisher()
	defer publisher.Close()

	// Parse log level
	level := logger.InfoLevel
	switch logLevel {
	case "debug":
		level = logger.DebugLevel
	case "error":
		level = logger.ErrorLevel
	}

	// Create console subscriber for text output
	subscriber := logger.NewConsoleSubscriber(level, outputFormatText)
	publisher.Subscribe(subscriber)

	// Create minimal logger for internal use
	internalLog := logger.NewLogger(level)

	// Execute plan with event publisher
	return executor.ExecutePlan(planData, c.String("sudo-pass"), dryRun, internalLog, publisher)
}

func factsCommand(c *cli.Context) error {
	format := c.String("format")

	// Validate format
	if format != outputFormatText && format != outputFormatJSON {
		return fmt.Errorf("invalid format: %s (use 'text' or 'json')", format)
	}

	// Collect facts
	f := facts.Collect()

	// Output based on format
	switch format {
	case outputFormatJSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(f)
	case outputFormatText:
		explain.DisplayFacts(f)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// actionsListCommand lists all registered actions with their platform support.
func actionsListCommand(c *cli.Context) error {
	format := c.String("format")

	// Validate format
	if format != outputFormatText && format != outputFormatJSON {
		return fmt.Errorf("invalid format: %s (use 'text' or 'json')", format)
	}

	// Get all registered actions
	actionsList := actions.List()

	// Sort by name for consistent output
	sort.Slice(actionsList, func(i, j int) bool {
		return actionsList[i].Name < actionsList[j].Name
	})

	// Output based on format
	switch format {
	case outputFormatJSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(actionsList)
	case outputFormatText:
		displayActionsTable(actionsList)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// displayActionsTable displays actions in a formatted table.
func displayActionsTable(actionsList []actions.ActionMetadata) {
	// Print header
	fmt.Printf("%-15s %-10s %-25s %-8s %-8s\n",
		"ACTION", "CATEGORY", "PLATFORMS", "SUDO", "CHECK")
	fmt.Println(strings.Repeat("-", 80))

	// Print each action
	for _, meta := range actionsList {
		// Format platforms
		platforms := "all"
		if len(meta.SupportedPlatforms) > 0 {
			platforms = strings.Join(meta.SupportedPlatforms, ",")
			if len(platforms) > 23 {
				platforms = platforms[:20] + "..."
			}
		}

		// Format sudo requirement
		sudo := "no"
		if meta.RequiresSudo {
			sudo = "yes" //nolint:goconst // Simple display string
		}

		// Format check implementation
		check := "no"
		if meta.ImplementsCheck {
			check = "yes"
		}

		fmt.Printf("%-15s %-10s %-25s %-8s %-8s\n",
			meta.Name,
			meta.Category,
			platforms,
			sudo,
			check)
	}
}

func writeFactsJSON(f *facts.Facts, path string) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal facts: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func planCommand(c *cli.Context) error {
	configPath := c.String("config")
	varsPath := c.String("vars")
	outputPath := c.String("output")
	format := c.String("format")
	showOrigins := c.Bool("show-origins")

	// Parse tags
	tags := parseTags(c.String("tags"))

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
	planner, err := plan.NewPlanner()
	if err != nil {
		return err
	}
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
	case outputFormatJSON:
		return formatPlanJSON(planData)
	case outputFormatYAML:
		return formatPlanYAML(planData)
	case outputFormatText:
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
	encoder.SetIndent(yamlIndentSpaces)
	defer func() {
		// Intentionally ignore Close() error - encoder writes to stdout which doesn't need explicit close handling
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

func agentRunCommand(c *cli.Context) error {
	goal := c.String("goal")
	planPath := c.String("plan")
	useStdin := c.Bool("stdin")
	provider := c.String("provider")
	model := c.String("model")
	maxIterations := c.Int("max-iterations")

	if goal == "" {
		return fmt.Errorf("--goal is required")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	opts := agent.RunOptions{
		Goal:          goal,
		PlanPath:      planPath,
		UseStdin:      useStdin,
		RepoRoot:      repoRoot,
		Provider:      provider,
		Model:         model,
		MaxIterations: maxIterations,
	}

	if provider == "claude" {
		result, err := agent.RunLoop(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Agent loop failed: %v\n", err)
			if result != nil && result.FinalLog != nil {
				printAgentSummary(result.FinalLog)
			}
			return err
		}

		fmt.Printf("Agent completed: %d iterations\n", len(result.Iterations))
		fmt.Printf("Stop reason: %s\n", result.StopReason)
		if result.FinalLog != nil {
			fmt.Println()
			printAgentSummary(result.FinalLog)
		}
		return nil
	}

	if planPath == "" && !useStdin {
		return fmt.Errorf("either --plan or --stdin must be specified (or use --provider=claude for loop mode)")
	}

	if planPath != "" && useStdin {
		return fmt.Errorf("cannot specify both --plan and --stdin")
	}

	if planPath != "" && !filepath.IsAbs(planPath) {
		planPath = filepath.Join(repoRoot, planPath)
	}

	opts.PlanPath = planPath

	log, err := agent.Run(opts)
	if err != nil {
		return err
	}

	printAgentSummary(log)
	return nil
}

func printAgentSummary(log *agent.IterationLog) {
	fmt.Printf("Iteration: %d\n", log.Iteration)
	fmt.Printf("Status: %s\n", log.Status)
	fmt.Printf("Files touched: %d\n", log.DiffStat.Files)
	fmt.Printf("Insertions: +%d\n", log.DiffStat.Insertions)
	fmt.Printf("Deletions: -%d\n", log.DiffStat.Deletions)

	if len(log.ChangedFiles) > 0 {
		fmt.Println("\nChanged files:")
		for _, file := range log.ChangedFiles {
			fmt.Printf("  %s\n", file)
		}
	}

	if len(log.Artifacts) > 0 {
		fmt.Println("\nArtifacts:")
		for _, artifact := range log.Artifacts {
			fmt.Printf("  %s\n", artifact)
		}
	}
}

func validateCommand(c *cli.Context) error {
	configPath := c.String("config")
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

func createApp() *cli.App {
	app := &cli.App{
		Name:                 "mooncake",
		Usage:                "Space fighters provisioning tool, Chookity!",
		EnableBashCompletion: true,

		Commands: []*cli.Command{
			presetsCommand(),
			docsCommand(),
			schemaCommand(),
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
						Name:  "output-format",
						Value: "text",
						Usage: "Output format: text or json (requires --raw)",
					},
					&cli.StringFlag{
						Name:  "artifacts-dir",
						Value: "",
						Usage: "Directory to store run artifacts (e.g., .mooncake)",
					},
					&cli.BoolFlag{
						Name:  "capture-full-output",
						Value: false,
						Usage: "Capture full stdout/stderr to artifacts (requires --artifacts-dir)",
					},
					&cli.IntFlag{
						Name:  "max-output-bytes",
						Value: defaultMaxOutputBytes,
						Usage: "Max bytes of output per step in results.json",
					},
					&cli.IntFlag{
						Name:  "max-output-lines",
						Value: defaultMaxOutputLines,
						Usage: "Max lines of output per step in results.json",
					},
					&cli.StringFlag{
						Name:  "from-plan",
						Usage: "Execute from saved plan file (JSON or YAML)",
					},
					&cli.StringFlag{
						Name:  "facts-json",
						Usage: "Path to write collected facts as JSON",
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
				Name:  "facts",
				Usage: "Display system facts",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Value:   "text",
						Usage:   "Output format: text or json",
					},
				},
				Action: factsCommand,
			},
			{
				Name:  "actions",
				Usage: "Manage and inspect actions",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List all available actions with platform support",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "format",
								Aliases: []string{"f"},
								Value:   "text",
								Usage:   "Output format: text or json",
							},
						},
						Action: actionsListCommand,
					},
				},
			},
			{
				Name:  "agent",
				Usage: "Agent operations",
				Subcommands: []*cli.Command{
					{
						Name:  "run",
						Usage: "Execute agent iteration",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "goal",
								Aliases:  []string{"g"},
								Required: true,
								Usage:    "Goal description",
							},
							&cli.StringFlag{
								Name:    "plan",
								Aliases: []string{"p"},
								Usage:   "Path to plan YAML file",
							},
							&cli.BoolFlag{
								Name:  "stdin",
								Usage: "Read plan from stdin",
							},
							&cli.StringFlag{
								Name:  "provider",
								Usage: "LLM provider (claude for loop mode)",
							},
							&cli.StringFlag{
								Name:  "model",
								Value: "sonnet",
								Usage: "Model name (when using --provider)",
							},
							&cli.IntFlag{
								Name:  "max-iterations",
								Value: 5,
								Usage: "Maximum iterations for loop mode",
							},
						},
						Action: agentRunCommand,
					},
				},
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
