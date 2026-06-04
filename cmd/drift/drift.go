// Package drift implements the `mooncake drift` subcommand family.
// PR A (spec-58): local-only `drift inspect` that reads last-applied records
// from agentd's state dir and runs InspectPlan to surface conformance drift.
package drift

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/agentd"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/alehatsman/mooncake/internal/template"
)

// Command returns the top-level `drift` command with its subcommands.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "drift",
		Usage: "Inspect and report plan-conformance drift (spec-58)",
		Subcommands: []*cli.Command{
			inspectCommand(),
		},
	}
}

func inspectCommand() *cli.Command {
	return &cli.Command{
		Name:      "inspect",
		Usage:     "Compare the last-applied plan against current system state",
		ArgsUsage: "[scope]",
		Description: "Loads last-applied plan records from agentd's state dir " +
			"and runs InspectPlan to check whether re-applying would change state. " +
			"With no argument, all recorded scopes are inspected. " +
			"Pass a scope hash to inspect a specific record.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "state-dir",
				Usage: "Override agentd state dir (default: platform default)",
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Emit JSONL (one JSON object per scope) instead of the table",
			},
		},
		Action: inspectAction,
	}
}

// stepVerdict is the per-step result emitted in --json mode.
type stepVerdict struct {
	StepID      string `json:"step_id"`
	ActionType  string `json:"action_type,omitempty"`
	WouldChange bool   `json:"would_change"`
	Checkable   bool   `json:"checkable"`
	Skipped     bool   `json:"skipped,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// scopeVerdict is emitted once per scope in --json mode.
type scopeVerdict struct {
	Scope       string        `json:"scope"`
	PlanPath    string        `json:"plan_path"`
	BaseDir     string        `json:"base_dir,omitempty"`
	AppliedAt   time.Time     `json:"applied_at"`
	RunID       string        `json:"run_id"`
	Changed     int           `json:"changed"`
	Uncheckable int           `json:"uncheckable"`
	TotalSteps  int           `json:"total_steps"`
	Error       string        `json:"error,omitempty"`
	Steps       []stepVerdict `json:"steps,omitempty"`
}

func inspectAction(c *cli.Context) error {
	stateDir := c.String("state-dir")
	if stateDir == "" {
		cfg, err := agentd.Default(false)
		if err != nil {
			return fmt.Errorf("resolve state dir: %w", err)
		}
		stateDir = cfg.StateDir
	}

	var records []agentd.LastAppliedRecord
	if c.NArg() > 0 {
		scope := c.Args().First()
		rec, err := agentd.ReadLastApplied(stateDir, scope)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return cli.Exit(fmt.Sprintf("drift inspect: no record for scope %q in %s", scope, stateDir), 1)
			}
			return err
		}
		records = []agentd.LastAppliedRecord{*rec}
	} else {
		var err error
		records, err = agentd.ListLastApplied(stateDir)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			fmt.Fprintln(c.App.Writer, "drift inspect: no last-applied records found (has fleet apply been run?)")
			return nil
		}
	}

	log := logger.NewDiscardLogger()
	verdicts := make([]scopeVerdict, 0, len(records))
	for _, rec := range records {
		v := inspectRecord(rec, log)
		verdicts = append(verdicts, v)
	}

	if c.Bool("json") {
		return renderJSON(c, verdicts)
	}
	return renderTable(c, verdicts)
}

// inspectRecord builds the plan from rec's recorded parameters and runs
// InspectPlan. Returns a populated scopeVerdict with Error set on failure.
func inspectRecord(rec agentd.LastAppliedRecord, log logger.Logger) scopeVerdict {
	v := scopeVerdict{
		Scope:     rec.Scope,
		PlanPath:  rec.PlanPath,
		BaseDir:   rec.BaseDir,
		AppliedAt: rec.AppliedAt,
		RunID:     rec.RunID,
	}

	if _, err := os.Stat(rec.PlanPath); err != nil {
		v.Error = fmt.Sprintf("plan_path missing: %v", err)
		return v
	}

	// Load vars files the same way executor.Start does: resolve each path
	// against rec.BaseDir (or cwd as fallback), merge in order.
	variables, err := loadVarsFiles(rec.BaseDir, rec.VarsFiles)
	if err != nil {
		v.Error = fmt.Sprintf("load vars: %v", err)
		return v
	}

	plr, err := plan.NewPlanner()
	if err != nil {
		v.Error = fmt.Sprintf("create planner: %v", err)
		return v
	}

	// chdir to BaseDir so Node-style relative path resolution in the plan
	// matches the original apply's working directory.
	if rec.BaseDir != "" {
		prev, _ := os.Getwd()
		if err := os.Chdir(rec.BaseDir); err != nil {
			v.Error = fmt.Sprintf("chdir %s: %v", rec.BaseDir, err)
			return v
		}
		defer func() { _ = os.Chdir(prev) }()
	}

	p, err := plr.BuildPlan(plan.PlannerConfig{
		ConfigPath: rec.PlanPath,
		Variables:  variables,
		Tags:       rec.Tags,
	})
	if err != nil {
		v.Error = fmt.Sprintf("build plan: %v", err)
		return v
	}

	inspections, err := executor.InspectPlan(p, "", log)
	if err != nil {
		v.Error = fmt.Sprintf("inspect plan: %v", err)
		return v
	}

	v.TotalSteps = len(inspections)
	v.Steps = make([]stepVerdict, 0, len(inspections))
	for _, s := range inspections {
		sv := stepVerdict{
			StepID:      s.StepID,
			ActionType:  s.ActionType,
			WouldChange: s.WouldChange,
			Checkable:   s.Checkable,
			Skipped:     s.Skipped,
			Reason:      s.Reason,
		}
		v.Steps = append(v.Steps, sv)
		if s.WouldChange {
			v.Changed++
		}
		if !s.Checkable && !s.Skipped {
			v.Uncheckable++
		}
	}
	return v
}

// loadVarsFiles loads and merges vars files in order (later wins on collision).
// Paths are resolved relative to baseDir (or cwd when baseDir is empty).
func loadVarsFiles(baseDir string, paths []string) (map[string]any, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		return nil, err
	}
	expander := pathutil.NewPathExpander(renderer)

	anchor := baseDir
	if anchor == "" {
		anchor, _ = os.Getwd()
	}

	variables := make(map[string]any)
	for _, p := range paths {
		if p == "" {
			continue
		}
		expanded, err := expander.ExpandPath(p, anchor, nil)
		if err != nil {
			return nil, fmt.Errorf("expand vars path %q: %w", p, err)
		}
		vars, err := config.ReadVariables(expanded)
		if err != nil {
			// Mirror executor.Start's warn-and-continue behaviour.
			continue
		}
		for k, v := range vars {
			variables[k] = v
		}
	}
	return variables, nil
}

func renderTable(c *cli.Context, verdicts []scopeVerdict) error {
	w := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SCOPE\tPLAN\tAPPLIED\tCHANGED\tUNCHECK\tSTATUS")
	anyDrift := false
	for _, v := range verdicts {
		planDisplay := v.PlanPath
		if v.BaseDir != "" {
			if rel, err := filepath.Rel(v.BaseDir, v.PlanPath); err == nil {
				planDisplay = rel
			}
		}
		if v.Error != "" {
			fmt.Fprintf(w, "%s\t%s\t%s\t-\t-\terror: %s\n",
				v.Scope, planDisplay, v.AppliedAt.Format("2006-01-02T15:04Z"), v.Error)
			anyDrift = true
			continue
		}
		status := "ok"
		if v.Changed > 0 {
			status = "DRIFTED"
			anyDrift = true
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n",
			v.Scope, planDisplay, v.AppliedAt.Format("2006-01-02T15:04Z"),
			v.Changed, v.Uncheckable, status)
	}
	_ = w.Flush()

	changed := 0
	for _, v := range verdicts {
		changed += v.Changed
	}
	fmt.Fprintf(c.App.Writer, "drift inspect: %d/%d scopes drifted, %d step deltas total\n",
		driftedCount(verdicts), len(verdicts), changed)

	if anyDrift {
		return cli.Exit("", 1)
	}
	return nil
}

func renderJSON(c *cli.Context, verdicts []scopeVerdict) error {
	enc := json.NewEncoder(c.App.Writer)
	anyDrift := false
	for _, v := range verdicts {
		if err := enc.Encode(v); err != nil {
			return err
		}
		if v.Changed > 0 || v.Error != "" {
			anyDrift = true
		}
	}
	if anyDrift {
		return cli.Exit("", 1)
	}
	return nil
}

func driftedCount(verdicts []scopeVerdict) int {
	n := 0
	for _, v := range verdicts {
		if v.Changed > 0 || v.Error != "" {
			n++
		}
	}
	return n
}
