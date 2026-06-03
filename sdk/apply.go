package mooncake

// apply.go is the SDK's execution surface (#122): run a typed plan with no
// LLM in the loop. Two entry points sit on the same kernel the CLI uses:
//
//   - Apply   — execute a config against the local machine and return a
//               typed *ApplyResult (the kernel's "what just happened":
//               per-step outcomes, the audit event tail, summary counters).
//               This is the "I already have a plan, just run it with my
//               actions and stream me events" entry. No reasoning, no model.
//
//   - Plan    — compile + inspect a config in non-mutating plan mode and
//               return the typed plan with per-step predictions (would-change,
//               structural diff, cost estimate). A dry-run preview with NO
//               side effects.
//
// Both honour a consumer-owned Registry, so an external caller can run a
// config built on its own custom typed actions without touching the global.

import (
	"context"
	"fmt"

	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/plan"
)

// ----------------------------------------------------------------------------
// Result re-exports
// ----------------------------------------------------------------------------

// ApplyResult is the kernel's typed "what just happened" shape returned by
// Apply: the compiled plan, per-step outcomes, the full audit event tail, and
// aggregate counters. It is never nil — on a pre-plan failure it carries a
// populated Summary with Success=false and a non-empty ErrorMessage.
type ApplyResult = apply.KernelResult

// StepResult bundles a step's typed input with the executor's typed outcome.
// One entry per executed step, in execution order, on ApplyResult.Steps.
type StepResult = apply.StepResult

// RunSummary aggregates run-wide counters (ok/changed/skipped/failed/…) plus
// wall-clock duration and success. Carried on ApplyResult.Summary.
type RunSummary = apply.RunSummary

// PlanResult is the compiled, inspected plan returned by Plan: the typed steps
// plus per-step Inspections (would-change, diff, cost). Plan fills Inspections;
// no side effects are performed.
type PlanResult = plan.Plan

// StepInspection is a single step's plan-mode prediction: whether applying it
// would change state, the handler-supplied reason, and — for handlers that
// implement Differ / Coster — the structural diff and cost estimate.
type StepInspection = plan.StepInspection

// ----------------------------------------------------------------------------
// Apply — execute a config with no LLM
// ----------------------------------------------------------------------------

// ApplyOptions configures a no-LLM apply via Apply.
type ApplyOptions struct {
	// ConfigPath is the root config file to run (required).
	ConfigPath string

	// VarsFiles are variable-override files, lowest-to-highest precedence
	// (later files win on key collision). Read exactly as the CLI's
	// `-v a.yml -v b.yml`.
	VarsFiles []string

	// Tags / SkipTags / Names filter steps at plan-build time. Empty means
	// no filtering. Names is the step-name filter (AND'd with Tags).
	Tags     []string
	SkipTags []string
	Names    []string

	// Registry resolves handlers for both planning and dispatch. nil uses
	// the process-wide global (built-ins). Set it to run a config built on
	// custom typed actions without mutating the global.
	Registry *Registry

	// Policy, if non-nil, is the per-run permissions-as-contract gate
	// enforced at preflight — an action allow/deny list, a network switch,
	// and a risk cap. A step that exceeds it fails the run before its side
	// effect. nil enforces nothing.
	Policy *Policy

	// Subscribers receive every event the kernel emits, in order, including
	// the run lifecycle. They are subscribed before the built-in sinks; the
	// run owns their lifecycle via the publisher (the caller must not Close
	// them itself).
	Subscribers []Subscriber

	// LogLevel gates the built-in console subscriber's verbosity:
	// "debug", "info", or "error". Empty defaults to "error" so an embedded
	// run stays quiet and the caller's Subscribers carry the signal.
	LogLevel string

	// OutputFormat selects the built-in renderer: "text", "json", "agent",
	// or "quiet". Empty defaults to "quiet" — no console noise; the caller's
	// Subscribers are the event channel.
	OutputFormat string
}

// Apply executes the config at opts.ConfigPath against the local machine with
// no LLM in the loop and returns the typed *ApplyResult. ctx cancellation is
// observed between steps (an in-flight step runs to completion; the run then
// returns ctx.Err()). The result is never nil even on error.
func Apply(ctx context.Context, opts ApplyOptions) (*ApplyResult, error) {
	outputFormat := opts.OutputFormat
	if outputFormat == "" {
		outputFormat = "quiet"
	}
	logLevel := opts.LogLevel
	if logLevel == "" {
		logLevel = "error"
	}

	cfg := &apply.Config{
		ConfigPath:       opts.ConfigPath,
		VarsFiles:        opts.VarsFiles,
		Tags:             opts.Tags,
		SkipTags:         opts.SkipTags,
		Names:            opts.Names,
		OutputFormat:     outputFormat,
		LogLevel:         logLevel,
		Policy:           opts.Policy,
		Registry:         opts.Registry,
		ExtraSubscribers: opts.Subscribers,
	}
	return apply.NewRunner(cfg).Run(ctx)
}

// ----------------------------------------------------------------------------
// Plan — dry-run preview with no side effects
// ----------------------------------------------------------------------------

// PlanOptions configures a dry-run preview via Plan.
type PlanOptions struct {
	// ConfigPath is the root config file to compile (required).
	ConfigPath string

	// VarsFiles are variable-override files, lowest-to-highest precedence.
	// Read as-is (resolve "~" yourself); inline Vars below override them.
	VarsFiles []string

	// Vars are inline variables overlaid on top of VarsFiles (highest
	// precedence). Useful when the caller holds values in memory rather
	// than on disk.
	Vars map[string]interface{}

	// Tags / SkipTags / Names filter steps at plan-build time.
	Tags     []string
	SkipTags []string
	Names    []string

	// Registry resolves handlers for plan-time checks and check-mode
	// dispatch. nil uses the process-wide global. Set it so a plan built on
	// custom typed actions surfaces those handlers' diffs/costs.
	Registry *Registry
}

// Plan compiles the config at opts.ConfigPath and runs it in non-mutating plan
// mode, returning the typed *PlanResult with per-step Inspections filled in
// (would-change, structural diff, cost estimate). No side effects are
// performed — this is a preview, not an apply.
func Plan(_ context.Context, opts PlanOptions) (*PlanResult, error) {
	variables, err := loadPlanVariables(opts.VarsFiles, opts.Vars)
	if err != nil {
		return nil, err
	}

	planner, err := plan.NewPlanner()
	if err != nil {
		return nil, fmt.Errorf("create planner: %w", err)
	}

	compiled, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: opts.ConfigPath,
		Variables:  variables,
		Tags:       opts.Tags,
		SkipTags:   opts.SkipTags,
		Names:      opts.Names,
		Registry:   opts.Registry,
	})
	if err != nil {
		return nil, fmt.Errorf("build plan: %w", err)
	}

	// Plan mode → no mutations, no sudo. Thread the consumer registry so
	// custom-action handlers' Differ/Coster output appears in the preview.
	inspections, err := executor.InspectPlanWithRegistry(compiled, "", NewDiscardLogger(), opts.Registry)
	if err != nil {
		return nil, fmt.Errorf("inspect plan: %w", err)
	}
	compiled.Inspections = inspections
	return compiled, nil
}

// loadPlanVariables merges variable files (lowest-to-highest precedence) and
// overlays the inline map on top (highest). Returns a non-nil map.
func loadPlanVariables(varsFiles []string, inline map[string]interface{}) (map[string]interface{}, error) {
	variables := make(map[string]interface{})
	for _, path := range varsFiles {
		if path == "" {
			continue
		}
		vars, err := config.ReadVariables(path)
		if err != nil {
			return nil, fmt.Errorf("read vars file %s: %w", path, err)
		}
		for k, v := range vars {
			variables[k] = v
		}
	}
	for k, v := range inline {
		variables[k] = v
	}
	return variables, nil
}
