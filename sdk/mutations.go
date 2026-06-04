package mooncake

// mutations.go — single-step mutation helpers: Edit / Write / Exec (#144).
//
// Each helper synthesizes one typed step and dispatches via ApplySteps — NOT a
// parallel code path. Every mutation therefore inherits Diff / Reverse capture /
// Policy gate / audit event from the kernel for free.
//
// PlanConfig / PlanSteps are the inline plan-mode counterparts: same kernel,
// no side effects, per-step Inspections filled in.

import (
	"context"
	"fmt"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/plan"
)

// Write writes content to path as a single file.write step. The mutation goes
// through the full kernel funnel: typed Diff, Reverse capture, Policy gate,
// and audit event. It is NOT a direct os.WriteFile call.
func Write(ctx context.Context, path string, content []byte, opts ApplyOptions) (*ApplyResult, error) {
	step := Step{
		Name: "write " + path,
		FileWrite: &config.File{
			Path:    path,
			State:   "file",
			Content: string(content),
		},
	}
	return ApplySteps(ctx, []Step{step}, opts)
}

// Edit performs a literal string-replace on path as a single text.replace
// step. old and new are plain strings — not regexes. The mutation goes through
// the full kernel funnel: typed Diff, Reverse capture, Policy gate, and audit
// event. It is NOT a direct read-modify-write call.
func Edit(ctx context.Context, path, old, new string, opts ApplyOptions) (*ApplyResult, error) {
	step := Step{
		Name: "edit " + path,
		TextReplace: &config.FileReplace{
			Path:    path,
			Pattern: old,
			Replace: new,
			Flags:   &config.ReplaceFlags{Regex: false},
		},
	}
	return ApplySteps(ctx, []Step{step}, opts)
}

// Exec runs cmd in the default shell as a single shell step. The mutation goes
// through the full kernel funnel: typed Diff, Reverse capture, Policy gate,
// and audit event. It is NOT a direct exec.Command call.
func Exec(ctx context.Context, cmd string, opts ApplyOptions) (*ApplyResult, error) {
	step := Step{
		Name:  "exec",
		Shell: &config.ShellAction{Cmd: cmd},
	}
	return ApplySteps(ctx, []Step{step}, opts)
}

// PlanConfig compiles a pre-parsed *Config in plan mode and returns the typed
// *PlanResult with per-step Inspections (would-change, structural diff, cost
// estimate). No side effects are performed — this is a preview only.
// Inline analog of Plan; accepts a Config already in memory instead of a
// ConfigPath on disk.
func PlanConfig(_ context.Context, cfg *Config, opts PlanOptions) (*PlanResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("PlanConfig: cfg must not be nil")
	}

	variables, err := loadPlanVariables(opts.VarsFiles, opts.Vars)
	if err != nil {
		return nil, err
	}

	planner, err := plan.NewPlanner()
	if err != nil {
		return nil, fmt.Errorf("create planner: %w", err)
	}

	compiled, err := planner.BuildPlanFromConfig(cfg, plan.PlannerConfig{
		Variables: variables,
		Tags:      opts.Tags,
		SkipTags:  opts.SkipTags,
		Names:     opts.Names,
		Registry:  opts.Registry,
	})
	if err != nil {
		return nil, fmt.Errorf("build plan: %w", err)
	}

	inspections, err := executor.InspectPlanWithRegistry(compiled, "", NewDiscardLogger(), opts.Registry)
	if err != nil {
		return nil, fmt.Errorf("inspect plan: %w", err)
	}
	compiled.Inspections = inspections
	return compiled, nil
}

// PlanSteps compiles an in-memory step slice in plan mode and returns the
// typed *PlanResult with per-step Inspections (would-change, diff, cost). No
// side effects are performed. Inline analog of Plan; mirrors ApplySteps for
// the preview path.
func PlanSteps(ctx context.Context, steps []Step, opts PlanOptions) (*PlanResult, error) {
	return PlanConfig(ctx, &Config{Steps: steps}, opts)
}

// PlanBytes parses raw YAML (or JSON) bytes as a config, compiles it in plan
// mode, and returns the typed *PlanResult with per-step Inspections. No side
// effects are performed. Inline analog of Plan; mirrors ApplyBytes for the
// preview path.
func PlanBytes(_ context.Context, data []byte, opts PlanOptions) (*PlanResult, error) {
	variables, err := loadPlanVariables(opts.VarsFiles, opts.Vars)
	if err != nil {
		return nil, err
	}

	planner, err := plan.NewPlanner()
	if err != nil {
		return nil, fmt.Errorf("create planner: %w", err)
	}

	compiled, err := planner.BuildPlanFromBytes(data, plan.PlannerConfig{
		Variables: variables,
		Tags:      opts.Tags,
		SkipTags:  opts.SkipTags,
		Names:     opts.Names,
		Registry:  opts.Registry,
	})
	if err != nil {
		return nil, fmt.Errorf("build plan: %w", err)
	}

	inspections, err := executor.InspectPlanWithRegistry(compiled, "", NewDiscardLogger(), opts.Registry)
	if err != nil {
		return nil, fmt.Errorf("inspect plan: %w", err)
	}
	compiled.Inspections = inspections
	return compiled, nil
}
