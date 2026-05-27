package apply

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/ops"
	"github.com/alehatsman/mooncake/internal/plan"
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
// NewRunner for the config-path apply, or NewRunnerFromPlan for the
// saved-plan apply (R1.1c). Both call Run(ctx) and return the same
// *KernelResult shape so downstream consumers (MCP, agent loop,
// future SDK) compose uniformly across input sources.
type Runner struct {
	// config-path mode
	cfg *Config

	// from-plan mode (R1.1c). When fromPlan is true, fromPlanPath
	// and fromPlanOpts drive the saved-plan apply path instead of
	// compiling from cfg.ConfigPath. fromPlan and cfg are mutually
	// exclusive — set by exactly one constructor.
	fromPlan     bool
	fromPlanPath string
	fromPlanOpts FromPlanOptions

	// in-memory-plan mode. When inMemoryPlan is true, the caller has
	// already built a *plan.Plan and wants the Runner to execute it
	// without re-reading the source config. Set exclusively by
	// NewRunnerFromInMemoryPlan; mutually exclusive with the config-
	// path and saved-plan modes. The primary consumer is
	// `mooncake task <name>`, which builds plans with the planner's
	// TaskName field and bypasses apply's config-reading path so
	// apply stays unaware of tasks.
	inMemoryPlan     bool
	inMemoryPlanData *plan.Plan
	inMemoryPlanOpts InMemoryPlanOptions
}

// NewRunner constructs a Runner around the given Config. cfg must
// not be nil. Config is not deep-copied — the caller is responsible
// for not mutating it after the call.
func NewRunner(cfg *Config) *Runner {
	return &Runner{cfg: cfg}
}

// Run validates the Config, sets up the event substrate (publisher
// + subscribers), dispatches to executor.Start, and returns a typed
// *KernelResult plus the run's error (R1.1b). The returned
// *KernelResult is never nil — on a validation failure or pre-plan
// error it carries a populated Summary with Success=false and a
// non-empty ErrorMessage; callers inspecting it after a non-nil error
// get a consistent shape.
//
// Signal handling is the *caller's* responsibility (F020): the CLI
// wraps ctx with signal.NotifyContext, agentd has its own shutdown
// path, MCP / SDK callers cancel ctx as they see fit. Run never calls
// os.Exit — if it did, an embedded caller's graceful shutdown could
// not survive a SIGTERM mid-apply.
//
// Context is wired through for future cancellation (the executor
// does not yet observe it). Direct callers (MCP, agent loop) get the
// same surface.
func (r *Runner) Run(ctx context.Context) (*KernelResult, error) {
	// R1.1c: dispatch to the saved-plan path when constructed via
	// NewRunnerFromPlan. The two paths share publisher / capture /
	// result-assembly plumbing but diverge on what the executor
	// receives (a config-file path vs a pre-built plan).
	if r.fromPlan {
		return r.runFromPlan(ctx)
	}
	if r.inMemoryPlan {
		return r.runFromInMemoryPlan(ctx)
	}

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

	// Inject caller-supplied subscribers first so they see every event
	// the kernel emits (including plan.loaded which fires before most
	// standard subscribers are wired). agentd uses this for its SSE hub
	// and events.jsonl sink without bypassing the kernel boundary.
	for _, sub := range r.cfg.ExtraSubscribers {
		publisher.Subscribe(sub)
	}

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

	// Always record run history (best-effort). When the caller minted an
	// op_id we write an enriched entry post-flush (spec-68 wave 2) and
	// skip the totals-only subscriber to avoid double-writing.
	var runID string
	if r.cfg.OpID != "" {
		runID = ops.NewRunID()
	} else {
		publisher.Subscribe(logger.NewRunLogSubscriber(r.cfg.ConfigPath))
	}

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
				publisher.Subscribe(logger.NewConsoleSubscriber(level, r.cfg.OutputFormat, r.cfg.StreamStepOutput))
			} else {
				tuiSubscriber.Start()
				defer tuiSubscriber.Stop()
				publisher.Subscribe(tuiSubscriber)
			}
		} else {
			// Console subscriber for text / json output.
			publisher.Subscribe(logger.NewConsoleSubscriber(level, r.cfg.OutputFormat, r.cfg.StreamStepOutput))
		}
	}

	// Minimal logger for internal use (errors, etc.)
	internalLog := logger.NewLogger(level)

	// F020: signal handling lives in the CLI caller (cmd/mooncake.go
	// wraps ctx with signal.NotifyContext). The kernel does not call
	// os.Exit; embedded callers (agentd, MCP, SDK) cancel ctx via their
	// own shutdown paths.

	// R1.1b: hand executor.Start a *RunCapture so the kernel result
	// carries the compiled plan and per-step records. Without this
	// hook the executor's hot path is identical to the legacy code —
	// the apply.Runner is the only caller setting it today.
	capture := &executor.RunCapture{}

	execErr := executor.Start(ctx, executor.StartConfig{
		ConfigFilePath:   r.cfg.ConfigPath,
		VarsFilePaths:    r.cfg.VarsFiles,
		SudoPass:         r.cfg.SudoPass,
		SudoPassFile:     r.cfg.SudoPassFile,
		AskBecomePass:    r.cfg.AskBecomePass,
		InsecureSudoPass: r.cfg.InsecureSudoPass,
		Tags:             r.cfg.Tags,
		SkipTags:         r.cfg.SkipTags,
		Names:            r.cfg.Names,

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

	// ExtraSubscribers may buffer events in their own goroutines
	// (e.g. RunEventSink's writeLoop). Flush() guarantees all OnEvent
	// calls are complete, so it is safe to Close them here — their
	// internal queues drain and flush to backing stores before this
	// function returns. publisher.Close() (deferred) closes channels
	// but does NOT call subscriber.Close().
	for _, sub := range r.cfg.ExtraSubscribers {
		sub.Close()
	}

	// Spec-68 wave 2: when the caller minted an op_id, write the
	// enriched runlog entry (totals + RunID + OpID + per-step records).
	// The totals-only subscriber path is suppressed above; this is the
	// single Append site for op-aware runs.
	if r.cfg.OpID != "" {
		writeEnrichedRunlog(filepath.Base(r.cfg.ConfigPath), r.cfg.OpID, runID, tail, capture)
	}

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
			Cancelled:    runData.CancelledSteps,
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
