package apply

import (
	"context"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/ops"
	"github.com/alehatsman/mooncake/internal/plan"
)

// InMemoryPlanOptions configures a Runner that executes a pre-built
// *plan.Plan without reading from disk. Used by callers that build the
// plan themselves and want to bypass the config-file → planner → run
// pipeline — notably `mooncake task <name>`, which builds the plan
// with TaskName set on PlannerConfig and then hands the result here.
//
// The saved-plan path (NewRunnerFromPlan) does staleness validation
// because plans on disk may have drifted from the host's state since
// they were written. An in-memory plan was built moments ago in the
// same process, so it cannot be stale by construction — this path
// skips that check.
type InMemoryPlanOptions struct {
	// SudoPass is the password used for steps marked become: true.
	// Plumbed straight through to the executor.
	SudoPass string

	// LogLevel mirrors Config.LogLevel — "debug", "info" (default),
	// or "error". Unknown values default to info.
	LogLevel string

	// OutputFormat selects the built-in renderer: "text", "json",
	// "agent", or "quiet". Empty defaults to "text" (preserving the
	// historic task-runner behaviour). SDK callers default it to
	// "quiet" so an embedded run stays silent.
	OutputFormat string

	// StreamStepOutput mirrors Config.StreamStepOutput: render captured
	// step stdout/stderr regardless of LogLevel. `mooncake task <name>`
	// sets this true so shell steps stream by default without flipping
	// the noisier internal debug logs on.
	StreamStepOutput bool

	// OpID, when non-empty, links this in-memory apply to a row in
	// ops.jsonl (spec-68 wave 2). Same semantics as Config.OpID on
	// the config-path Runner — minted by the CLI before invocation.
	OpID string

	// RootFile is a human-readable label that flows into the runlog
	// entry and op-linkage record. Typically the absolute path of the
	// config the plan was built from (mooncake.yml, tasks.yml). The
	// in-memory path has no other source for this — without it, the
	// runlog entry shows an empty file reference.
	RootFile string

	// Policy, if non-nil, is the per-run permissions-as-contract gate
	// enforced at preflight — same semantics as Config.Policy on the
	// config-path Runner. nil enforces nothing.
	Policy *executor.Policy

	// Registry resolves handlers for dispatch. nil uses the
	// process-wide global. Set it to run a plan built on custom typed
	// actions without mutating the global.
	Registry *actions.Registry

	// ExtraSubscribers receive every kernel event in order under the
	// run's lifecycle. Subscribed before the built-in sinks. The run
	// owns their lifecycle via the publisher (the caller must not
	// Close them itself).
	ExtraSubscribers []events.Subscriber
}

// NewRunnerFromInMemoryPlan constructs a Runner that executes a plan
// the caller already built in-process. Mirrors NewRunnerFromPlan's
// shape so downstream consumers of *KernelResult don't have to branch
// on input source. Skips the spec-16 staleness validation (an
// in-memory plan cannot be stale — it was just built).
//
// Intended consumer: `mooncake task <name>`, which:
//  1. builds the plan via plan.NewPlanner().BuildPlan(PlannerConfig{TaskName: ...})
//  2. hands the result here for execution
//
// Apply itself stays unaware of tasks — task semantics live entirely
// in the planner's input transformation.
func NewRunnerFromInMemoryPlan(p *plan.Plan, opts InMemoryPlanOptions) *Runner {
	return &Runner{
		inMemoryPlan:     true,
		inMemoryPlanData: p,
		inMemoryPlanOpts: opts,
	}
}

// runFromInMemoryPlan executes the caller-supplied plan. Subscriber
// fan-out mirrors runFromPlan's set: console (text by default),
// stderr-error, optional runlog or enriched op-linkage. SDK callers
// set OutputFormat="quiet" and supply their own ExtraSubscribers so
// the run is silent and the event channel carries the signal. We
// deliberately do not install the first-run-hint subscriber here —
// that's a config-path affordance for new users.
func (r *Runner) runFromInMemoryPlan(ctx context.Context) (*KernelResult, error) {
	opts := r.inMemoryPlanOpts

	publisher := events.NewPublisher()
	defer publisher.Close()

	// ExtraSubscribers first — they see every event, including
	// plan.loaded, which fires before the built-in sinks are wired.
	for _, sub := range opts.ExtraSubscribers {
		publisher.Subscribe(sub)
	}

	tail := newCaptureSubscriber()
	publisher.Subscribe(tail)

	level := parseLogLevel(opts.LogLevel)

	outputFormat := opts.OutputFormat
	if outputFormat == "" {
		outputFormat = outputFormatText
	}

	switch outputFormat {
	case outputFormatQuiet:
		publisher.Subscribe(logger.NewQuietSubscriber())
	default:
		publisher.Subscribe(logger.NewConsoleSubscriber(level, outputFormat, opts.StreamStepOutput))
	}

	var runID string
	if opts.OpID != "" {
		runID = ops.NewRunID()
	} else {
		publisher.Subscribe(logger.NewRunLogSubscriber(opts.RootFile))
	}

	publisher.Subscribe(logger.NewStderrErrorSubscriber())

	internalLog := internalLogger(outputFormat, level)
	capture := &executor.RunCapture{}

	execErr := executor.ExecutePlanFull(
		ctx,
		r.inMemoryPlanData,
		opts.SudoPass,
		actions.ModeApply,
		internalLog,
		publisher,
		capture,
		opts.Policy,
		opts.Registry,
	)

	publisher.Flush()

	for _, sub := range opts.ExtraSubscribers {
		sub.Close()
	}

	if opts.OpID != "" {
		writeEnrichedRunlog(
			filepath.Base(opts.RootFile),
			opts.OpID,
			runID,
			tail,
			capture,
		)
	}

	result := assembleResult(capture, tail, execErr)
	if result.Plan == nil {
		result.Plan = r.inMemoryPlanData
	}
	return result, execErr
}
