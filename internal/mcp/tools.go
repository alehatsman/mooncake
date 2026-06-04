package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/explain"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/metrics"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/alehatsman/mooncake/internal/snapshot"
)

// RegisterAllTools registers every tool returned by AllTools() on srv with
// the package's default handlers. Both `mooncake mcp` (stdio) and the daemon's
// /v1/mcp endpoint use this so the tool surface stays in one place.
func RegisterAllTools(srv *Server) {
	for _, def := range AllTools() {
		switch def.Name {
		case "get_facts":
			srv.RegisterTool(def, HandleGetFacts)
		case "get_snapshot":
			srv.RegisterTool(def, HandleGetSnapshot)
		case "fact_query":
			srv.RegisterTool(def, HandleFactQuery)
		case "run_plan":
			srv.RegisterTool(def, HandleRunPlan)
		case "check_plan":
			srv.RegisterTool(def, HandleCheckPlan)
		case "get_metrics":
			srv.RegisterTool(def, HandleGetMetrics)
		case "query_file":
			srv.RegisterTool(def, HandleQueryFile)
		case "explain":
			srv.RegisterTool(def, HandleExplain)
		case "list_actions":
			srv.RegisterTool(def, HandleListActions)
		case "describe_action":
			srv.RegisterTool(def, HandleDescribeAction)
		case "list_presets":
			srv.RegisterTool(def, HandleListPresets)
		case "read_file":
			srv.RegisterTool(def, HandleReadFile)
		case "grep_files":
			srv.RegisterTool(def, HandleGrepFiles)
		case "glob_files":
			srv.RegisterTool(def, HandleGlobFiles)
		case "run_plan_inline":
			srv.RegisterTool(def, HandleRunPlanInline)
		case "fleet_list_peers":
			srv.RegisterTool(def, HandleListPeers)
		case "fleet_run_plan":
			srv.RegisterTool(def, HandleFleetRunPlan)
		}
	}
}

// ---- schema helpers ---------------------------------------------------------

func strProp(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

func boolProp(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "boolean", "description": desc}
}

func objSchema(props map[string]interface{}, required []string) map[string]interface{} {
	if props == nil {
		props = map[string]interface{}{}
	}
	s := map[string]interface{}{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func strArrayProp(desc string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": "string"},
		"description": desc,
	}
}

// policyProp is the JSON-Schema for the optional permissions-as-contract
// gate (#11) on run_plan. The agent (or the operator wiring the MCP
// client) declares what the run may do; the executor refuses any step
// that exceeds it, before its side effect. Omit for an ungated run.
func policyProp() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"description": "Optional permissions-as-contract gate (#11): refuse steps that exceed the declared limits, before any side effect. Omit to run ungated.",
		"properties": map[string]interface{}{
			"allowed_actions": strArrayProp("Allowlist of action types the run may use (e.g. ['file.write','text.replace']). Empty = any action allowed unless denied."),
			"denied_actions":  strArrayProp("Denylist of action types the run may NOT use (e.g. ['shell','cmd']). Wins over the allowlist."),
			"deny_network":    boolProp("Refuse any step that declares network egress (pkg install, download, http.request, remote git clone)."),
			"max_risk":        map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 10, "description": "Refuse any step whose estimated risk band (1..10) exceeds this cap. 0 = no cap."},
		},
	}
}

// policyArg is the wire shape of the run_plan `policy` argument; it
// mirrors executor.Policy with snake_case JSON keys for MCP clients.
type policyArg struct {
	AllowedActions []string `json:"allowed_actions"`
	DeniedActions  []string `json:"denied_actions"`
	DenyNetwork    bool     `json:"deny_network"`
	MaxRisk        int      `json:"max_risk"`
}

// toExecutorPolicy lowers the wire shape into an *executor.Policy,
// returning nil when no field is set so an empty/absent policy object is
// treated identically to "no policy" (ungated run).
func (p *policyArg) toExecutorPolicy() *executor.Policy {
	if p == nil {
		return nil
	}
	pol := &executor.Policy{
		AllowedActions: p.AllowedActions,
		DeniedActions:  p.DeniedActions,
		DenyNetwork:    p.DenyNetwork,
		MaxRisk:        p.MaxRisk,
	}
	if pol.IsZero() {
		return nil
	}
	return pol
}

// ---- tool definitions -------------------------------------------------------

// AllTools returns the complete list of tool definitions.
func AllTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "get_facts",
			Description: "Inspect the user's machine before you touch it — OS, CPU, memory, installed tools, package manager. Returns JSON. Read-only.",
			InputSchema: objSchema(nil, nil),
		},
		{
			Name:        "get_snapshot",
			Description: "One-glance summary of the user's machine state — same view a human sees from `mooncake snapshot`. Use it to orient yourself before proposing changes. Read-only.",
			InputSchema: objSchema(map[string]interface{}{
				"format": strProp("Output format: 'text' (default) or 'json'"),
			}, nil),
		},
		{
			Name:        "fact_query",
			Description: "Look up one fact about the user's machine by dotted key (e.g. `go_version`, `os_distribution`). Cheaper than `get_facts` when you only need a single value. Read-only.",
			InputSchema: objSchema(map[string]interface{}{
				"query": strProp("Dot-path key to look up (use _ not . between segments)"),
			}, []string{"query"}),
		},
		{
			Name:        "run_plan",
			Description: "Apply a mooncake config to the user's system. Every step is typed and rolls back automatically on failure — safer than running raw shell. Mutates the system; preview with `check_plan` first.",
			InputSchema: objSchema(map[string]interface{}{
				"config":  strProp("Path to mooncake config YAML file"),
				"dry_run": boolProp("If true, simulate without making changes"),
				"policy":  policyProp(),
			}, []string{"config"}),
		},
		{
			Name:        "check_plan",
			Description: "Preview what a mooncake config will do before you let it run — per-step structured diff, no side effects. Run this before `run_plan`.",
			InputSchema: objSchema(map[string]interface{}{
				"config": strProp("Path to mooncake config YAML file"),
			}, []string{"config"}),
		},
		{
			Name:        "get_metrics",
			Description: "Sample the user's live system metrics (CPU, GPU, memory, load, network) as JSON — what `top` and `nvidia-smi` show right now. Cached 2–5s per metric; pass `fields=[...]` to narrow the response or `refresh=true` to bypass the cache. Read-only.",
			InputSchema: objSchema(map[string]interface{}{
				"fields": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Optional. Restrict the response to these keys (e.g. ['cpu_usage_pct', 'gpus_metrics']). Adds a _collected_at sibling map of key → RFC3339 timestamp.",
				},
				"refresh": boolProp("If true, force-refresh metrics (bypass TTL)."),
			}, nil),
		},
		{
			Name:        "query_file",
			Description: "Read a value out of a JSON or YAML file on disk — give it a path and a dotted query, get `{found, value, format}` back. Mirrors the `mooncake query` CLI. Path syntax: dotted keys (`a.b.c`) and bracketed integer indices (`a[0]`). Read-only.",
			InputSchema: objSchema(map[string]interface{}{
				"path":      strProp("Absolute or relative path to the file"),
				"query":     strProp("Optional dotted path to extract. Empty returns the whole document."),
				"format":    strProp("Optional format override: 'json' or 'yaml'. Default: auto-detect from extension."),
				"max_bytes": map[string]interface{}{"type": "integer", "description": "Optional. Refuse to load files larger than this size in bytes. Default: 4194304."},
			}, []string{"path"}),
		},
		{
			Name:        "explain",
			Description: "Look up typed information about a mooncake noun — an action verb (e.g. `pkg.install`), a run id (`r/...`), a resource handle (`file:/path`, `pkg:apt/name`, `user:name`, `service:unit`), or an operation id (`op/...`). Returns the typed schema, applicable examples, the Diff and Reverse shapes, and a typed `not_found` with candidates when the noun does not resolve. Read-only.",
			InputSchema: objSchema(map[string]interface{}{
				"noun": strProp("One of: an action verb (e.g. 'pkg.install'); a run id ('r/...'); a resource handle ('<kind>:<id>'); an op id ('op/...')."),
				"examples_limit": map[string]interface{}{
					"type":        "integer",
					"minimum":     0,
					"maximum":     explainExamplesLimitMax,
					"description": "Cap on example excerpts returned for kind:action. Default 3.",
				},
			}, []string{"noun"}),
		},
		// agent proposal-01: discovery tools. Each is a thin wrapper
		// over an existing internal API — list_actions over
		// actions.List(), describe_action over the same Registry +
		// schemagen pipeline that backs `mooncake actions show`, and
		// list_presets over presets.DiscoverAllPresets. Together they
		// close the "agents can't introspect the action surface
		// without out-of-band knowledge" gap surfaced in proposal-01.
		{
			Name:        "list_actions",
			Description: "List every action this mooncake build supports — name, category, platforms, and the spec-22 ABI capability matrix (check/diff/cost/reverse/permissions). Optional `category` filter narrows the result to a single category (e.g. `file`, `system`, `network`). Returns JSON. Read-only.",
			InputSchema: objSchema(map[string]interface{}{
				"category": strProp("Optional filter: only return actions in this category (e.g. 'file', 'system'). Omit for the full inventory."),
			}, nil),
		},
		{
			Name:        "describe_action",
			Description: "Show one action's typed contract — required + optional parameters, default values, JSON-Schema constraints, and the same capability matrix `list_actions` returns per row. Same data the CLI's `mooncake actions show <name>` surfaces, exposed to MCP clients so they don't need to shell out. Returns JSON. Read-only.",
			InputSchema: objSchema(map[string]interface{}{
				"name": strProp("Action verb (e.g. 'file.copy', 'pkg.install'). See `list_actions` for the full set."),
			}, []string{"name"}),
		},
		{
			Name:        "list_presets",
			Description: "List every preset discoverable on this host — built-in, registry-cloned, and local. Each entry carries name + description + version + source + path so an agent can decide which to plan with before running it. Returns JSON. Read-only.",
			InputSchema: objSchema(nil, nil),
		},
		// CodingBackend wire-form tools (#145). These four tools are the JSON-RPC
		// surface of the sdk.CodingBackend interface — swapping the backend
		// implementation on the daemon side needs no prompt change.
		{
			Name:        "read_file",
			Description: "Read raw bytes from a file on the daemon host. Returns base64-encoded content in a JSON envelope so binary files round-trip safely. Read-only.",
			InputSchema: objSchema(map[string]interface{}{
				"path":   strProp("Absolute or relative path to read"),
				"offset": map[string]interface{}{"type": "integer", "description": "Byte offset to start from. 0 or omitted means start of file."},
				"limit":  map[string]interface{}{"type": "integer", "description": "Maximum bytes to return. 0 or omitted means up to 16 MiB."},
			}, []string{"path"}),
		},
		{
			Name:        "grep_files",
			Description: "Walk a directory tree and return lines matching a RE2 pattern. Equivalent to sdk.Grep — direct filesystem walk, no executor. Read-only.",
			InputSchema: objSchema(map[string]interface{}{
				"pattern":          strProp("RE2 regular expression to match against each line"),
				"dir":              strProp("Root directory to search. Defaults to the daemon's working directory."),
				"extensions":       strArrayProp("Only search files with these extensions (without leading dot, e.g. 'go', 'ts'). Empty means all files."),
				"max_results":      map[string]interface{}{"type": "integer", "description": "Cap on number of matches returned. 0 means no cap."},
				"case_insensitive": boolProp("Match case-insensitively."),
			}, []string{"pattern"}),
		},
		{
			Name:        "glob_files",
			Description: "Return paths matching a glob pattern on the daemon host. Equivalent to sdk.Glob — direct filepath.Glob call, no executor. Read-only.",
			InputSchema: objSchema(map[string]interface{}{
				"pattern": strProp("Glob pattern (e.g. '*.go', 'src/**/*.ts')"),
				"dir":     strProp("Base directory to resolve the pattern against. Omit to use pattern as-is."),
			}, []string{"pattern"}),
		},
		{
			Name:        "run_plan_inline",
			Description: "Compile and run a mooncake YAML/JSON plan supplied as a string — no temp file required. Returns the same JSON envelope as run_plan. Use check_plan for a dry-run preview first. Mutates the system.",
			InputSchema: objSchema(map[string]interface{}{
				"yaml":   strProp("Inline YAML or JSON plan content"),
				"policy": policyProp(),
			}, []string{"yaml"}),
		},
		{
			Name:        "fleet_list_peers",
			Description: "List every peer configured in peers.toml — name, addr, transport, tags, and roles. Read-only. Use this before fleet_run_plan to see which machines are available.",
			InputSchema: objSchema(map[string]interface{}{
				"peers_file": strProp("Optional path to peers.toml. Defaults to $XDG_CONFIG_HOME/mooncake/peers.toml."),
			}, nil),
		},
		{
			Name:        "fleet_run_plan",
			Description: "Apply a mooncake config across the fleet (one or more remote peers). Drives `mooncake fleet apply` under the hood — uploads the plan, runs it on each peer in parallel, and returns per-peer outcomes + a summary. Mutates remote systems; use fleet_list_peers first to confirm targets.",
			InputSchema: objSchema(map[string]interface{}{
				"config":     strProp("Path to mooncake config YAML file"),
				"peers_file": strProp("Optional path to peers.toml. Defaults to $XDG_CONFIG_HOME/mooncake/peers.toml."),
				"peers":      strArrayProp("Optional peer name filter. Omit to target all peers. Each entry is an exact peer name from peers.toml."),
				"vars_file":  strArrayProp("Optional vars files (relative or absolute paths). Same semantics as --vars-file on the CLI."),
				"parallel":   map[string]interface{}{"type": "integer", "description": "Max number of peers to apply to concurrently. 0 = default (all peers)."},
			}, []string{"config"}),
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
	Name        string                `json:"name"`
	Action      string                `json:"action,omitempty"`
	Changed     bool                  `json:"changed,omitempty"`
	WouldChange bool                  `json:"would_change,omitempty"`
	Skipped     bool                  `json:"skipped,omitempty"`
	Failed      bool                  `json:"failed,omitempty"`
	DurationMs  int64                 `json:"duration_ms,omitempty"`
	Error       string                `json:"error,omitempty"`
	Diff        *actions.Diff         `json:"diff,omitempty"`
	Cost        *actions.CostEstimate `json:"cost,omitempty"`
}

// aggregatePermissions walks the plan steps and merges each handler's declared
// PermissionSet into a single plan-level summary. Returns nil when no step
// declares any permissions.
func aggregatePermissions(p *plan.Plan) *actions.PermissionSet {
	var merged actions.PermissionSet
	anySet := false
	for i := range p.Steps {
		step := &p.Steps[i]
		actionType := step.DetermineActionType()
		h, ok := actions.Get(actionType)
		if !ok {
			continue
		}
		perm, ok2 := h.(actions.Permitter)
		if !ok2 {
			continue
		}
		ps := perm.Permissions(step)
		if ps.Sudo {
			merged.Sudo = true
			anySet = true
		}
		if ps.Network {
			merged.Network = true
			anySet = true
		}
		if len(ps.RequiredBinaries) > 0 {
			merged.RequiredBinaries = appendUnique(merged.RequiredBinaries, ps.RequiredBinaries)
			anySet = true
		}
		if len(ps.FilesystemWrite) > 0 {
			merged.FilesystemWrite = append(merged.FilesystemWrite, ps.FilesystemWrite...)
			anySet = true
		}
		if len(ps.Notes) > 0 {
			merged.Notes = append(merged.Notes, ps.Notes...)
			anySet = true
		}
	}
	if !anySet {
		return nil
	}
	return &merged
}

// appendUnique appends elements from src to dst, skipping duplicates.
func appendUnique(dst, src []string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, s := range dst {
		seen[s] = struct{}{}
	}
	for _, s := range src {
		if _, ok := seen[s]; !ok {
			dst = append(dst, s)
			seen[s] = struct{}{}
		}
	}
	return dst
}

// aggregateCost summarises a slice of step inspections into a plan-level
// cost object. Returns nil when no step has cost information.
func aggregateCost(inspections []plan.StepInspection) map[string]interface{} {
	maxRisk := 0
	totalResources := 0
	wouldChange := 0
	hasCost := false
	for _, ins := range inspections {
		if ins.Cost == nil {
			continue
		}
		hasCost = true
		if ins.Cost.Risk > maxRisk {
			maxRisk = ins.Cost.Risk
		}
		if ins.Cost.Resources > 0 {
			totalResources += ins.Cost.Resources
		}
		if ins.WouldChange {
			wouldChange++
		}
	}
	if !hasCost {
		return nil
	}
	band := "low"
	switch {
	case maxRisk >= 7:
		band = "high"
	case maxRisk >= 4:
		band = "medium"
	}
	return map[string]interface{}{
		"max_risk":           maxRisk,
		"risk_band":          band,
		"total_resources":    totalResources,
		"would_change_count": wouldChange,
	}
}

// buildInspectionIndex returns a map from StepID to StepInspection for quick
// lookup when merging inspection data into apply-mode step results.
func buildInspectionIndex(inspections []plan.StepInspection) map[string]plan.StepInspection {
	idx := make(map[string]plan.StepInspection, len(inspections))
	for _, ins := range inspections {
		idx[ins.StepID] = ins
	}
	return idx
}

func runConfig(ctx context.Context, configPath string, policy *executor.Policy) (string, error) {
	// Pre-inspect: collect predicted Diff + Cost per step (ModePlan).
	// Runs before the apply so run_plan can return per-step diffs alongside
	// apply-time outcomes. Step IDs are deterministic (step-0001, etc.) so
	// the inspection index matches the Runner's compiled plan.
	planner, err := plan.NewPlanner()
	if err != nil {
		return "", err
	}
	planData, err := planner.BuildPlan(plan.PlannerConfig{ConfigPath: configPath})
	if err != nil {
		return "", err
	}
	// InspectPlan logs are noise to the MCP caller; errors are non-fatal
	// because the predicted diff/cost is a UX nicety — if it fails we'd
	// rather the apply proceed than block on the prediction.
	inspections, _ := executor.InspectPlan(planData, "", logger.NewDiscardLogger())
	inspIdx := buildInspectionIndex(inspections)

	// Run via apply.Runner — wires publisher, event capture, and result
	// assembly internally. OutputFormat "quiet" suppresses console output
	// (MCP callers consume the returned JSON, not stdout).
	kr, runErr := apply.NewRunner(&apply.Config{
		ConfigPath:   configPath,
		OutputFormat: "quiet",
		Policy:       policy,
	}).Run(ctx)

	// Extract per-step duration from the event audit trail; executor.Result
	// does not carry DurationMs directly.
	durationByID := make(map[string]int64, len(kr.Steps))
	for _, ev := range kr.Events {
		if d, ok := ev.Data.(events.StepCompletedData); ok {
			durationByID[d.StepID] = d.DurationMs
		}
	}

	// Map KernelResult.Steps to the MCP stepResult shape.
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

func HandleRunPlan(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Config string     `json:"config"`
		Policy *policyArg `json:"policy"`
	}
	if err := json.Unmarshal(args, &params); err != nil || params.Config == "" {
		return "", fmt.Errorf("config parameter required")
	}
	return runConfig(ctx, params.Config, params.Policy.toExecutorPolicy())
}

// explainExamplesLimitMax is the upper bound advertised in the
// `explain` tool's inputSchema (`maximum: 10`) and enforced by
// HandleExplain. Lifted to a package-level constant so the schema
// literal and the handler check share one source of truth — F044
// shipped because the cap existed in the schema but not in the
// handler.
const explainExamplesLimitMax = 10

// HandleExplain resolves a noun (action verb, run id, resource handle, op id)
// and returns the typed payload as indented JSON. Mirrors `mooncake explain
// <noun> --format json` over MCP. Read-only — delegates to explain.Resolve,
// which reads only ~/.mooncake/{runs,ops}.jsonl, the embedded schema, and the
// in-tree actions registry.
func HandleExplain(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Noun          string `json:"noun"`
		ExamplesLimit int    `json:"examples_limit"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if strings.TrimSpace(params.Noun) == "" {
		return "", fmt.Errorf("noun parameter required")
	}
	// F044: the inputSchema advertises minimum:0 / maximum:10 for
	// examples_limit. Pre-fix nothing on the wire path enforced
	// either bound — examples_limit:-1 silently fell back to the
	// default 3, and examples_limit:1000 returned 1000 excerpts.
	// Reject loudly so MCP clients that don't pre-validate the
	// schema can't blow past the cap. Matches the codebase's
	// argument-validation style (see "noun parameter required"
	// above) — schema violations are errors, not silent clamps.
	if params.ExamplesLimit < 0 {
		return "", fmt.Errorf("examples_limit must be >= 0")
	}
	if params.ExamplesLimit > explainExamplesLimitMax {
		return "", fmt.Errorf("examples_limit must be <= %d", explainExamplesLimitMax)
	}

	result := explain.Resolve(params.Noun, explain.Options{
		ExamplesLimit: params.ExamplesLimit,
	})

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return "", fmt.Errorf("marshal explain result: %w", err)
	}
	return strings.TrimRight(buf.String(), "\n"), nil
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
	inspections, err := executor.InspectPlan(planData, "", logger.NewDiscardLogger())
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"root_file":    planData.RootFile,
		"generated_on": planData.GeneratedOn,
		"inspections":  inspections,
	}
	if reqs := aggregatePermissions(planData); reqs != nil {
		result["requires"] = reqs
	}
	if costSum := aggregateCost(inspections); costSum != nil {
		result["cost_summary"] = costSum
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return "", err
	}
	return buf.String(), nil
}
