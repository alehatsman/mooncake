// Package main provides the mooncake CLI application.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/diff"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/explain"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/factsfmt"
	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/ops"
	"github.com/alehatsman/mooncake/internal/pilot"
	"github.com/alehatsman/mooncake/internal/plan"
	_ "github.com/alehatsman/mooncake/internal/register" // Register action handlers
	"github.com/alehatsman/mooncake/internal/schemagen"
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
	// step.completed JSON either. Hard-error when the user explicitly
	// sets either flag without the partner. c.IsSet skips the case
	// where the default-valued flag is silently in scope.
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

	// R1.1a: orchestration lives in internal/apply. cmd.run is now a
	// flag-parse + Config-construct + Runner.Run shim. The kernel's
	// Apply() entry point is callable directly from MCP / agent loop /
	// future SDK without going through CLI parsing. See
	// docs-working/vision/kernel.md for the kernel framing.
	cfg := &apply.Config{
		ConfigPath:        configPath,
		VarsFiles:         resolvedVars,
		Tags:              parseTags(c.String("tags")),
		SkipTags:          parseTags(c.String("skip-tags")),
		SudoPass:          c.String("sudo-pass"),
		SudoPassFile:      c.String("sudo-pass-file"),
		AskBecomePass:     c.Bool("ask-become-pass"),
		InsecureSudoPass:  c.Bool("insecure-sudo-pass"),
		TUI:               c.Bool("tui"),
		LogLevel:          c.String("log-level"),
		OutputFormat:      c.String("output-format"),
		ArtifactsDir:      c.String("artifacts-dir"),
		CaptureFullOutput: c.Bool("capture-full-output"),
		MaxOutputBytes:    c.Int("max-output-bytes"),
		MaxOutputLines:    c.Int("max-output-lines"),
		FactsJSONPath:     c.String("facts-json"),
		OpID:              recordOp("apply", configPath, false),
	}
	return runWithSignalCtx(c.Context, func(ctx context.Context) error {
		_, runErr := apply.NewRunner(cfg).Run(ctx)
		return runErr
	})
}

// runFromPlan is now a CLI shim over apply.NewRunnerFromPlan (R1.1c).
// The plan-load + spec-16 stale-plan validation + executor dispatch
// live in internal/apply alongside the config-path Runner so both
// apply entry points produce the same typed *KernelResult.
func runFromPlan(c *cli.Context, planPath string) error {
	opID := recordOp("apply", planPath, false)
	return runWithSignalCtx(c.Context, func(ctx context.Context) error {
		_, err := apply.NewRunnerFromPlan(planPath, apply.FromPlanOptions{
			SudoPass:   c.String("sudo-pass"),
			LogLevel:   c.String("log-level"),
			MaxPlanAge: c.Duration("max-plan-age"),
			AllowStale: c.Bool("allow-stale"),
			OpID:       opID,
		}).Run(ctx)
		return err
	})
}

// runWithSignalCtx wires SIGINT/SIGTERM handling for CLI-launched
// applies and translates a received signal into the standard exit code
// (130 for SIGINT, 143 for SIGTERM). Signal handling lives here, in
// the CLI, rather than in apply.Runner — F020. Embedded callers
// (agentd, MCP, future SDK) call apply.Runner directly and drive
// their own shutdown without going through this wrapper.
//
// On signal the goroutine prints a friendly stderr message and calls
// os.Exit immediately — the same shape as the pre-F020 kernel handler,
// just relocated to the CLI. Hard-exit is required today because the
// executor's hot loop does not yet observe ctx cancellation, so a
// running shell child (e.g. `sleep 30`) would otherwise block apply
// for its full duration after a Ctrl-C. F016 is the follow-up to
// thread ctx through executor → handler → exec.CommandContext so the
// apply can drain on signal instead of being killed by os.Exit.
//
// recordOp mints an op_id, writes an ops.jsonl entry, and returns the
// minted id so callers can stamp it onto apply.Config / FromPlanOptions
// / plan-mode metadata (spec-68 wave 2). Best-effort: when the append
// fails we still return the id so the run can proceed; the missing
// ops.jsonl row just means `mooncake explain op/<id>` won't resolve.
//
// Lives at the CLI layer because actor / args / config-path are CLI
// concerns; the apply / plan packages take an already-minted opID as
// input and don't reach back here.
func recordOp(command, configPath string, planOnly bool) string {
	opID := ops.NewOpID()
	user := os.Getenv("USER")
	if user == "" {
		user = "unknown"
	}
	// os.Args[0] is the binary path; skip it so Args is just the
	// post-binary flags the user typed.
	var args []string
	if len(os.Args) > 1 {
		args = append(args, os.Args[1:]...)
	}
	_ = ops.Append(ops.Entry{
		TS:       time.Now().UTC(),
		OpID:     opID,
		Command:  command,
		Args:     args,
		Actor:    "user:" + user,
		Config:   configPath,
		PlanOnly: planOnly,
	})
	return opID
}

// ctx is wired through to the body so future cancellation-aware
// handlers observe the signal as a cancelled context.
func runWithSignalCtx(parent context.Context, body func(context.Context) error) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	done := make(chan struct{})
	go func() {
		select {
		case sig := <-sigCh:
			fmt.Fprintf(os.Stderr, "\n⚠ received %s, aborting apply\n", sig)
			// Stop listening so a follow-up signal during shutdown hits
			// the default handler and hard-kills if we hang anywhere.
			signal.Stop(sigCh)
			cancel()
			code := 130 // SIGINT
			if sig == syscall.SIGTERM {
				code = 143
			}
			os.Exit(code)
		case <-done:
		}
	}()

	err := body(ctx)
	close(done)
	return err
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
		factsfmt.DisplayFacts(f)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// explainCommand resolves a noun (action verb, run id, resource handle, op id)
// and renders the typed payload as text / JSON / YAML. See spec-68.
//
// Wave 1: only kind:action resolves; run / resource / op fall through to a
// typed not_found.
func explainCommand(c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("usage: mooncake explain <noun>")
	}
	noun := c.Args().First()

	format := c.String("format")
	switch format {
	case outputFormatText, outputFormatJSON, outputFormatYAML:
	default:
		return fmt.Errorf("invalid format: %s (use 'text', 'json', or 'yaml')", format)
	}

	// F044: mirror the MCP-side validation. The flag default is 3 so
	// users who omit the flag never hit this; users who pass an
	// out-of-range value get a clear rejection.
	limit := c.Int("examples-limit")
	const explainExamplesLimitMax = 10
	if limit < 0 {
		return fmt.Errorf("--examples-limit must be >= 0 (got %d)", limit)
	}
	if limit > explainExamplesLimitMax {
		return fmt.Errorf("--examples-limit must be <= %d (got %d)", explainExamplesLimitMax, limit)
	}

	result := explain.Resolve(noun, explain.Options{
		ExamplesLimit: limit,
	})

	// not_found on action lookups is an agent-recoverable signal, but on the
	// CLI we want a non-zero exit so shell pipelines / `&&` chains stop.
	switch format {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
	case outputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(yamlIndentSpaces)
		defer func() { _ = enc.Close() }()
		if err := enc.Encode(result); err != nil {
			return err
		}
	case outputFormatText:
		renderExplainText(os.Stdout, result)
	}

	if result.Kind == explain.KindNotFound {
		return cli.Exit("", 1)
	}
	return nil
}

func renderExplainText(w io.Writer, r explain.Result) {
	switch r.Kind {
	case explain.KindAction:
		renderExplainActionText(w, r.Action)
	case explain.KindRun:
		renderExplainRunText(w, r.Run)
	case explain.KindResource:
		renderExplainResourceText(w, r.Resource)
	case explain.KindOp:
		renderExplainOpText(w, r.Op)
	case explain.KindNotFound:
		renderExplainNotFoundText(w, r.NotFound)
	default:
		fmt.Fprintf(w, "kind: %s (no text renderer)\n", r.Kind)
	}
}

func renderExplainRunText(w io.Writer, p *explain.RunPayload) {
	fmt.Fprintf(w, "run: %s\n", p.RunID)
	if p.OpID != "" {
		fmt.Fprintf(w, "  op:       %s\n", p.OpID)
	}
	fmt.Fprintf(w, "  ts:       %s\n", p.TS.Format(time.RFC3339))
	if p.Config != "" {
		fmt.Fprintf(w, "  config:   %s\n", p.Config)
	}
	if p.DurationMs > 0 {
		fmt.Fprintf(w, "  duration: %dms\n", p.DurationMs)
	}
	fmt.Fprintf(w, "  totals:   changed=%d ok=%d skipped=%d failed=%d\n",
		p.Totals.Changed, p.Totals.Ok, p.Totals.Skipped, p.Totals.Failed)
	if p.Caveats.IrreversibleStepCount > 0 {
		fmt.Fprintf(w, "  caveats:  %d irreversible step(s)\n", p.Caveats.IrreversibleStepCount)
	}
	if len(p.Steps) > 0 {
		fmt.Fprintln(w, "\nsteps:")
		for _, s := range p.Steps {
			rev := ""
			if s.Reversible {
				rev = " (reversible)"
			}
			res := s.Resource
			if res == "" {
				res = "-"
			}
			fmt.Fprintf(w, "  %2d. %-20s %-7s  %s%s\n", s.Index, s.Action, s.Result, res, rev)
		}
	}
}

func renderExplainResourceText(w io.Writer, p *explain.ResourcePayload) {
	fmt.Fprintf(w, "resource: %s\n", p.Resource)
	if len(p.History) == 0 {
		fmt.Fprintln(w, "  history: (none — this resource has not been touched by any logged run)")
		return
	}
	fmt.Fprintln(w, "\nhistory (newest first):")
	for _, h := range p.History {
		rev := ""
		if h.Reversible {
			rev = " (reversible)"
		}
		// F045: when the same run touched this resource multiple times,
		// the rows would otherwise be visually identical (same TS, same
		// RunID, same Action, same Result). step=N gives readers a
		// stable ordering key. Pre-spec-68 runs have no step index;
		// omit the suffix there.
		step := ""
		if h.StepIndex > 0 {
			step = fmt.Sprintf(" step=%d", h.StepIndex)
		}
		fmt.Fprintf(w, "  %s  %-20s %-7s  run=%s%s%s\n",
			h.TS.Format(time.RFC3339), h.Action, h.Result, h.RunID, step, rev)
	}
}

func renderExplainOpText(w io.Writer, p *explain.OpPayload) {
	fmt.Fprintf(w, "op: %s\n", p.OpID)
	fmt.Fprintf(w, "  ts:       %s\n", p.TS.Format(time.RFC3339))
	fmt.Fprintf(w, "  command:  %s\n", p.Command)
	if len(p.Args) > 0 {
		fmt.Fprintf(w, "  args:     %s\n", strings.Join(p.Args, " "))
	}
	if p.Actor != "" {
		fmt.Fprintf(w, "  actor:    %s\n", p.Actor)
	}
	if p.Parent != "" {
		fmt.Fprintf(w, "  parent:   %s\n", p.Parent)
	}
	if p.Config != "" {
		fmt.Fprintf(w, "  config:   %s\n", p.Config)
	}
	if p.PlanOnly {
		fmt.Fprintln(w, "  plan_only: true")
	}
	if len(p.Runs) > 0 {
		fmt.Fprintln(w, "\nruns:")
		for _, r := range p.Runs {
			fmt.Fprintf(w, "  - %s\n", r)
		}
	} else if !p.PlanOnly {
		fmt.Fprintln(w, "  runs:     (none yet)")
	}
}

func renderExplainActionText(w io.Writer, p *explain.ActionPayload) {
	fmt.Fprintf(w, "action: %s\n", p.Name)
	if p.Metadata.Description != "" {
		fmt.Fprintf(w, "  description: %s\n", p.Metadata.Description)
	}
	if p.Metadata.Category != "" {
		fmt.Fprintf(w, "  category:    %s\n", p.Metadata.Category)
	}
	if p.Metadata.Version != "" {
		fmt.Fprintf(w, "  version:     %s\n", p.Metadata.Version)
	}
	if len(p.Metadata.SupportedPlatforms) > 0 {
		fmt.Fprintf(w, "  platforms:   %s\n", strings.Join(p.Metadata.SupportedPlatforms, ", "))
	}
	fmt.Fprintf(w, "  dry_run:     %t\n", p.Metadata.SupportsDryRun)
	fmt.Fprintf(w, "  become:      %t\n", p.Metadata.SupportsBecome)
	fmt.Fprintf(w, "  check:       %t\n", p.Metadata.ImplementsCheck)

	if p.Schema != nil && len(p.Schema.Properties) > 0 {
		fmt.Fprintln(w, "\nschema:")
		required := map[string]bool{}
		for _, r := range p.Schema.Required {
			required[r] = true
		}
		names := make([]string, 0, len(p.Schema.Properties))
		for n := range p.Schema.Properties {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			prop := p.Schema.Properties[n]
			marker := " "
			if required[n] {
				marker = "*"
			}
			t := prop.Type
			if t == "" && len(prop.OneOf) > 0 {
				t = strings.Join(prop.OneOf, "|")
			}
			fmt.Fprintf(w, "  %s %-20s %s", marker, n, t)
			if prop.Description != "" {
				fmt.Fprintf(w, "  — %s", prop.Description)
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "  (* required)")
	}

	fmt.Fprintln(w, "\ndiff:    ", p.DiffShape.Note)
	fmt.Fprintln(w, "reverse: ", p.ReverseShape.Caveat)

	if len(p.Examples) > 0 {
		fmt.Fprintln(w, "\nexamples:")
		for _, ex := range p.Examples {
			fmt.Fprintf(w, "  %s\n", ex.Path)
			for _, line := range strings.Split(strings.TrimRight(ex.Excerpt, "\n"), "\n") {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}
	}
}

func renderExplainNotFoundText(w io.Writer, p *explain.NotFoundPayload) {
	fmt.Fprintf(w, "not_found: %q\n", p.Noun)
	if p.Reason != "" {
		fmt.Fprintf(w, "  reason: %s\n", p.Reason)
	}
	if len(p.Candidates) > 0 {
		fmt.Fprintln(w, "  did you mean:")
		for _, cand := range p.Candidates {
			fmt.Fprintf(w, "    - %s (%s)\n", cand.ID, cand.Kind)
		}
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
//
// proposal-05: DIFF/COST/REVERSE/PERM columns surface the spec-22
// four-method ABI per handler. Values come from ActionMetadata's
// Implements* bools, which Registry.List() populates centrally from
// the live interface satisfaction (so the table can't drift from
// what each handler actually implements).
func displayActionsTable(actionsList []actions.ActionMetadata) {
	const rowFmt = "%-15s %-10s %-25s %-5s %-5s %-5s %-5s %-7s %-5s\n"
	// Print header
	fmt.Printf(rowFmt,
		"ACTION", "CATEGORY", "PLATFORMS",
		"SUDO", "CHECK", "DIFF", "COST", "REVERSE", "PERM")
	fmt.Println(strings.Repeat("-", 95))

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

		fmt.Printf(rowFmt,
			meta.Name,
			meta.Category,
			platforms,
			yesNo(meta.RequiresSudo),
			yesNo(meta.ImplementsCheck),
			yesNo(meta.ImplementsDiff),
			yesNo(meta.ImplementsCost),
			yesNo(meta.ImplementsReverse),
			yesNo(meta.ImplementsPermissions))
	}
}

// yesNo renders a bool as the two-state token the actions-list table
// has used since the SUDO/CHECK columns shipped. Kept tiny + local
// because it's a display detail, not a vocabulary worth promoting.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// actionsShowCommand prints a per-action card (dx proposal-04). The
// per-action surface is "what parameters does this action take, what's
// required, what's the minimum example" — the question a user hits the
// moment after `actions list`. All data is already in the registry +
// schemagen; this is the rendering shell over both.
//
// --format text (default) prints a human-shaped card; json/yaml dump
// the schema Definition (already x-implements-*-decorated by
// proposal-05) so editors and agents can consume it directly.
func actionsShowCommand(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("specify an action name; try `mooncake actions list`")
	}
	name := c.Args().First()
	format := c.String("format")

	// Validate format before doing any work — clearer error than a
	// switch-default panic at the bottom of the function.
	switch format {
	case outputFormatText, outputFormatJSON, "yaml":
	default:
		return fmt.Errorf("invalid format: %s (use 'text', 'json', or 'yaml')", format)
	}

	// Pull the metadata via the same Registry.List() pipeline `actions
	// list` uses — the four spec-22 ABI capability bools (proposal-05)
	// are populated there from live interface satisfaction, so the
	// card stays in lockstep with the table without per-call probing.
	var meta *actions.ActionMetadata
	all := actions.List()
	for i := range all {
		if all[i].Name == name {
			meta = &all[i]
			break
		}
	}
	if meta == nil {
		known := make([]string, 0, len(all))
		for _, m := range all {
			known = append(known, m.Name)
		}
		if suggestion := nearestActionName(name, known); suggestion != "" {
			return fmt.Errorf("unknown action %q (did you mean %q? try `mooncake actions list`)", name, suggestion)
		}
		return fmt.Errorf("unknown action %q (try `mooncake actions list`)", name)
	}

	// Generate the full schema and pluck this action's Definition.
	// Extensions are on by default so x-implements-* keys ride through
	// the json/yaml form for downstream consumers.
	gen := schemagen.NewGenerator(schemagen.GeneratorOptions{
		IncludeExtensions: true,
		OutputFormat:      "json",
	})
	schema, err := gen.Generate()
	if err != nil {
		return fmt.Errorf("generate schema: %w", err)
	}
	def, ok := schema.Definitions[name]
	if !ok {
		// Registry knew the name but schemagen didn't — shouldn't
		// happen for v1, but surface a clear message rather than
		// printing a card with no fields.
		return fmt.Errorf("action %q has no schema definition (registry/schemagen drift)", name)
	}

	switch format {
	case outputFormatText:
		renderActionShowText(meta, def, os.Stdout)
		return nil
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(def)
	case "yaml":
		return yaml.NewEncoder(os.Stdout).Encode(def)
	}
	return nil
}

// renderActionShowText writes the human-shaped per-action card to w.
// Exported-ish through tests; the rest of cmd doesn't need it.
func renderActionShowText(meta *actions.ActionMetadata, def *schemagen.Definition, w io.Writer) {
	title := meta.Name
	fmt.Fprintln(w, title)
	fmt.Fprintln(w, strings.Repeat("─", len(title)))
	if def.Description != "" {
		fmt.Fprintln(w, def.Description)
	} else if meta.Description != "" {
		fmt.Fprintln(w, meta.Description)
	}
	fmt.Fprintln(w)

	platforms := "all"
	if len(meta.SupportedPlatforms) > 0 {
		platforms = strings.Join(meta.SupportedPlatforms, ", ")
	}
	fmt.Fprintf(w, "Category:         %s\n", meta.Category)
	fmt.Fprintf(w, "Platforms:        %s\n", platforms)
	fmt.Fprintf(w, "Requires sudo:    %s\n", yesNo(meta.RequiresSudo))
	fmt.Fprintf(w, "Implements check: %s\n", yesNo(meta.ImplementsCheck))
	fmt.Fprintf(w, "Implements diff:  %s\n", yesNo(meta.ImplementsDiff))
	fmt.Fprintf(w, "Implements cost:  %s\n", yesNo(meta.ImplementsCost))
	fmt.Fprintf(w, "Implements reverse: %s\n", yesNo(meta.ImplementsReverse))
	fmt.Fprintf(w, "Implements permissions: %s\n", yesNo(meta.ImplementsPermissions))
	fmt.Fprintf(w, "Supports dry-run: %s\n", yesNo(meta.SupportsDryRun))
	if meta.Version != "" {
		fmt.Fprintf(w, "Version:          %s\n", meta.Version)
	}
	if len(meta.EmitsEvents) > 0 {
		fmt.Fprintf(w, "Emits events:     %s\n", strings.Join(meta.EmitsEvents, ", "))
	}

	required := map[string]bool{}
	for _, name := range def.Required {
		required[name] = true
	}
	var reqNames, optNames []string
	for n := range def.Properties {
		if required[n] {
			reqNames = append(reqNames, n)
		} else {
			optNames = append(optNames, n)
		}
	}
	sort.Strings(reqNames)
	sort.Strings(optNames)

	if len(reqNames) > 0 {
		fmt.Fprintln(w, "\nRequired parameters:")
		for _, n := range reqNames {
			fmt.Fprintln(w, "  "+formatPropertyLine(n, def.Properties[n]))
		}
	}
	if len(optNames) > 0 {
		fmt.Fprintln(w, "\nOptional parameters:")
		for _, n := range optNames {
			fmt.Fprintln(w, "  "+formatPropertyLine(n, def.Properties[n]))
		}
	}

	// Minimum example: a single-step playbook with the required fields
	// only, default-valued. Skipped when no required fields exist (the
	// action is either parameterless or takes a scalar — neither warrants
	// a synthetic example with no real semantics).
	if len(reqNames) > 0 {
		fmt.Fprintln(w, "\nMinimum example:")
		fmt.Fprintf(w, "  - %s:\n", meta.Name)
		for _, n := range reqNames {
			fmt.Fprintf(w, "      %s: %s\n", n, exampleValue(def.Properties[n]))
		}
	}
}

// formatPropertyLine renders one row in the required/optional table:
// "name  type      description". Width is fixed so columns align
// across short and long names; long descriptions wrap on whitespace
// to keep card width sensible.
func formatPropertyLine(name string, p *schemagen.Property) string {
	t := p.Type
	if t == "" && p.Ref != "" {
		t = "ref"
	}
	if t == "" {
		t = "object"
	}
	desc := strings.TrimSpace(p.Description)
	if desc == "" {
		desc = "—"
	}
	return fmt.Sprintf("%-18s %-9s %s", name, t, desc)
}

// exampleValue picks a stand-in literal for a property when building
// the minimum example. Strings → "string"; integers → 0; bools →
// false; arrays → []; everything else → null. Cheap, but enough to
// make the rendered example syntactically valid YAML.
func exampleValue(p *schemagen.Property) string {
	if p == nil {
		return "null"
	}
	switch p.Type {
	case "string":
		return `"…"`
	case "integer":
		return "0"
	case "number":
		return "0.0"
	case "boolean":
		return "false"
	case "array":
		return "[]"
	case "object":
		return "{}"
	}
	return "null"
}

// nearestActionName returns the closest match from candidates using
// a cheap edit-distance metric, or "" when no candidate is close
// enough for a confident suggestion. Mirrors closestTag /
// levenshtein in internal/plan/filter but uses tighter thresholds:
// action names are longer than tags (e.g. "pkg.install" vs "linux"),
// so the filter package's 67%-of-maxLen window admits matches a user
// wouldn't recognise. Add an absolute edit-distance cap of 4 — human
// typos cluster at 1-2 characters; 4 covers transpose + substitute
// + adjacent insertion without admitting "completely-unrelated"-
// suggests-"file.template".
func nearestActionName(needle string, candidates []string) string {
	if needle == "" || len(candidates) == 0 {
		return ""
	}
	const maxAbsoluteDist = 4
	best := ""
	bestDist := 1 << 30
	for _, c := range candidates {
		d := actionsShowDistance(needle, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	if bestDist > maxAbsoluteDist {
		return best[:0] // empty — distance too large for confident suggestion
	}
	maxLen := len(needle)
	if len(best) > maxLen {
		maxLen = len(best)
	}
	if maxLen > 0 && bestDist*2 <= maxLen {
		return best
	}
	return ""
}

// actionsShowDistance is a small Levenshtein wrapper local to cmd —
// the filter package's helper is unexported and we don't want to
// promote it to public API for one caller.
func actionsShowDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = prev[j] + 1
			if cur[j-1]+1 < cur[j] {
				cur[j] = cur[j-1] + 1
			}
			if prev[j-1]+cost < cur[j] {
				cur[j] = prev[j-1] + cost
			}
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

// writeFactsJSON is preserved for cmd/cmd_test.go's coverage of the
// snake_case marshal pattern. The live caller moved to
// internal/apply/runner.go's writeFactsJSON in R1.1a; this cmd-side
// copy stays so the existing TestWriteFactsJSON* suite keeps pinning
// the contract.
//
//nolint:unused // covered by cmd/cmd_test.go; lint runs with tests:false.
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
	// Spec-68 wave 2: every `mooncake plan` invocation produces an
	// ops.jsonl entry with PlanOnly=true and Runs=[]. No runs.jsonl
	// row is written for a plan-only op. The id is discarded here
	// because the plan path doesn't surface it back to the user yet;
	// future iterations may print it alongside the plan summary.
	_ = recordOp("plan", configPath, true)
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
			if r := diff.Lookup(ins.Detail, ins.Diff); r != nil {
				var buf strings.Builder
				if err := r.Render(&buf, diff.FormatText); err == nil && buf.Len() > 0 {
					fmt.Print(buf.String())
					fmt.Println()
				}
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
//
//	1–3   safe (read-only, idempotent writes to scratch)
//	4–6   routine (config writes, package installs)
//	7–9   high impact (service restarts, kernel params)
//	10    destructive (deletes, drops, rm -rf)
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

func pilotRunCommand(c *cli.Context) error {
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

	opts := pilot.RunOptions{
		Goal:          goal,
		PlanPath:      planPath,
		UseStdin:      useStdin,
		RepoRoot:      repoRoot,
		Provider:      provider,
		Model:         model,
		MaxIterations: maxIterations,
		AutoApply:     c.Bool("auto-apply"),
	}

	if provider == "claude" {
		result, loopErr := pilot.RunLoop(opts)
		if loopErr != nil {
			fmt.Fprintf(os.Stderr, "Pilot loop failed: %v\n", loopErr)
			if result != nil && result.FinalLog != nil {
				printPilotSummary(result.FinalLog)
			}
			return loopErr
		}

		fmt.Printf("Pilot completed: %d iterations\n", len(result.Iterations))
		fmt.Printf("Stop reason: %s\n", result.StopReason)
		if result.FinalLog != nil {
			fmt.Println()
			printPilotSummary(result.FinalLog)
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

	log, err := pilot.Run(opts)
	if err != nil {
		return err
	}

	printPilotSummary(log)
	return nil
}

func printPilotSummary(log *pilot.IterationLog) {
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
		// Fleet DX proposal-01: --peer uses commas as AND-group
		// separators inside `@k=v,k2=v2` selectors. urfave/cli's
		// default behavior auto-splits StringSliceFlag values on
		// commas BEFORE the action sees them, which would silently
		// turn one AND-group into N OR-groups. Disable the cli-level
		// split; internal parsers (extractStepFilter,
		// derivePsStatusFilter) already split on commas themselves.
		DisableSliceFlagSeparator: true,

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
				Name:      "explain",
				Usage:     "Look up typed information about a mooncake noun (action verb, run, resource, op)",
				ArgsUsage: "<noun>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Value:   outputFormatText,
						Usage:   "Output format: text, json, or yaml",
					},
					&cli.IntFlag{
						Name:  "examples-limit",
						Value: 3,
						Usage: "Max example excerpts to include for kind:action results",
					},
				},
				Action: explainCommand,
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
					{
						Name:      "show",
						Usage:     "Show per-action documentation (params, platforms, capabilities, minimum example)",
						ArgsUsage: "<action-name>",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "format",
								Aliases: []string{"f"},
								Value:   "text",
								Usage:   "Output format: text, json, or yaml",
							},
						},
						Action: actionsShowCommand,
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
						Name:      "follow",
						Usage:     "Stream events for an existing run, rendered like `apply`",
						ArgsUsage: "<run_id>",
						Flags: []cli.Flag{
							&cli.BoolFlag{Name: "system", Usage: "Use the system-mode agentd socket"},
						},
						Action: runsFollowCommand,
					},
					{
						Name:      "get",
						Usage:     "Print the JSON record for one run",
						ArgsUsage: "<run_id>",
						Flags:     []cli.Flag{&cli.BoolFlag{Name: "system"}},
						Action:    runsGetCommand,
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
				Name:  "pilot",
				Usage: "Pilot operations",
				Subcommands: []*cli.Command{
					{
						Name:  "run",
						Usage: "Execute pilot iteration",
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
							&cli.BoolFlag{
								Name:  "auto-apply",
								Usage: "Skip the plan-confirm gate (required for unattended/CI runs; spec-67 §10)",
							},
						},
						Action: pilotRunCommand,
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

// installApplySignalHandler used to live here; the apply orchestration
// moved to internal/apply (R1.1a). The signal handler lives there now
// alongside the executor.Start call it gates.
