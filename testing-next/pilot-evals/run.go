// Command pilot-evals runs the mooncake pilot evaluation harness.
//
// For each (goal, snapshot, assertions) tuple in goals/, the runner
// builds a system+user prompt via internal/pilot.BuildPrompt, calls
// internal/pilot/llm.NewClient().GeneratePlan, then runs the goal's
// assertions against the returned plan YAML. Reports pass/fail per
// goal plus a summary.
//
// Gated on MOONCAKE_PILOT_EVAL=1 so accidental invocations do not
// burn API tokens. Use -dry-run to validate the harness shape
// without contacting an LLM.
//
// See spec-67 §14 (docs-working/streams/agent/specs/spec-67-mooncake-pilot.md)
// and testing-next/pilot-evals/README.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/pilot"
	"github.com/alehatsman/mooncake/internal/pilot/llm"
	"github.com/alehatsman/mooncake/testing-next/pilot-evals/assertions"
	"gopkg.in/yaml.v3"
)

type goalFile struct {
	Path       string   // resolved path on disk; not in YAML
	Goal       string   `yaml:"goal"`
	Snapshot   string   `yaml:"snapshot"`
	Provider   string   `yaml:"provider"`
	Model      string   `yaml:"model"`
	Assertions []string `yaml:"assertions"`
}

func main() {
	var (
		goalsDir = flag.String("goals", "testing-next/pilot-evals/goals", "directory of goal YAML files")
		snapsDir = flag.String("snapshots", "testing-next/pilot-evals/snapshots", "directory of snapshot JSON files")
		dryRun   = flag.Bool("dry-run", false, "validate goals, snapshots, and assertions without calling an LLM")
		only     = flag.String("only", "", "run only the goal whose basename (without extension) matches this string")
		timeout  = flag.Duration("timeout", 90*time.Second, "per-goal LLM call timeout")
	)
	flag.Parse()

	if !*dryRun && os.Getenv("MOONCAKE_PILOT_EVAL") != "1" {
		fmt.Fprintln(os.Stderr, "refusing to run: set MOONCAKE_PILOT_EVAL=1 to opt in, or pass -dry-run to validate the harness without an LLM call")
		os.Exit(2)
	}

	goals, err := loadGoals(*goalsDir)
	if err != nil {
		fail("load goals: %v", err)
	}
	if *only != "" {
		goals = filterGoals(goals, *only)
		if len(goals) == 0 {
			fail("no goal matched -only=%q", *only)
		}
	}
	if len(goals) == 0 {
		fail("no goal files found under %s", *goalsDir)
	}

	var client llm.Client
	if !*dryRun {
		c, err := llm.NewClient()
		if err != nil {
			fail("no LLM client available: %v", err)
		}
		client = c
	}

	var passed, failed int
	for _, g := range goals {
		ok := runOne(g, *snapsDir, client, *dryRun, *timeout)
		if ok {
			passed++
		} else {
			failed++
		}
	}

	fmt.Println()
	fmt.Printf("summary: %d passed, %d failed (of %d)\n", passed, failed, len(goals))
	if failed > 0 {
		os.Exit(1)
	}
}

func runOne(g goalFile, snapsDir string, client llm.Client, dryRun bool, timeout time.Duration) bool {
	name := strings.TrimSuffix(filepath.Base(g.Path), filepath.Ext(g.Path))
	fmt.Printf("=== %s\n", name)
	fmt.Printf("    goal:     %s\n", firstLine(g.Goal))
	fmt.Printf("    snapshot: %s\n", g.Snapshot)
	fmt.Printf("    provider: %s / %s\n", g.Provider, g.Model)

	// Parse assertions first — if a goal file has a typo, we want to
	// catch it before paying for an LLM call.
	checks, err := assertions.ParseAll(g.Assertions)
	if err != nil {
		fmt.Printf("    FAIL: %v\n", err)
		return false
	}

	snapPath := filepath.Join(snapsDir, g.Snapshot+".json")
	snapshotBytes, err := os.ReadFile(snapPath) // #nosec G304 -- harness-controlled path
	if err != nil {
		fmt.Printf("    FAIL: read snapshot %s: %v\n", snapPath, err)
		return false
	}

	// Build the prompt — exercises the real prompt builder so prompt
	// regressions surface here.
	_, _, err = pilot.BuildPrompt(pilot.PlanInput{Goal: g.Goal, Snapshot: snapshotBytes})
	if err != nil {
		fmt.Printf("    FAIL: build prompt: %v\n", err)
		return false
	}

	if dryRun {
		fmt.Printf("    dry-run OK (%d assertions parsed, prompt built)\n", len(checks))
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	sys, usr, _ := pilot.BuildPrompt(pilot.PlanInput{Goal: g.Goal, Snapshot: snapshotBytes})
	t0 := time.Now()
	planYAML, err := client.GeneratePlan(ctx, sys, usr, g.Model)
	dt := time.Since(t0)
	if err != nil {
		fmt.Printf("    FAIL: GeneratePlan: %v\n", err)
		return false
	}
	fmt.Printf("    plan emitted in %s (%d bytes)\n", dt.Round(time.Millisecond), len(planYAML))

	steps, err := assertions.ParsePlan(planYAML)
	if err != nil {
		// A parse failure is itself an assertion failure for schema_valid;
		// but other assertions can't meaningfully run. Report and stop.
		fmt.Printf("    FAIL: %v\n", err)
		return false
	}

	allPassed := true
	for _, a := range checks {
		if err := a.Check(planYAML, steps); err != nil {
			fmt.Printf("    FAIL %s: %v\n", a.String(), err)
			allPassed = false
		} else {
			fmt.Printf("    OK   %s\n", a.String())
		}
	}
	return allPassed
}

func loadGoals(dir string) ([]goalFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []goalFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(p) // #nosec G304 -- harness-controlled path
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var g goalFile
		if err := yaml.Unmarshal(b, &g); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		g.Path = p
		if g.Goal == "" {
			return nil, fmt.Errorf("%s: missing 'goal' field", p)
		}
		if g.Snapshot == "" {
			return nil, fmt.Errorf("%s: missing 'snapshot' field", p)
		}
		if len(g.Assertions) == 0 {
			return nil, fmt.Errorf("%s: at least one assertion required", p)
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func filterGoals(goals []goalFile, only string) []goalFile {
	var out []goalFile
	for _, g := range goals {
		name := strings.TrimSuffix(filepath.Base(g.Path), filepath.Ext(g.Path))
		if name == only || strings.Contains(name, only) {
			out = append(out, g)
		}
	}
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "pilot-evals: "+format+"\n", args...)
	os.Exit(1)
}
