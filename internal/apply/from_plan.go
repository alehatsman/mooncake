package apply

import (
	"context"
	"fmt"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
)

// FromPlanOptions configures a Runner that applies a pre-built saved
// plan instead of compiling a fresh one from a config file.
//
// The Runner produced by NewRunnerFromPlan uses a simpler subscriber
// fan-out than the config-path Runner: no facts-json early-write, no
// first-run-hint, no output-format choice (saved-plan apply always
// renders as text). The kernel-result shape is identical to the
// config-path Runner's so downstream consumers don't have to branch
// on input source.
type FromPlanOptions struct {
	// SudoPass is the password used for steps marked become: true.
	// Plumbed straight through to the executor — no validation /
	// mutual-exclusion checks here (the saved-plan path predates the
	// CLI password-method mutex; preserve that).
	SudoPass string

	// LogLevel mirrors Config.LogLevel — "debug", "info" (default),
	// or "error". Unknown values default to info.
	LogLevel string

	// MaxPlanAge / AllowStale are the spec-16 stale-plan policy.
	// Plans older than MaxPlanAge are rejected unless AllowStale is
	// true. Zero MaxPlanAge disables the age check.
	MaxPlanAge time.Duration
	AllowStale bool
}

// NewRunnerFromPlan constructs a Runner that loads + validates a
// saved plan from disk, then executes it. The plan is validated
// against the spec-16 stale-plan policy (host facts, source-file
// hashes, max age) unless AllowStale is true.
//
// Run() returns the same *KernelResult shape as the config-path
// Runner so MCP / agent loop / future SDK consumers can compose
// uniformly across input sources.
//
// R1.1c follow-up to R1.1a: the saved-plan apply path used to live
// in cmd/mooncake.go's runFromPlan; this lifts it next to the
// config-path Runner.
func NewRunnerFromPlan(planPath string, opts FromPlanOptions) *Runner {
	return &Runner{
		fromPlan:     true,
		fromPlanPath: planPath,
		fromPlanOpts: opts,
	}
}

// runFromPlan dispatches the saved-plan path. Shape mirrors
// Runner.Run's config-path body but with a tighter subscriber set
// and executor.ExecutePlanWithCapture instead of executor.Start.
func (r *Runner) runFromPlan(_ context.Context) (*KernelResult, error) {
	planData, loadErr := plan.LoadPlanFromFile(r.fromPlanPath)
	if loadErr != nil {
		err := fmt.Errorf("failed to load plan: %w", loadErr)
		return failedResult(err), err
	}

	// Spec 16 stale-plan policy.
	validateOpts := plan.ValidateOptions{
		MaxAge:     r.fromPlanOpts.MaxPlanAge,
		AllowStale: r.fromPlanOpts.AllowStale,
	}
	if err := plan.ValidateForApply(planData, validateOpts); err != nil {
		wrapped := fmt.Errorf("refusing to apply stale plan: %w (use --allow-stale to override)", err)
		return failedResult(wrapped), wrapped
	}

	publisher := events.NewPublisher()
	defer publisher.Close()

	// Event-tail capture so the *KernelResult.Events field is
	// populated, same as the config-path Runner.Run.
	tail := newCaptureSubscriber()
	publisher.Subscribe(tail)

	level := parseLogLevel(r.fromPlanOpts.LogLevel)

	// Saved-plan apply renders only as text. Pre-extraction
	// runFromPlan hardcoded this; keep the contract.
	publisher.Subscribe(logger.NewConsoleSubscriber(level, outputFormatText))

	// Record run history (best-effort) keyed by the plan file the
	// user pointed at, mirroring the pre-extraction behavior.
	publisher.Subscribe(logger.NewRunLogSubscriber(r.fromPlanPath))

	// Structured JSON errors to stderr on step failures — matches
	// the config-path Runner so consumers get a consistent surface.
	publisher.Subscribe(logger.NewStderrErrorSubscriber())

	// Minimal logger for internal use (errors, etc.)
	internalLog := logger.NewLogger(level)

	// F020: signal handling lives in the CLI caller — same contract as
	// the config-path Runner. Embedded callers cancel ctx via their
	// own shutdown paths.

	capture := &executor.RunCapture{}

	execErr := executor.ExecutePlanWithCapture(
		planData,
		r.fromPlanOpts.SudoPass,
		actions.ModeApply,
		internalLog,
		publisher,
		capture,
	)

	// Drain pending events so the audit tail is complete before we
	// build the KernelResult.
	publisher.Flush()

	// The from-plan path skips compilation, so capture.Plan() may
	// still be nil — populate it from planData so the KernelResult
	// carries the executed plan like the config-path Runner does.
	result := assembleResult(capture, tail, execErr)
	if result.Plan == nil {
		result.Plan = planData
	}
	return result, execErr
}
