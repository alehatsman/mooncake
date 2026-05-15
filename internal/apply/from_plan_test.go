package apply_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/plan"
)

// TestRunnerFromPlan_KernelResult_Shape pins R1.1c: the saved-plan
// path produces the same *KernelResult shape as the config-path
// Runner. Compiles a plan via the config-path Runner, writes it to
// disk, then runs it via NewRunnerFromPlan. Both paths must populate
// Plan / Steps / Summary identically.
func TestRunnerFromPlan_KernelResult_Shape(t *testing.T) {
	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "from-plan.txt")
	cfgPath := writeConfig(t, tmp, `
- name: write from saved plan
  file.write:
    path: `+targetPath+`
    state: file
    content: "from saved plan\n"
    mode: "0644"
`)

	// Step 1: compile a plan via the existing planner.
	planner, err := plan.NewPlanner()
	if err != nil {
		t.Fatalf("plan.NewPlanner: %v", err)
	}
	compiled, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: cfgPath,
	})
	if err != nil {
		t.Fatalf("planner.BuildPlan: %v", err)
	}

	// Step 2: write it to disk.
	planFile := filepath.Join(tmp, "saved-plan.yml")
	if err := plan.SavePlanToFile(compiled, planFile); err != nil {
		t.Fatalf("plan.SavePlanToFile: %v", err)
	}

	// Step 3: apply it via the from-plan path.
	result, err := apply.NewRunnerFromPlan(planFile, apply.FromPlanOptions{
		LogLevel:   "error",
		AllowStale: true, // tests run after compile; otherwise spec-16 may flag staleness
	}).Run(context.Background())
	if err != nil {
		t.Fatalf("RunnerFromPlan.Run: %v", err)
	}
	if result == nil {
		t.Fatal("RunnerFromPlan.Run returned nil *KernelResult on success")
	}

	// Same shape promise as the config-path Runner.
	if result.Plan == nil {
		t.Error("KernelResult.Plan = nil; want the loaded plan")
	}
	if len(result.Steps) == 0 {
		t.Error("KernelResult.Steps is empty; want at least 1 record")
	}
	if !result.Summary.Success {
		t.Errorf("Summary.Success = false; want true on a clean run")
	}
	if result.Summary.TotalSteps == 0 {
		t.Error("Summary.TotalSteps = 0; want > 0")
	}
	if _, statErr := os.Stat(targetPath); statErr != nil {
		t.Errorf("apply did not create %s: %v", targetPath, statErr)
	}
}

// TestRunnerFromPlan_MissingFile_FailsCleanly ensures the LoadPlan
// error surfaces as a populated KernelResult (matches Run's
// invariant — never returns nil result, even on early-exit paths).
func TestRunnerFromPlan_MissingFile_FailsCleanly(t *testing.T) {
	result, err := apply.NewRunnerFromPlan("/nonexistent/plan.yml",
		apply.FromPlanOptions{LogLevel: "error"}).Run(context.Background())
	if err == nil {
		t.Fatal("expected error on missing plan file; got nil")
	}
	if result == nil {
		t.Fatal("KernelResult must not be nil even on error (invariant)")
	}
	if result.Summary.Success {
		t.Error("Summary.Success = true on error path; want false")
	}
	if result.Summary.ErrorMessage == "" {
		t.Error("Summary.ErrorMessage is empty on error path; want populated")
	}
}

// TestRunnerFromPlan_StalePlan_RejectsWithoutAllowStale verifies
// the spec-16 stale-plan policy still fires through the new entry
// point. AllowStale=true would let the apply proceed (covered by
// the shape test above); here we set AllowStale=false against a
// plan whose facts won't match (we tweak the saved file).
//
// We can't easily forge a host-fact mismatch from a unit test;
// instead this asserts the validator is wired by setting an
// impossibly small MaxPlanAge.
func TestRunnerFromPlan_StalePlan_RejectsWithoutAllowStale(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := writeConfig(t, tmp, `
- name: noop
  log:
    msg: hi
`)
	planner, err := plan.NewPlanner()
	if err != nil {
		t.Fatalf("plan.NewPlanner: %v", err)
	}
	compiled, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: cfgPath,
	})
	if err != nil {
		t.Fatalf("planner.BuildPlan: %v", err)
	}
	planFile := filepath.Join(tmp, "p.yml")
	if err := plan.SavePlanToFile(compiled, planFile); err != nil {
		t.Fatalf("plan.SavePlanToFile: %v", err)
	}

	// Force expiry by setting MaxPlanAge to 1ns — by the time the
	// test reads the plan, it's "old."
	result, err := apply.NewRunnerFromPlan(planFile, apply.FromPlanOptions{
		LogLevel:   "error",
		MaxPlanAge: 1, // 1 nanosecond
		AllowStale: false,
	}).Run(context.Background())

	if err == nil {
		t.Fatal("expected stale-plan rejection; got nil error")
	}
	if result == nil || result.Summary.Success {
		t.Errorf("stale-plan path should return non-nil KernelResult with Success=false")
	}
}

// guard against unused-imports in the off chance compile changes.
var _ = executor.RunCapture{}
