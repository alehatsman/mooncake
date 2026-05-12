package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/metrics"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/alehatsman/mooncake/internal/snapshot"
)

// ---- schema helpers ---------------------------------------------------------

func strProp(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

func boolProp(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "boolean", "description": desc}
}

func objSchema(props map[string]interface{}, required []string) map[string]interface{} {
	s := map[string]interface{}{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// ---- tool definitions -------------------------------------------------------

// AllTools returns the complete list of tool definitions.
func AllTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "get_facts",
			Description: "Return system facts as JSON (OS, CPU, memory, installed tools, etc.)",
			InputSchema: objSchema(nil, nil),
		},
		{
			Name:        "get_snapshot",
			Description: "Return a compact machine state summary",
			InputSchema: objSchema(map[string]interface{}{
				"format": strProp("Output format: 'text' (default) or 'json'"),
			}, nil),
		},
		{
			Name:        "fact_query",
			Description: "Query a specific fact by dot-path key (e.g. go_version, python_version)",
			InputSchema: objSchema(map[string]interface{}{
				"query": strProp("Dot-path key to look up (use _ not . between segments)"),
			}, []string{"query"}),
		},
		{
			Name:        "run_plan",
			Description: "Run a mooncake config file and return structured results",
			InputSchema: objSchema(map[string]interface{}{
				"config":  strProp("Path to mooncake config YAML file"),
				"dry_run": boolProp("If true, simulate without making changes"),
			}, []string{"config"}),
		},
		{
			Name:        "check_plan",
			Description: "Dry-run a mooncake config file and return what would change",
			InputSchema: objSchema(map[string]interface{}{
				"config": strProp("Path to mooncake config YAML file"),
			}, []string{"config"}),
		},
		{
			Name:        "get_metrics",
			Description: "Return live system metrics (CPU/GPU/memory/load/network) as JSON. Cached per-metric with TTLs ~2-5s. Use fields=[...] to restrict the response; use refresh=true to force re-sample.",
			InputSchema: objSchema(map[string]interface{}{
				"fields": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Optional. Restrict the response to these keys (e.g. ['cpu_usage_pct', 'gpus_metrics']). Adds a _collected_at sibling map of key → RFC3339 timestamp.",
				},
				"refresh": boolProp("If true, force-refresh metrics (bypass TTL)."),
			}, nil),
		},
	}
}

// ---- handlers ---------------------------------------------------------------

func HandleGetFacts(_ context.Context, _ json.RawMessage) (string, error) {
	f := facts.Collect()
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal facts: %w", err)
	}
	return string(b), nil
}

func HandleGetSnapshot(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Format string `json:"format"`
	}
	if args != nil {
		_ = json.Unmarshal(args, &params)
	}

	f := facts.Collect()
	snap := snapshot.CollectSystem(f)

	if params.Format == "json" {
		b, err := snap.RenderJSON()
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return snap.RenderText(0), nil
}

func HandleGetMetrics(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Fields  []string `json:"fields"`
		Refresh bool     `json:"refresh"`
	}
	if args != nil {
		_ = json.Unmarshal(args, &params)
	}

	if params.Refresh {
		metrics.Refresh()
	}

	m, collectedAt, _ := metrics.Collect(params.Fields)
	mm := m.ToMap()

	payload := mm
	if len(params.Fields) > 0 {
		payload = make(map[string]interface{}, len(params.Fields)+1)
		for _, k := range params.Fields {
			payload[k] = mm[k]
		}
		ts := make(map[string]string, len(collectedAt))
		for k, v := range collectedAt {
			ts[k] = v.UTC().Format(time.RFC3339)
		}
		payload["_collected_at"] = ts
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal metrics: %w", err)
	}
	return string(b), nil
}

func HandleFactQuery(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &params); err != nil || params.Query == "" {
		return "", fmt.Errorf("query parameter required")
	}

	f := facts.Collect()
	m := f.ToMap()
	key := strings.ReplaceAll(params.Query, ".", "_")
	val, ok := m[key]
	if !ok || val == nil || val == "" || val == false {
		return `{"found":false}`, nil
	}

	b, err := json.Marshal(map[string]interface{}{"found": true, "value": val})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// stepResult is a single step entry in run_plan / check_plan output.
type stepResult struct {
	Name       string `json:"name"`
	Action     string `json:"action"`
	Changed    bool   `json:"changed,omitempty"`
	WouldChange bool   `json:"would_change,omitempty"`
	Skipped    bool   `json:"skipped,omitempty"`
	Failed     bool   `json:"failed,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
}

// runCollector subscribes to run events and collects step results.
type runCollector struct {
	steps []stepResult
	stats struct {
		Changed    int
		Ok         int
		Skipped    int
		Failed     int
		DurationMs int64
	}
}

func (c *runCollector) OnEvent(ev events.Event) {
	switch ev.Type {
	case events.EventStepCompleted:
		d, ok := ev.Data.(events.StepCompletedData)
		if !ok {
			return
		}
		c.steps = append(c.steps, stepResult{
			Name:       d.Name,
			Action:     "",
			Changed:    d.Changed,
			DurationMs: d.DurationMs,
		})

	case events.EventStepSkipped:
		d, ok := ev.Data.(events.StepSkippedData)
		if !ok {
			return
		}
		c.steps = append(c.steps, stepResult{Name: d.Name, Skipped: true})

	case events.EventStepFailed:
		d, ok := ev.Data.(events.StepFailedData)
		if !ok {
			return
		}
		c.steps = append(c.steps, stepResult{
			Name:       d.Name,
			Failed:     true,
			DurationMs: d.DurationMs,
			Error:      d.ErrorMessage,
		})

	case events.EventRunCompleted:
		d, ok := ev.Data.(events.RunCompletedData)
		if !ok {
			return
		}
		c.stats.Changed = d.ChangedSteps
		c.stats.Ok = d.SuccessSteps - d.ChangedSteps
		if c.stats.Ok < 0 {
			c.stats.Ok = 0
		}
		c.stats.Skipped = d.SkippedSteps
		c.stats.Failed = d.FailedSteps
		c.stats.DurationMs = d.DurationMs
	}
}

func (c *runCollector) Close() {}

func runConfig(configPath string) (string, error) {
	publisher := events.NewPublisher()
	defer publisher.Close()

	col := &runCollector{}
	publisher.Subscribe(col)

	internalLog := logger.NewTestLogger()

	runErr := executor.Start(executor.StartConfig{
		ConfigFilePath: configPath,
	}, internalLog, publisher)

	result := map[string]interface{}{
		"changed":     col.stats.Changed,
		"ok":          col.stats.Ok,
		"skipped":     col.stats.Skipped,
		"failed":      col.stats.Failed,
		"duration_ms": col.stats.DurationMs,
		"steps":       col.steps,
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

func HandleRunPlan(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(args, &params); err != nil || params.Config == "" {
		return "", fmt.Errorf("config parameter required")
	}
	return runConfig(params.Config)
}

// HandleCheckPlan builds the plan and inspects it without applying.
// Returns per-step would-change predictions plus the host/hash metadata
// callers need to decide whether to apply.
func HandleCheckPlan(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(args, &params); err != nil || params.Config == "" {
		return "", fmt.Errorf("config parameter required")
	}

	planner, err := plan.NewPlanner()
	if err != nil {
		return "", err
	}
	planData, err := planner.BuildPlan(plan.PlannerConfig{ConfigPath: params.Config})
	if err != nil {
		return "", err
	}
	inspections, err := executor.InspectPlan(planData, "", logger.NewTestLogger())
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]interface{}{
		"root_file":   planData.RootFile,
		"generated_on": planData.GeneratedOn,
		"inspections": inspections,
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
