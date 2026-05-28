package kernel

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/cmd/cmdutil"
	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/executor"
)

// ApplyCommand returns the `mooncake apply` cli.Command.
func ApplyCommand() *cli.Command {
	return &cli.Command{
		Name:   "apply",
		Usage:  "Apply a playbook or saved plan. Use --dry-run to preview without changes.",
		Flags:  applyFlags(),
		Action: run,
	}
}

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

		// Step output rendering: stream stdout/stderr inline by default
		// so operators see what their `cmd:` / `shell:` steps print
		// without flipping --log-level debug (which firehoses internal
		// traces too). --no-stream-output keeps only the lifecycle
		// markers (▶ / ~ / ✓ / ✗) for CI logs or noisy commands.
		&cli.BoolFlag{
			Name:  "no-stream-output",
			Value: false,
			Usage: "Hide captured stdout/stderr from steps; show only lifecycle markers. Has no effect with --output-format json (event stream is always full).",
		},
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
		return planAction(c)
	}

	// Check if running from plan
	fromPlan := c.String("from-plan")
	if fromPlan != "" {
		return runFromPlan(c, fromPlan)
	}

	// Resolve config path: explicit --config wins, else auto-discover.
	configPath, err := cmdutil.ResolveConfigPath(c)
	if err != nil {
		if cmdutil.PrintNoConfigHintAndExit(err, "apply") {
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
		Tags:              cmdutil.ParseTags(c.String("tags")),
		SkipTags:          cmdutil.ParseTags(c.String("skip-tags")),
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
		StreamStepOutput:  !c.Bool("no-stream-output"),
	}
	return runWithSignalCtx(c.Context, func(ctx context.Context) error {
		kr, runErr := apply.NewRunner(cfg).Run(ctx)
		return mapCancelExit(kr, runErr)
	})
}

// runFromPlan is now a CLI shim over apply.NewRunnerFromPlan (R1.1c).
// The plan-load + spec-16 stale-plan validation + executor dispatch
// live in internal/apply alongside the config-path Runner so both
// apply entry points produce the same typed *KernelResult.
func runFromPlan(c *cli.Context, planPath string) error {
	opID := recordOp("apply", planPath, false)
	return runWithSignalCtx(c.Context, func(ctx context.Context) error {
		kr, err := apply.NewRunnerFromPlan(planPath, apply.FromPlanOptions{
			SudoPass:   c.String("sudo-pass"),
			LogLevel:   c.String("log-level"),
			MaxPlanAge: c.Duration("max-plan-age"),
			AllowStale: c.Bool("allow-stale"),
			OpID:       opID,
		}).Run(ctx)
		return mapCancelExit(kr, err)
	})
}

// mapCancelExit translates a cancelled-but-not-failed run into the
// proposal-02 exit code 130 (SIGINT-equivalent). Other outcomes pass
// runErr through unchanged.
//
// The OS-signal path in runWithSignalCtx hard-exits with 130/143
// before this code runs. This shim covers the embedded / programmatic
// cancel paths — timeout-bounded runs, fleet-driven cancel, MCP
// shutdown — where ctx is cancelled without an OS signal and the
// Runner returns ctx.Err(). Without this, those runs exit 1, which
// makes timeouts indistinguishable from real failures in CI.
func mapCancelExit(kr *apply.KernelResult, runErr error) error {
	if kr != nil && kr.Summary.Cancelled > 0 && kr.Summary.Failed == 0 {
		return cli.Exit("", 130)
	}
	return runErr
}

// runWithSignalCtx wires SIGINT/SIGTERM handling for CLI-launched
// applies and translates a received signal into the standard exit code
// (130 for SIGINT, 143 for SIGTERM). Signal handling lives at the CLI
// layer rather than in apply.Runner — embedded callers (agentd, MCP,
// future SDK) call apply.Runner directly and drive their own shutdown.
//
// On signal the goroutine prints a friendly stderr message and calls
// os.Exit immediately. Hard-exit is required today because the
// executor's hot loop does not yet observe ctx cancellation, so a
// running shell child (e.g. `sleep 30`) would otherwise block apply
// for its full duration after a Ctrl-C. F016 is the follow-up to
// thread ctx through executor → handler → exec.CommandContext so the
// apply can drain on signal instead of being killed by os.Exit.
func runWithSignalCtx(parent context.Context, body func(context.Context) error) error {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)

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
			// Attach the signal cause so any step that errors during
			// teardown is classified as CancelledReasonSigint by
			// executor.syncResultEnvelope (F4). The os.Exit below
			// short-circuits the recap today (see F2), but the cause
			// is plumbed for when that hard-kill is removed.
			cancel(executor.ErrCancelSignal)
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
