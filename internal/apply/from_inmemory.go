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
// fan-out mirrors runFromPlan's set: console (text), stderr-error,
// optional runlog or enriched op-linkage. We deliberately do not
// install the first-run-hint subscriber here — that's a config-path
// affordance for new users, and an in-memory apply is by definition
// a programmatic call from another command (the user already knows
// what they're doing).
func (r *Runner) runFromInMemoryPlan(ctx context.Context) (*KernelResult, error) {
	publisher := events.NewPublisher()
	defer publisher.Close()

	tail := newCaptureSubscriber()
	publisher.Subscribe(tail)

	level := parseLogLevel(r.inMemoryPlanOpts.LogLevel)

	// Match runFromPlan's text-only contract. Callers wanting JSON /
	// agent output should render the *KernelResult themselves; the
	// in-memory path's reason for being is "execute this plan", not
	// "negotiate output format".
	publisher.Subscribe(logger.NewConsoleSubscriber(level, outputFormatText, r.inMemoryPlanOpts.StreamStepOutput))

	var runID string
	if r.inMemoryPlanOpts.OpID != "" {
		runID = ops.NewRunID()
	} else {
		publisher.Subscribe(logger.NewRunLogSubscriber(r.inMemoryPlanOpts.RootFile))
	}

	publisher.Subscribe(logger.NewStderrErrorSubscriber())

	internalLog := logger.NewLogger(level)
	capture := &executor.RunCapture{}

	execErr := executor.ExecutePlanWithCapture(
		ctx,
		r.inMemoryPlanData,
		r.inMemoryPlanOpts.SudoPass,
		actions.ModeApply,
		internalLog,
		publisher,
		capture,
	)

	publisher.Flush()

	if r.inMemoryPlanOpts.OpID != "" {
		writeEnrichedRunlog(
			filepath.Base(r.inMemoryPlanOpts.RootFile),
			r.inMemoryPlanOpts.OpID,
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
