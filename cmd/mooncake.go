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

var version = "dev"

const (
	outputFormatJSON  = "json"
	outputFormatText  = "text"
	outputFormatYAML  = "yaml"
	outputFormatAgent = "agent"
	outputFormatQuiet = "quiet"

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


// applyFlags returns the canonical flag set shared by `apply` and the
// deprecated `run` alias. Centralized so both commands stay in sync.
func applyFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Path to configuration file",
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
		&cli.StringFlag{Name: "sudo-pass-file", Usage: "Read sudo password from file (must have 0600 permissions)"},
		&cli.BoolFlag{Name: "insecure-sudo-pass", Usage: "Allow --sudo-pass flag (WARNING: password visible in shell history)"},
		&cli.StringFlag{
			Name:    "tags",
			Aliases: []string{"t"},
			Usage:   "Filter steps by tags (comma-separated)",
		},
		&cli.BoolFlag{
			Name:  "tui",
			Value: false,
			Usage: "Use the animated TUI subscriber (default: raw console output)",
		},
		&cli.StringFlag{Name: "output-format", Value: "text", Usage: "Output format: text or json (json requires not using --tui)"},
		&cli.StringFlag{Name: "artifacts-dir", Value: "", Usage: "Directory to store run artifacts (e.g., .mooncake)"},
		&cli.BoolFlag{Name: "capture-full-output", Value: false, Usage: "Capture full stdout/stderr to artifacts (requires --artifacts-dir)"},
		&cli.IntFlag{Name: "max-output-bytes", Value: defaultMaxOutputBytes, Usage: "Max bytes of output per step in results.json"},
		&cli.IntFlag{Name: "max-output-lines", Value: defaultMaxOutputLines, Usage: "Max lines of output per step in results.json"},
		&cli.StringFlag{Name: "from-plan", Usage: "Apply from saved plan file (JSON or YAML)"},
		&cli.StringFlag{Name: "facts-json", Usage: "Path to write collected facts as JSON"},

		// Spec 16 stale-plan policy (only meaningful with --from-plan).
		&cli.BoolFlag{Name: "allow-stale", Value: false, Usage: "Apply a saved plan even if host facts mismatch or input files have changed since plan time"},
		&cli.DurationFlag{Name: "max-plan-age", Value: 0, Usage: "Refuse to apply a saved plan older than this duration (e.g. 1h). Default: no limit."},
	}
}

func run(c *cli.Context) error {
	// Check if running from plan
	fromPlan := c.String("from-plan")
	if fromPlan != "" {
		return runFromPlan(c, fromPlan)
	}

	tui := c.Bool("tui")
	logLevel := c.String("log-level")
	outputFormat := c.String("output-format")

	// Validate output format
	if outputFormat != outputFormatText && outputFormat != outputFormatJSON && outputFormat != outputFormatAgent && outputFormat != outputFormatQuiet {
		return fmt.Errorf("invalid output-format: %s (must be 'text', 'json', 'agent', or 'quiet')", outputFormat)
	}

	// JSON format requires raw mode
	if outputFormat == outputFormatJSON && tui {
		return fmt.Errorf("--output-format json cannot be combined with --tui")
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

	// Always emit structured JSON errors to stderr on step failures.
	publisher.Subscribe(logger.NewStderrErrorSubscriber())

	// Always record run history (best-effort).
	publisher.Subscribe(logger.NewRunLogSubscriber(c.String("config")))

	// Create appropriate subscriber based on mode
	if outputFormat == outputFormatAgent {
		publisher.Subscribe(logger.NewAgentSubscriber())
	} else if outputFormat == outputFormatQuiet {
		publisher.Subscribe(logger.NewQuietSubscriber())
	} else if tui && logger.IsTUISupported() {
		// Use TUI subscriber when explicitly requested.
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

	// Spec 16 stale-plan policy: refuse to apply a plan that was built
	// for a different host, against source files that have changed, or
	// older than --max-plan-age. --allow-stale demotes all rejections
	// to warnings.
	validateOpts := plan.ValidateOptions{
		MaxAge:     c.Duration("max-plan-age"),
		AllowStale: c.Bool("allow-stale"),
	}
	if err := plan.ValidateForApply(planData, validateOpts); err != nil {
		return fmt.Errorf("refusing to apply stale plan: %w\n\nUse --allow-stale to override.", err)
	}

	// Setup logger
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
	publisher.Subscribe(logger.NewRunLogSubscriber(planPath))

	// Create minimal logger for internal use
	internalLog := logger.NewLogger(level)

	// Execute plan with event publisher
	return executor.ExecutePlan(planData, c.String("sudo-pass"), actions.ModeApply, internalLog, publisher)
}

func factsCommand(c *cli.Context) error {
	// Collect facts (cached)
	f := facts.Collect()

	// --query mode: print specific values and exit
	if queries := c.StringSlice("query"); len(queries) > 0 {
		return queryMap(f.ToMap(), queries)
	}

	format := c.String("format")
	if format != outputFormatText && format != outputFormatJSON {
		return fmt.Errorf("invalid format: %s (use 'text' or 'json')", format)
	}

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
	noInspect := c.Bool("no-inspect")

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

	// Spec 16: after the static config expansion, inspect each step
	// against current target state. Unless --no-inspect is set.
	// Inspection routes through the executor in check mode; legacy
	// handlers that don't implement Spec-16 Runner report as
	// "not checkable" until they migrate (Phase 5).
	if !noInspect {
		internalLog := logger.NewLogger(logger.ErrorLevel)
		inspections, err := executor.InspectPlan(planData, "", internalLog)
		if err != nil {
			return fmt.Errorf("failed to inspect plan: %w", err)
		}
		planData.Inspections = inspections
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

// planSymbol returns the leading symbol for a per-step plan line.
//
//	↑  WouldChange  — applying this step would change state
//	✓  AlreadyOk    — already in desired state
//	-  Skipped      — when / tag filter removed this step
//	?  not checkable — handler can't predict (e.g. shell)
func planSymbol(ins plan.StepInspection, stepSkipped bool) string {
	switch {
	case stepSkipped || ins.Skipped:
		return "-"
	case !ins.Checkable:
		return "?"
	case ins.WouldChange:
		return "↑"
	default:
		return "✓"
	}
}

func formatPlanText(p *plan.Plan, showOrigins bool) error {
	fmt.Printf("Plan: %s\n", p.RootFile)
	hostBits := []string{}
	if p.GeneratedOn.OsFamily != "" {
		hostBits = append(hostBits, p.GeneratedOn.OsFamily)
	}
	if p.GeneratedOn.Arch != "" {
		hostBits = append(hostBits, p.GeneratedOn.Arch)
	}
	if p.GeneratedOn.DistroFamily != "" {
		hostBits = append(hostBits, p.GeneratedOn.DistroFamily)
	}
	if len(hostBits) > 0 {
		fmt.Printf("Generated: %s on %s\n", p.GeneratedAt.Format("2006-01-02 15:04:05"), strings.Join(hostBits, "/"))
	} else {
		fmt.Printf("Generated: %s\n", p.GeneratedAt.Format("2006-01-02 15:04:05"))
	}
	if len(p.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(p.Tags, ", "))
	}
	fmt.Println()

	// Build a lookup from stepID to inspection so the order matches steps.
	insByID := make(map[string]plan.StepInspection, len(p.Inspections))
	for _, ins := range p.Inspections {
		insByID[ins.StepID] = ins
	}

	var wouldChange, ok, skipped, notCheckable int
	for _, step := range p.Steps {
		ins := insByID[step.ID]
		sym := planSymbol(ins, step.Skipped)

		name := step.Name
		if name == "" {
			name = step.ID
		}
		// One line per step: <sym> <name>   <reason>
		line := fmt.Sprintf("%s %s", sym, name)
		if ins.Reason != "" {
			line = fmt.Sprintf("%-50s  %s", line, ins.Reason)
		} else if step.Skipped {
			line = fmt.Sprintf("%-50s  %s", line, "skipped (tags)")
		}
		fmt.Println(line)

		switch sym {
		case "↑":
			wouldChange++
		case "✓":
			ok++
		case "-":
			skipped++
		case "?":
			notCheckable++
		}

		if showOrigins && step.Origin != nil {
			fmt.Printf("    %s:%d:%d\n", step.Origin.FilePath, step.Origin.Line, step.Origin.Column)
			if len(step.Origin.IncludeChain) > 0 {
				fmt.Printf("    via: %s\n", strings.Join(step.Origin.IncludeChain, " -> "))
			}
		}
	}

	fmt.Println()
	fmt.Printf("PLAN SUMMARY  would-change=%d  ok=%d  skipped=%d  not-checkable=%d\n",
		wouldChange, ok, skipped, notCheckable)
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
		result, loopErr := agent.RunLoop(opts)
		if loopErr != nil {
			fmt.Fprintf(os.Stderr, "Agent loop failed: %v\n", loopErr)
			if result != nil && result.FinalLog != nil {
				printAgentSummary(result.FinalLog)
			}
			return loopErr
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
		Version:              version,
		EnableBashCompletion: true,

		Commands: []*cli.Command{
			presetsCommand(),
			docsCommand(),
			schemaCommand(),
			snapshotCommand(),
			lastCommand(),
			mcpCommand(),
			agentdCommand(),
			stepCommand(),
			toolCommand(),
			{
				Name:   "apply",
				Usage:  "Apply a playbook or saved plan to the system",
				Flags:  applyFlags(),
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
					&cli.BoolFlag{
						Name:  "no-inspect",
						Value: false,
						Usage: "Skip the per-step state inspection pass (Spec 16). With this flag, plan output reflects only static YAML expansion — no would-change predictions.",
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
					&cli.StringSliceFlag{
						Name:    "query",
						Aliases: []string{"q"},
						Usage:   "Query a specific fact by dot-path key (e.g. go.version). Repeatable.",
					},
				},
				Action: factsCommand,
			},
			{
				Name:  "metrics",
				Usage: "Display live system metrics (CPU/GPU/memory/load/network)",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Value:   "text",
						Usage:   "Output format: text or json",
					},
					&cli.StringSliceFlag{
						Name:    "query",
						Aliases: []string{"q"},
						Usage:   "Query a specific metric by key (e.g. cpu_usage_pct). Repeatable.",
					},
					&cli.StringSliceFlag{
						Name:  "fields",
						Usage: "Restrict output to these keys. Repeatable or comma-separated. Adds a _collected_at sibling map.",
					},
					&cli.BoolFlag{
						Name:  "refresh",
						Usage: "Force re-sample, bypassing TTL",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Save metrics to file (JSON)",
					},
				},
				Action: metricsCommand,
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
