package apply_test

import (
	"context"
	"os"
	"path/filepath"
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
