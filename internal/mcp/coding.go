package mcp

// coding.go — the MCP tool that bridges the JSON-RPC surface to the
// kernel:
//
//   run_plan_inline  — run an inline YAML/JSON plan (no temp file required)
//
// read_file / grep_files / glob_files used to live here too, as the wire
// form of sdk.CodingBackend (#145). They were removed in #177: every
// agent harness that connects this server already ships native file
// read, grep, and glob, so re-exposing them only spent tool-description
// context and competed for the model's attention. The CodingBackend
// interface itself is unaffected — it just no longer has an MCP
// projection.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
)

// HandleRunPlanInline compiles and runs a YAML/JSON plan from an inline
// string — no temp file required. Returns the same JSON envelope as
// run_plan so callers can swap between the two without changing their
// response parsing.
func HandleRunPlanInline(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		YAML   string     `json:"yaml"`
		Policy *policyArg `json:"policy"`
	}
	if err := json.Unmarshal(args, &params); err != nil || params.YAML == "" {
		return "", fmt.Errorf("run_plan_inline: yaml parameter required")
	}
	return runConfigBytes(ctx, []byte(params.YAML), params.Policy.toExecutorPolicy())
}

// runConfigBytes mirrors runConfig but compiles from bytes instead of a
// file path. Used by run_plan_inline and the sdk.RemoteBackend mutation path.
func runConfigBytes(ctx context.Context, data []byte, policy *executor.Policy) (string, error) {
	planner, err := plan.NewPlanner()
	if err != nil {
		return "", err
	}
	planData, err := planner.BuildPlanFromBytes(data, plan.PlannerConfig{})
	if err != nil {
		return "", err
	}
	inspections, _ := executor.InspectPlanWithRegistry(planData, "", logger.NewDiscardLogger(), nil)
	inspIdx := buildInspectionIndex(inspections)

	kr, runErr := apply.NewRunnerFromInMemoryPlan(planData, apply.InMemoryPlanOptions{
		OutputFormat: "quiet",
		Policy:       policy,
	}).Run(ctx)

	durationByID := make(map[string]int64, len(kr.Steps))
	for _, ev := range kr.Events {
		if d, ok := ev.Data.(events.StepCompletedData); ok {
			durationByID[d.StepID] = d.DurationMs
		}
	}

	steps := make([]stepResult, 0, len(kr.Steps))
	for _, sr := range kr.Steps {
		res := stepResult{
			Name:       sr.Step.Name,
			Action:     sr.Step.DetermineActionType(),
			DurationMs: durationByID[sr.Step.ID],
		}
		if sr.Result != nil {
			res.Changed = sr.Result.Changed
			res.Failed = sr.Result.Failed
			res.Skipped = sr.Result.Skipped
			if sr.Result.Failed && sr.Result.Reason != "" {
				res.Error = sr.Result.Reason
			}
		}
		if insp, ok := inspIdx[sr.Step.ID]; ok {
			res.Diff = insp.Diff
			res.Cost = insp.Cost
			res.WouldChange = insp.WouldChange
		}
		steps = append(steps, res)
	}

	result := map[string]interface{}{
		"changed":     kr.Summary.Changed,
		"ok":          kr.Summary.Ok,
		"skipped":     kr.Summary.Skipped,
		"failed":      kr.Summary.Failed,
		"duration_ms": kr.Summary.DurationMs,
		"steps":       steps,
	}
	if reqs := aggregatePermissions(kr.Plan); reqs != nil {
		result["requires"] = reqs
	}
	if costSum := aggregateCost(inspections); costSum != nil {
		result["cost_summary"] = costSum
	}
	if runErr != nil {
		result["error"] = runErr.Error()
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}
