package apply_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/plan"
	_ "github.com/alehatsman/mooncake/internal/register" // register handlers
)

// TestRunnerFromInMemoryPlan_ExecutesPlan verifies that a plan built
// in-process and handed to NewRunnerFromInMemoryPlan executes the
// same way as the config-path Runner — same result shape, real file
// writes, populated Steps/Events.
//
// This is the contract `mooncake task <name>` depends on: build the
// plan with PlannerConfig{TaskName: ...}, hand it here, get back a
// fully populated *KernelResult.
func TestRunnerFromInMemoryPlan_ExecutesPlan(t *testing.T) {
	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "hello.txt")

	// Write a config; build the plan in-process; hand it to the
	// in-memory runner. The plan uses TaskName to pick "demo" so
	// the test also exercises the task→steps splice end-to-end.
	cfgPath := filepath.Join(tmp, "tasks.yml")
	body := `version: "1"
tasks:
  demo:
    steps:
      - name: write hello
        file.write:
          path: ` + targetPath + `
          state: file
          content: "hello from in-memory plan\n"
          mode: "0644"
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	planner, err := plan.NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	planData, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: cfgPath,
		TaskName:   "demo",
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	opts := apply.InMemoryPlanOptions{
		LogLevel: "error",
		RootFile: cfgPath,
	}
	result, err := apply.NewRunnerFromInMemoryPlan(planData, opts).Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("Run returned nil *KernelResult on success")
	}

	// The plan we passed in must round-trip through assembleResult so
	// downstream consumers can inspect what actually executed.
	if result.Plan == nil {
		t.Errorf("KernelResult.Plan = nil; want the in-memory plan attached")
	}

	// Confirm the file was actually written — proves we executed, not
	// merely planned.
	content, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read target file: %v", readErr)
	}
	if string(content) != "hello from in-memory plan\n" {
		t.Errorf("file content = %q, want %q", string(content), "hello from in-memory plan\n")
	}

	if len(result.Steps) == 0 {
		t.Errorf("KernelResult.Steps is empty; want >= 1 record")
	}
	if len(result.Events) < 3 {
		t.Errorf("KernelResult.Events length = %d; want >= 3 lifecycle events", len(result.Events))
	}
}

// buildFailingTaskPlan builds a task plan whose first and third steps fail and
// whose second and fourth succeed. Shaped like a CI gate: a standing failure
// early, real work behind it.
func buildFailingTaskPlan(t *testing.T, marker string) *plan.Plan {
	t.Helper()
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "tasks.yml")
	body := `version: "1"
tasks:
  gate:
    steps:
      - name: stage one fails
        shell:
          cmd: "exit 1"
      - name: stage two passes
        shell:
          cmd: "true"
      - name: stage three fails
        shell:
          cmd: "exit 1"
      - name: stage four writes
        shell:
          cmd: "touch ` + marker + `"
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	planner, err := plan.NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	planData, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: cfgPath,
		TaskName:   "gate",
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	return planData
}

// The #193 contract: `mooncake task <gate> --keep-going` runs every stage and
// reports all failures, instead of letting one standing failure hide the
// stages behind it. The run must still fail.
func TestRunnerFromInMemoryPlan_KeepGoingRunsEveryStep(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "reached-the-end")
	planData := buildFailingTaskPlan(t, marker)

	_, err := apply.NewRunnerFromInMemoryPlan(planData, apply.InMemoryPlanOptions{
		LogLevel:  "error",
		KeepGoing: true,
	}).Run(context.Background())

	// Still fails — keep-going changes when you learn, not whether it passed.
	if err == nil {
		t.Fatal("Run returned nil error; a run with two failing steps must fail")
	}
	// Both failures reported, not just the first.
	if !strings.Contains(err.Error(), "2 steps failed") {
		t.Errorf("error = %q, want it to report 2 failed steps", err.Error())
	}
	// The last step ran despite the failure in step one.
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("final step did not run under --keep-going: %v", statErr)
	}
}

// Without the flag the run still aborts at the first failure — keep-going is
// opt-in, and the dev-loop default stays fail-fast.
func TestRunnerFromInMemoryPlan_WithoutKeepGoingStopsAtFirstFailure(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "reached-the-end")
	planData := buildFailingTaskPlan(t, marker)

	_, err := apply.NewRunnerFromInMemoryPlan(planData, apply.InMemoryPlanOptions{
		LogLevel: "error",
	}).Run(context.Background())

	if err == nil {
		t.Fatal("Run returned nil error; a run with a failing step must fail")
	}
	if strings.Contains(err.Error(), "steps failed (run continued") {
		t.Errorf("error = %q, want a first-failure abort, not a deferred-failure summary", err.Error())
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("final step ran without --keep-going; the run should have aborted at step one")
	}
}
