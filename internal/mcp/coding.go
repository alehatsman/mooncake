package mcp

// coding.go — MCP tools that are the wire form of the sdk.CodingBackend
// interface (#145). Four tools bridge the JSON-RPC surface to local OS
// + kernel operations:
//
//   read_file        — raw byte read from a path (base64-encoded response)
//   grep_files       — walk a directory and return line matches (RE2)
//   glob_files       — return paths matching a glob pattern
//   run_plan_inline  — run an inline YAML/JSON plan (no temp file required)

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
)

// readFileMaxBytes caps the single read_file response at 16 MiB.
const readFileMaxBytes int64 = 16 << 20

// HandleReadFile reads bytes from a path and returns a JSON envelope with
// base64-encoded content so binary files round-trip cleanly over JSON-RPC.
//
// Response shape:
//
//	{"path":"...","size":N,"encoding":"base64","content":"<b64>"}
func HandleReadFile(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path   string `json:"path"`
		Offset int64  `json:"offset"`
		Limit  int64  `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("read_file: parse args: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("read_file: path parameter required")
	}

	f, err := os.Open(params.Path) // #nosec G304 -- caller-supplied path is intentional
	if err != nil {
		return "", fmt.Errorf("read_file: open: %w", err)
	}
	defer func() { _ = f.Close() }()

	if params.Offset > 0 {
		if _, err := f.Seek(params.Offset, io.SeekStart); err != nil {
			return "", fmt.Errorf("read_file: seek: %w", err)
		}
	}

	limit := readFileMaxBytes
	if params.Limit > 0 && params.Limit < limit {
		limit = params.Limit
	}
	buf := make([]byte, limit)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("read_file: read: %w", err)
	}
	data := buf[:n]

	b, err := json.Marshal(map[string]any{
		"path":     params.Path,
		"size":     n,
		"encoding": "base64",
		"content":  base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return "", fmt.Errorf("read_file: marshal: %w", err)
	}
	return string(b), nil
}

// HandleGrepFiles walks a directory tree searching for RE2-pattern matches
// and returns a JSON array of matching lines.
//
// Response shape:
//
//	{"matches":[{"path":"...","line":N,"content":"..."}]}
func HandleGrepFiles(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Pattern         string   `json:"pattern"`
		Dir             string   `json:"dir"`
		Extensions      []string `json:"extensions"`
		MaxResults      int      `json:"max_results"`
		CaseInsensitive bool     `json:"case_insensitive"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("grep_files: parse args: %w", err)
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("grep_files: pattern parameter required")
	}

	flags := ""
	if params.CaseInsensitive {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + params.Pattern)
	if err != nil {
		return "", fmt.Errorf("grep_files: invalid pattern: %w", err)
	}

	dir := params.Dir
	if dir == "" {
		if dir, err = os.Getwd(); err != nil {
			return "", fmt.Errorf("grep_files: getwd: %w", err)
		}
	}

	extSet := make(map[string]struct{}, len(params.Extensions))
	for _, e := range params.Extensions {
		extSet["."+strings.TrimPrefix(e, ".")] = struct{}{}
	}

	type match struct {
		Path    string `json:"path"`
		Line    int    `json:"line"`
		Content string `json:"content"`
	}
	var matches []match

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if len(extSet) > 0 {
			if _, ok := extSet[filepath.Ext(path)]; !ok {
				return nil
			}
		}
		if params.MaxResults > 0 && len(matches) >= params.MaxResults {
			return filepath.SkipAll
		}

		f, openErr := os.Open(path) // #nosec G304 G122 -- path from WalkDir
		if openErr != nil {
			return nil
		}
		defer func() { _ = f.Close() }()

		scanner := bufio.NewScanner(f)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if re.MatchString(line) {
				matches = append(matches, match{Path: path, Line: lineNo, Content: line})
				if params.MaxResults > 0 && len(matches) >= params.MaxResults {
					return filepath.SkipAll
				}
			}
		}
		return scanner.Err()
	})
	if err != nil {
		return "", fmt.Errorf("grep_files: walk: %w", err)
	}

	if matches == nil {
		matches = []match{}
	}
	b, err := json.Marshal(map[string]any{"matches": matches})
	if err != nil {
		return "", fmt.Errorf("grep_files: marshal: %w", err)
	}
	return string(b), nil
}

// HandleGlobFiles returns paths matching a glob pattern.
//
// Response shape:
//
//	{"paths":["..."]}
func HandleGlobFiles(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Pattern string `json:"pattern"`
		Dir     string `json:"dir"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("glob_files: parse args: %w", err)
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("glob_files: pattern parameter required")
	}

	pat := params.Pattern
	if params.Dir != "" {
		pat = filepath.Join(params.Dir, pat)
	}
	paths, err := filepath.Glob(pat)
	if err != nil {
		return "", fmt.Errorf("glob_files: glob: %w", err)
	}
	if paths == nil {
		paths = []string{}
	}
	b, err := json.Marshal(map[string]any{"paths": paths})
	if err != nil {
		return "", fmt.Errorf("glob_files: marshal: %w", err)
	}
	return string(b), nil
}

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
