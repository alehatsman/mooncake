package apply

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/logger"
)

// Valid output-format values for Config.OutputFormat. Kept private —
// the canonical user-facing names ship from the CLI layer; internal
// callers (MCP / SDK / agent loop) should use these literals directly.
const (
	outputFormatText  = "text"
	outputFormatJSON  = "json"
	outputFormatAgent = "agent"
	outputFormatQuiet = "quiet"
)

// Runner is the kernel's local-apply entry point. Construct with
// NewRunner, then call Run(ctx).
//
// Today Run returns a flat error matching the pre-extraction CLI
// shape; R1.1b crystallizes the typed *KernelResult contract on top
// of this. Direct callers (MCP, agent loop) get the same surface.
type Runner struct {
	cfg *Config
}

// NewRunner constructs a Runner around the given Config. cfg must
// not be nil. Config is not deep-copied — the caller is responsible
// for not mutating it after the call.
func NewRunner(cfg *Config) *Runner {
	return &Runner{cfg: cfg}
}

// Run validates the Config, sets up the event substrate (publisher
// + subscribers), installs SIGINT/SIGTERM handling, dispatches to
// executor.Start, and returns a typed *KernelResult plus the run's
// error (R1.1b). The returned *KernelResult is never nil — on a
// validation failure or pre-plan error it carries a populated
// Summary with Success=false and a non-empty ErrorMessage; callers
// inspecting it after a non-nil error get a consistent shape.
//
// Context is wired through for future cancellation (the executor
// does not yet observe it; SIGINT/SIGTERM handling is installed
// separately). Direct callers (MCP, agent loop) get the same surface.
func (r *Runner) Run(_ context.Context) (*KernelResult, error) {
	if err := r.validate(); err != nil {
		return failedResult(err), err
	}

	// Collect facts early if facts-json requested. Best-effort:
	// failure is logged but does not abort the run.
	if r.cfg.FactsJSONPath != "" {
		systemFacts := facts.Collect()
		if err := writeFactsJSON(systemFacts, r.cfg.FactsJSONPath); err != nil {
			log.Printf("Warning: failed to write facts JSON: %v", err)
		}
	}

	// Event publisher + subscribers.
	publisher := events.NewPublisher()
	defer publisher.Close()

	// R1.1b: install an event-tail capture subscriber so the
	// *KernelResult's Events field carries the run's audit substrate.
	// Plan + per-step records flow through executor.RunCapture (see
	// below) rather than the publisher because they need typed step
	// data that doesn't fit in the JSON event payloads.
	tail := newCaptureSubscriber()
	publisher.Subscribe(tail)

	level := parseLogLevel(r.cfg.LogLevel)

	// Always emit structured JSON errors to stderr on step failures.
	publisher.Subscribe(logger.NewStderrErrorSubscriber())

	// Always record run history (best-effort).
	publisher.Subscribe(logger.NewRunLogSubscriber(r.cfg.ConfigPath))

	// One-shot next-step hint after the first successful run on this
	// host. The subscriber is self-suppressing for non-text formats
	// and respects MOONCAKE_NO_HINTS=1.
	publisher.Subscribe(logger.NewFirstRunHintSubscriber(os.Stdout, r.cfg.OutputFormat))

	// Output-mode-specific subscriber.
	switch r.cfg.OutputFormat {
	case outputFormatAgent:
		publisher.Subscribe(logger.NewAgentSubscriber())
	case outputFormatQuiet:
		publisher.Subscribe(logger.NewQuietSubscriber())
	default:
		if r.cfg.TUI && logger.IsTUISupported() {
			// Use TUI subscriber when explicitly requested.
			tuiSubscriber, err := logger.NewTUISubscriber(level)
			if err != nil {
				// Fallback to console subscriber if TUI init fails.
				publisher.Subscribe(logger.NewConsoleSubscriber(level, r.cfg.OutputFormat))
			} else {
				tuiSubscriber.Start()
				defer tuiSubscriber.Stop()
				publisher.Subscribe(tuiSubscriber)
			}
		} else {
			// Console subscriber for text / json output.
			publisher.Subscribe(logger.NewConsoleSubscriber(level, r.cfg.OutputFormat))
		}
	}

	// Minimal logger for internal use (errors, etc.)
	internalLog := logger.NewLogger(level)

	// issue-87: install a SIGINT/SIGTERM handler so Ctrl-C actually
	// terminates the run. Pre-fix the shell child died (process-group
	// signal delivery) but mooncake's own process stayed alive — the
	// executor caught the canceled-step error and proceeded as if the
	// step had merely failed, then sat in the next step waiting on
	// its shell etc. We exit on the first signal (after stopping the
	// handler so a follow-up Ctrl-C during shutdown can still
	// hard-kill).
	stopSig := installSignalHandler()
	defer stopSig()

	// R1.1b: hand executor.Start a *RunCapture so the kernel result
	// carries the compiled plan and per-step records. Without this
	// hook the executor's hot path is identical to the legacy code —
	// the apply.Runner is the only caller setting it today.
	capture := &executor.RunCapture{}

	execErr := executor.Start(executor.StartConfig{
		ConfigFilePath:   r.cfg.ConfigPath,
		VarsFilePaths:    r.cfg.VarsFiles,
		SudoPass:         r.cfg.SudoPass,
		SudoPassFile:     r.cfg.SudoPassFile,
		AskBecomePass:    r.cfg.AskBecomePass,
		InsecureSudoPass: r.cfg.InsecureSudoPass,
		Tags:             r.cfg.Tags,
		SkipTags:         r.cfg.SkipTags,

		// Artifact configuration.
		ArtifactsDir:      r.cfg.ArtifactsDir,
		CaptureFullOutput: r.cfg.CaptureFullOutput,
		MaxOutputBytes:    r.cfg.MaxOutputBytes,
		MaxOutputLines:    r.cfg.MaxOutputLines,

		Capture: capture,
	}, internalLog, publisher)

	// Drain pending events so the audit tail is complete before we
	// build the KernelResult. Close (deferred above) only waits on
	// forwarding goroutines; Flush waits on the per-subscriber
	// inboxes that drive captureSubscriber.OnEvent.
	publisher.Flush()

	return assembleResult(capture, tail, execErr), execErr
}

// failedResult returns a KernelResult for early-exit paths where the
// executor never ran. Used for validate() errors so callers see a
// consistent shape even when err != nil.
func failedResult(err error) *KernelResult {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &KernelResult{
		Summary: RunSummary{
			Success:      false,
			ErrorMessage: msg,
		},
	}
}

// assembleResult lifts the executor's capture and the publisher's
// event tail into a typed *KernelResult. Steps are returned in
// execution order; Events in observed order. The summary is taken
// from the run.completed event when present, falling back to a
// minimal Summary when the run failed before run.completed fired.
func assembleResult(capture *executor.RunCapture, tail *captureSubscriber, execErr error) *KernelResult {
	out := &KernelResult{Plan: capture.Plan()}

	for _, rec := range capture.Steps() {
		out.Steps = append(out.Steps, StepResult{
			Step:   rec.Step,
			Result: rec.Result,
		})
	}

	if tail != nil {
		tail.mu.Lock()
		out.Events = append(out.Events, tail.events...)
		runData := tail.run
		tail.mu.Unlock()

		out.Summary = RunSummary{
			TotalSteps:   runData.TotalSteps,
			Ok:           runData.SuccessSteps,
			Changed:      runData.ChangedSteps,
			Skipped:      runData.SkippedSteps,
			Failed:       runData.FailedSteps,
			Reverted:     runData.RevertedSteps,
			DurationMs:   runData.DurationMs,
			Success:      runData.Success,
			ErrorMessage: runData.ErrorMessage,
			CheckMode:    runData.CheckMode,
		}
	}

	// If the run never reached run.completed (catastrophic setup
	// error before plan compilation), the tail summary is the zero
	// value. Reflect the actual error so consumers see something
	// useful.
	if out.Summary.TotalSteps == 0 && execErr != nil {
		out.Summary.Success = false
		if out.Summary.ErrorMessage == "" {
			out.Summary.ErrorMessage = execErr.Error()
		}
	}
	return out
}

// validate checks the Config's invariants. Run() calls this first;
// direct callers can call it for early validation.
func (r *Runner) validate() error {
	c := r.cfg

	switch c.OutputFormat {
	case outputFormatText, outputFormatJSON, outputFormatAgent, outputFormatQuiet:
		// ok
	default:
		return fmt.Errorf("invalid output-format: %s (must be 'text', 'json', 'agent', or 'quiet')", c.OutputFormat)
	}

	if c.OutputFormat == outputFormatJSON && c.TUI {
		return fmt.Errorf("--output-format json cannot be combined with --tui")
	}

	passwordMethods := 0
	if c.SudoPass != "" {
		passwordMethods++
	}
	if c.AskBecomePass {
		passwordMethods++
	}
	if c.SudoPassFile != "" {
		passwordMethods++
	}
	if passwordMethods > 1 {
		return fmt.Errorf("only one password method can be specified (--sudo-pass, --ask-become-pass, --sudo-pass-file)")
	}

	if c.SudoPass != "" && !c.InsecureSudoPass {
		return fmt.Errorf("--sudo-pass requires --insecure-sudo-pass flag (WARNING: password will be visible in shell history and process list)")
	}

	return nil
}

// parseLogLevel maps the string value to the logger's level int.
// Unknown values (including empty) default to InfoLevel — matches
// pre-extraction behavior.
func parseLogLevel(s string) int {
	switch s {
	case "debug":
		return logger.DebugLevel
	case "error":
		return logger.ErrorLevel
	default:
		return logger.InfoLevel
	}
}

// writeFactsJSON marshals facts via ToMap() so keys are snake_case
// (MT-74), matching the daemon's /v1/facts endpoint shape.
func writeFactsJSON(f *facts.Facts, path string) error {
	data, err := json.MarshalIndent(f.ToMap(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal facts: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// installSignalHandler installs a SIGINT/SIGTERM handler that
// terminates the apply with the standard exit code (130 for SIGINT,
// 143 for SIGTERM). issue-87.
//
// The returned stop func unregisters the handler — caller defers it
// so signals after a clean apply don't keep the goroutine spinning.
func installSignalHandler() (stop func()) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-sigCh:
			fmt.Fprintf(os.Stderr, "\n⚠ received %s, aborting apply\n", sig)
			// Stop listening so a follow-up signal during shutdown
			// can hit the default handler and hard-kill if we hang.
			signal.Stop(sigCh)
			code := 130 // SIGINT
			if sig == syscall.SIGTERM {
				code = 143
			}
			os.Exit(code)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(sigCh)
		close(done)
	}
}
