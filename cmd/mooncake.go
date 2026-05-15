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
	"github.com/alehatsman/mooncake/internal/effects"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/explain"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/fleet"
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


// hostnameForLocalOverlays returns os.Hostname() with the first DNS label
// only. macOS reports "MacBook-Air.local"; this trims to "MacBook-Air" so
// the corresponding overlay file an operator would commit is
// vars/by-host/MacBook-Air.yml. No-op on systems that already return a bare
// label.
func hostnameForLocalOverlays() (string, error) {
	h, err := os.Hostname()
	if err != nil {
		return "", err
	}
	if i := strings.Index(h, "."); i >= 0 {
		h = h[:i]
	}
	return h, nil
}

// resolveLocalOverlays returns the auto-loaded overlay vars-files for a
// local `mooncake apply` (or `plan`) run, honoring spec-51's --host,
// $MOONCAKE_HOST, and --overlays=off. The result is meant to be prepended
// to any explicit --vars-file args so user flags still win on collision.
//
// Hostname source order:
//
//  1. --host <name> flag       (explicit; missing by-host file → error)
//  2. $MOONCAKE_HOST env var   (explicit; missing by-host file → error)
//  3. os.Hostname() first-label (implicit; missing by-host file → silent)
//
// configPath is the resolved config file; the plan-dir for overlay lookup
// is its directory.
func resolveLocalOverlays(c *cli.Context, configPath string) ([]string, error) {
	switch c.String("overlays") {
	case "", "on":
		// proceed
	case "off":
		return nil, nil
	default:
		return nil, fmt.Errorf("--overlays must be 'on' or 'off', got %q", c.String("overlays"))
	}

	hostname := c.String("host")
	hostExplicit := hostname != ""
	if hostname == "" {
		if env := os.Getenv("MOONCAKE_HOST"); env != "" {
			hostname = env
			hostExplicit = true
		}
	}
	if hostname == "" {
		derived, err := hostnameForLocalOverlays()
		if err != nil {
			return nil, fmt.Errorf("failed to derive hostname for overlay auto-load: %w", err)
		}
		hostname = derived
	}

	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}
	planDir := filepath.Dir(absConfig)

	overlays := fleet.ResolveLocalOverlays(planDir, hostname)

	if hostExplicit {
		// Operator named a host specifically. A missing by-host file likely
		// means a typo or a not-yet-created overlay — surface it rather than
		// silently producing an un-overlaid plan.
		wantHost := filepath.Clean(filepath.Join(planDir, "vars", "by-host", hostname+".yml"))
		found := false
		for _, p := range overlays {
			if p == wantHost {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("overlay file not found for host %q: %s does not exist", hostname, wantHost)
		}
	}

	return overlays, nil
}

// applyFlags returns the canonical flag set shared by `apply` and the
// deprecated `run` alias. Centralized so both commands stay in sync.
func applyFlags() []cli.Flag {
	return []cli.Flag{
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
		&cli.StringFlag{
			Name:  "skip-tags",
			Usage: "Exclude steps whose tags appear in this list (comma-separated). Composes with --tags via AND.",
		},
		&cli.BoolFlag{
			Name:    "dry-run",
			Aliases: []string{"n"},
			Usage:   "Preview changes without executing (sugar for `mooncake plan`)",
		},
		&cli.BoolFlag{
			Name:  "tui",
			Value: false,
			Usage: "Use the animated TUI subscriber (default: raw console output)",
		},
		&cli.StringFlag{
			Name:    "output-format",
			Aliases: []string{"format"}, // MT-68: every other command uses --format; accept the shorter name here too
			Value:   "text",
			Usage:   "Output format: text or json (json requires not using --tui)",
		},
		&cli.StringFlag{Name: "artifacts-dir", Value: "", Usage: "Directory to store run artifacts (e.g., .mooncake)"},
		&cli.BoolFlag{Name: "capture-full-output", Value: false, Usage: "Capture full stdout/stderr to artifacts (requires --artifacts-dir)"},
		&cli.IntFlag{Name: "max-output-bytes", Value: defaultMaxOutputBytes, Usage: "Max bytes of step output captured to the artifacts bundle (stdout.log/stderr.log)"},
		&cli.IntFlag{Name: "max-output-lines", Value: defaultMaxOutputLines, Usage: "Max lines of step output captured to the artifacts bundle (stdout.log/stderr.log)"},
		&cli.StringFlag{Name: "from-plan", Usage: "Apply from saved plan file (JSON or YAML)"},
		&cli.StringFlag{Name: "facts-json", Usage: "Path to write collected facts as JSON"},

		// Spec 51: local overlay auto-load. Pulls vars/common.yml and
		// vars/by-host/<host>.yml (relative to the config's directory) into
		// the vars list, so the spec-48 overlay convention works the same
		// on `mooncake apply` as it does on `mooncake fleet apply`.
		&cli.StringFlag{
			Name:  "host",
			Usage: "Override the auto-detected hostname for overlay lookup (vars/by-host/<host>.yml). Also honors $MOONCAKE_HOST. An explicit name whose by-host file is missing is an error.",
		},
		&cli.StringFlag{
			Name:  "overlays",
			Value: "on",
			Usage: "Local overlay auto-load: 'on' (default) loads vars/common.yml and vars/by-host/<host>.yml; 'off' disables.",
		},

		// Spec 16 stale-plan policy (only meaningful with --from-plan).
		&cli.BoolFlag{Name: "allow-stale", Value: false, Usage: "Apply a saved plan even if host facts mismatch or input files have changed since plan time"},
		&cli.DurationFlag{Name: "max-plan-age", Value: 0, Usage: "Refuse to apply a saved plan older than this duration (e.g. 1h). Default: no limit."},
	}
}

func run(c *cli.Context) error {
	// MT-76: --capture-full-output is a no-op without --artifacts-dir
	// because there's no bundle to write to. The flag's help text says
	// "requires --artifacts-dir"; honor that instead of silently
	// dropping the capture and leaving the user wondering where their
	// logs went. Check before --dry-run / --from-plan so the validation
	// fires consistently regardless of mode.
	if c.Bool("capture-full-output") && c.String("artifacts-dir") == "" {
		return fmt.Errorf("--capture-full-output requires --artifacts-dir (the captured logs need a directory to land in)")
	}

	// MT-86: --max-output-bytes / --max-output-lines only affect the
	// artifact bundle (stdout.log / stderr.log). Without --artifacts-dir
	// they're silently ignored — and the truncation never reaches the
	// step.completed JSON either. Same shape as MT-76: hard-error when
	// the user explicitly sets either flag without the partner.
	// c.IsSet skips the case where the default-valued flag is silently
	// in scope; we only complain about a user-supplied override.
	if c.String("artifacts-dir") == "" {
		if c.IsSet("max-output-bytes") {
			return fmt.Errorf("--max-output-bytes requires --artifacts-dir (the truncation limit only applies to the artifact bundle's stdout.log/stderr.log)")
		}
		if c.IsSet("max-output-lines") {
			return fmt.Errorf("--max-output-lines requires --artifacts-dir (the truncation limit only applies to the artifact bundle's stdout.log/stderr.log)")
		}
	}

	// --dry-run short-circuits to the plan path. Sugar over `mooncake plan`.
	if c.Bool("dry-run") {
		if c.String("from-plan") != "" {
			return fmt.Errorf("--dry-run is incompatible with --from-plan: the plan was already produced; just run `mooncake apply --from-plan <file>` to apply it, or `mooncake plan -c <config>` to re-preview")
		}
		return planCommand(c)
	}

	// Check if running from plan
	fromPlan := c.String("from-plan")
	if fromPlan != "" {
		return runFromPlan(c, fromPlan)
	}

	// Resolve config path: explicit --config wins, else auto-discover.
	configPath, err := resolveConfigPath(c)
	if err != nil {
		if printNoConfigHintAndExit(err, "apply") {
			return nil
		}
		return err
	}

	// Spec 51: auto-load local overlays (vars/common.yml + vars/by-host/
	// <host>.yml under the config's directory) and prepend to --vars so
	// explicit -v args still win on key collision.
	overlayVars, err := resolveLocalOverlays(c, configPath)
	if err != nil {
		return err
	}
	explicitVars := c.StringSlice("vars")
	resolvedVars := make([]string, 0, len(overlayVars)+len(explicitVars))
	resolvedVars = append(resolvedVars, overlayVars...)
	resolvedVars = append(resolvedVars, explicitVars...)

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
	skipTags := parseTags(c.String("skip-tags"))

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
	publisher.Subscribe(logger.NewRunLogSubscriber(configPath))

	// One-shot next-step hint after the first successful run on this host.
	// The subscriber is self-suppressing for non-text formats and respects
	// MOONCAKE_NO_HINTS=1.
	publisher.Subscribe(logger.NewFirstRunHintSubscriber(os.Stdout, outputFormat))

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
		ConfigFilePath:   configPath,
		VarsFilePaths:    resolvedVars,
		SudoPass:         c.String("sudo-pass"),
		SudoPassFile:     c.String("sudo-pass-file"),
		AskBecomePass:    c.Bool("ask-become-pass"),
		InsecureSudoPass: c.Bool("insecure-sudo-pass"),
		Tags:             tags,
		SkipTags:         skipTags,

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
		return fmt.Errorf("refusing to apply stale plan: %w (use --allow-stale to override)", err)
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
	// MT-74: marshal via ToMap() so keys are snake_case, matching the
	// daemon's /v1/facts endpoint and the template scope (`{{ os }}`).
	// Direct json.Marshal(*Facts) would emit PascalCase Go field names.
	data, err := json.MarshalIndent(f.ToMap(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal facts: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func planCommand(c *cli.Context) error {
	configPath, err := resolveConfigPath(c)
	if err != nil {
		if printNoConfigHintAndExit(err, "plan") {
			return nil
		}
		return err
	}
	// Spec 51: prepend local overlay vars. For standalone `plan`, --host
	// and --overlays aren't registered as flags; the helper falls through
	// to the derived hostname and the default on-state, so behavior matches
	// `apply --dry-run` reached via the apply flag set.
	overlayVars, err := resolveLocalOverlays(c, configPath)
	if err != nil {
		return err
	}
	explicitVars := c.StringSlice("vars")
	varsPaths := make([]string, 0, len(overlayVars)+len(explicitVars))
	varsPaths = append(varsPaths, overlayVars...)
	varsPaths = append(varsPaths, explicitVars...)
	outputPath := c.String("output")
	// When invoked via `apply --dry-run`, the plan-specific --format flag
	// isn't registered on apply; fall back to the same default as `plan`.
	format := c.String("format")
	if format == "" {
		format = outputFormatText
	}
	showOrigins := c.Bool("show-origins")
	showDiff := c.Bool("diff")
	noInspect := c.Bool("no-inspect")

	// Parse tags
	tags := parseTags(c.String("tags"))
	skipTags := parseTags(c.String("skip-tags"))

	// Load variables from each file in order; later files override earlier
	// on key collision. Matches `apply -v a.yml -v b.yml` semantics.
	variables := make(map[string]interface{})
	for _, varsPath := range varsPaths {
		if varsPath == "" {
			continue
		}
		vars, err := config.ReadVariables(varsPath)
		if err != nil {
			return fmt.Errorf("failed to read variables from %s: %w", varsPath, err)
		}
		for k, v := range vars {
			variables[k] = v
		}
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
		SkipTags:   skipTags,
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
		return formatPlanText(planData, showOrigins, showDiff)
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

func formatPlanText(p *plan.Plan, showOrigins bool, showDiff bool) error {
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

	var wouldChange, ok, skipped, notCheckable, maxRisk int
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

		// Spec-22 phase 6: show a one-line cost summary under any
		// step that would change. Suppressed for ok/skipped to keep
		// "nothing changed" plans uncluttered.
		if sym == "↑" && ins.Cost != nil {
			fmt.Printf("    cost: %s\n", formatCostSummary(ins.Cost))
			if ins.Cost.Risk > maxRisk {
				maxRisk = ins.Cost.Risk
			}
		}

		if showDiff && sym == "↑" {
			if udiff := extractUnifiedDiff(ins.Detail); udiff != "" {
				for _, dl := range strings.Split(strings.TrimRight(udiff, "\n"), "\n") {
					fmt.Printf("  %s\n", dl)
				}
				fmt.Println()
			}
		}

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
	summary := fmt.Sprintf("PLAN SUMMARY  would-change=%d  ok=%d  skipped=%d  not-checkable=%d",
		wouldChange, ok, skipped, notCheckable)
	if maxRisk > 0 {
		summary += fmt.Sprintf("  max-risk=%d (%s)", maxRisk, riskBand(maxRisk))
	}
	fmt.Println(summary)
	return nil
}

// formatCostSummary renders an actions.CostEstimate as a single
// human-readable line for plan output. Fields that are -1 (unknown
// — handler couldn't predict statically) are omitted rather than
// shown as "-1" to keep the line scannable. Spec-22 phase 6.
func formatCostSummary(c *actions.CostEstimate) string {
	parts := []string{fmt.Sprintf("risk %d (%s)", c.Risk, riskBand(c.Risk))}
	if c.Reversible {
		parts = append(parts, "reversible")
	} else {
		parts = append(parts, "irreversible")
	}
	if c.Resources >= 0 {
		unit := "resource"
		if c.Resources != 1 {
			unit = "resources"
		}
		parts = append(parts, fmt.Sprintf("%d %s", c.Resources, unit))
	}
	if c.Bytes >= 0 {
		parts = append(parts, fmt.Sprintf("%d bytes", c.Bytes))
	}
	return strings.Join(parts, " • ")
}

// riskBand maps a numeric risk score (1..10) to the band label
// the spec-22 phase 6 contract documents:
//   1–3   safe (read-only, idempotent writes to scratch)
//   4–6   routine (config writes, package installs)
//   7–9   high impact (service restarts, kernel params)
//   10    destructive (deletes, drops, rm -rf)
func riskBand(r int) string {
	switch {
	case r >= 10:
		return "destructive"
	case r >= 7:
		return "high"
	case r >= 4:
		return "routine"
	case r >= 1:
		return "safe"
	}
	return "unknown"
}

// extractUnifiedDiff pulls the unified diff string out of a StepInspection.Detail
// value, which may be an effects.ContentDiff (in-memory) or map[string]interface{}
// (decoded from saved JSON plan).
func extractUnifiedDiff(detail any) string {
	switch v := detail.(type) {
	case effects.ContentDiff:
		return v.UnifiedDiff
	case map[string]interface{}:
		s, _ := v["unified_diff"].(string)
		return s
	}
	return ""
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
		// MT-57: when a user types `mooncake runs submit` (real name is
		// `apply`), urfave/cli's default fallthrough is `No help topic
		// for 'submit'` — no hint that they're close. Enabling Suggest
		// adds the standard "Did you mean 'apply'?" line.
		Suggest: true,

		Commands: []*cli.Command{
			initCommand(),
			doctorCommand(),
			presetsCommand(),
			docsCommand(),
			schemaCommand(),
			snapshotCommand(),
			historyCommand(),
			mcpCommand(),
			agentdCommand(),
			fleetCommand(),
			stepCommand(),
			toolCommand(),
			queryCommand(),
			{
				Name:   "apply",
				Usage:  "Apply a playbook or saved plan. Use --dry-run to preview without changes.",
				Flags:  applyFlags(),
				Action: run,
			},
			{
				Name:  "plan",
				Usage: "Generate and display execution plan (dry-run)",
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
						Name:    "tags",
						Aliases: []string{"t"},
						Usage:   "Filter steps by tags (comma-separated)",
					},
					&cli.StringFlag{
						Name:  "skip-tags",
						Usage: "Exclude steps whose tags appear in this list (comma-separated)",
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
					&cli.BoolFlag{
						Name:    "diff",
						Aliases: []string{"d"},
						Value:   false,
						Usage:   "Show unified diff for file steps that would change content",
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
				Name:  "runs",
				Usage: "Submit and follow runs on the local agentd daemon",
				Subcommands: []*cli.Command{
					{
						Name:  "apply",
						Usage: "Submit a config to agentd and stream events back",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Usage: "Path to configuration file (default: ./mooncake.yml or ./mooncake/main.yml)"},
							&cli.StringSliceFlag{Name: "vars", Aliases: []string{"v"}, Usage: "Path to a variables file. Repeat to layer."},
							&cli.StringFlag{Name: "tags", Aliases: []string{"t"}, Usage: "Filter steps by tags (comma-separated)"},
							&cli.StringFlag{Name: "base-dir", Usage: "Base directory the daemon should chdir into (default: dirname of --config)"},
							&cli.StringFlag{Name: "goal", Aliases: []string{"g"}, Usage: "Free-text goal recorded with the run"},
							&cli.BoolFlag{Name: "system", Usage: "Use the system-mode agentd socket (/run/mooncake/agentd.sock)"},
						},
						Action: runsApplyCommand,
					},
					{
						Name:  "follow",
						Usage: "Stream events for an existing run, rendered like `apply`",
						Flags: []cli.Flag{
							&cli.BoolFlag{Name: "system", Usage: "Use the system-mode agentd socket"},
						},
						Action: runsFollowCommand,
					},
					{
						Name:   "get",
						Usage:  "Print the JSON record for one run",
						Flags:  []cli.Flag{&cli.BoolFlag{Name: "system"}},
						Action: runsGetCommand,
					},
					{
						Name:  "list",
						Usage: "List runs known to the daemon",
						Flags: []cli.Flag{
							&cli.BoolFlag{Name: "system"},
							// Daemon already returns JSON natively; accept the
							// flag for symmetry with metrics / snapshot / facts
							// so `--format json` doesn't error (#56).
							&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Value: "json", Usage: "Output format (currently only 'json' is supported)"},
						},
						Action: runsListCommand,
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
				Action: validateCommand,
			},
		},
	}

	// MT-66: by default urfave/cli prints the full help dump on a flag
	// parse error (e.g. `mooncake apply --max-plan-age garbage`) — the
	// real error scrolls off-screen above 100+ lines of usage. Set
	// OnUsageError on the app and every command/subcommand so we just
	// emit the error itself. The error surfaces via log.Fatal in main.
	app.OnUsageError = quietUsageError
	applyQuietUsageError(app.Commands)

	return app
}

// quietUsageError suppresses urfave/cli's default "print full help on
// any flag parse error" behavior. The error itself is informative;
// the help dump just buries it. (MT-66)
func quietUsageError(_ *cli.Context, err error, _ bool) error {
	return err
}

// applyQuietUsageError walks the command tree and installs
// quietUsageError on every command/subcommand that doesn't already
// declare its own usage-error handler.
func applyQuietUsageError(cmds []*cli.Command) {
	for _, c := range cmds {
		if c.OnUsageError == nil {
			c.OnUsageError = quietUsageError
		}
		if len(c.Subcommands) > 0 {
			applyQuietUsageError(c.Subcommands)
		}
	}
}

func main() {
	app := createApp()

	if err := app.Run(reorderArgs(os.Args, app)); err != nil {
		log.Fatal(err)
	}
}

// reorderArgs makes urfave/cli v2 accept flags after positional args. The
// library uses Go's stdlib flag.Parse, which stops at the first non-flag
// token, so `mooncake fleet apply <plan> --step-filter X` would otherwise
// reject the flag. We walk the subcommand chain in os.Args, then shuffle
// the tail so any --flag (and its value) precedes the bare positionals.
//
// The flag-vs-positional split needs to know which flags take a value
// (every non-bool urfave flag does) — we look up the matched subcommand's
// Flags slice. Tokens after `--` are passed through unchanged.
func reorderArgs(args []string, app *cli.App) []string {
	if len(args) < 2 {
		return args
	}

	// Walk subcommands. `head` accumulates the program name + subcommand
	// names; `cmd` ends pointing at the deepest matched command (or nil).
	head := []string{args[0]}
	i := 1
	var cmd *cli.Command
	cmds := app.Commands
	for i < len(args) {
		name := args[i]
		var match *cli.Command
		for _, c := range cmds {
			if c.HasName(name) {
				match = c
				break
			}
		}
		if match == nil {
			break
		}
		head = append(head, name)
		cmd = match
		cmds = match.Subcommands
		i++
	}
	if cmd == nil {
		return args
	}

	// Map each flag name (long + aliases) to "takes a value" — true for
	// every urfave/cli v2 flag type except *cli.BoolFlag.
	takesValue := make(map[string]bool)
	for _, f := range cmd.Flags {
		_, isBool := f.(*cli.BoolFlag)
		for _, n := range f.Names() {
			takesValue[n] = !isBool
		}
	}

	// Split the tail into flags-and-their-values vs. bare positionals.
	var flags, positionals []string
	for i < len(args) {
		tok := args[i]
		if tok == "--" {
			// `--` ends flag parsing; everything after is positional.
			positionals = append(positionals, args[i:]...)
			break
		}
		if strings.HasPrefix(tok, "-") && len(tok) > 1 {
			flags = append(flags, tok)
			// `--name=value` already carries its value; `--name value`
			// needs the next token IF the flag is known to take one.
			if !strings.Contains(tok, "=") {
				name := strings.TrimLeft(tok, "-")
				if takesValue[name] && i+1 < len(args) {
					flags = append(flags, args[i+1])
					i++
				}
			}
			i++
			continue
		}
		positionals = append(positionals, tok)
		i++
	}

	out := make([]string, 0, len(args))
	out = append(out, head...)
	out = append(out, flags...)
	out = append(out, positionals...)
	return out
}
