package plan

// F024 regression: walkAndRender only handled map[string]string;
// map[string]interface{} fields (os.systemd.Service / Unit / Timer /
// Socket / Install, text.patch.json.Set / .Merge, text.patch.yaml.Set
// / .Merge, use.With) silently passed templates through. A
// playbook like
//
//   - vars: { binary_path: /usr/local/bin/myapp }
//   - os.systemd:
//       name: myapp.service
//       service:
//         ExecStart: "{{ binary_path }}"
//
// produced an OsSystemd step whose Service["ExecStart"] was the
// literal string "{{ binary_path }}", which the handler then wrote
// to the unit file verbatim. systemctl rejected the unit on load.
// The fix extends walkAndRender to unwrap interface{} values and
// render string-valued entries.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanner_RendersOsSystemdServiceSection(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	body := `version: "1.0"
vars:
  binary_path: /usr/local/bin/myapp
steps:
  - name: render systemd service
    os.systemd:
      name: myapp.service
      service:
        ExecStart: "{{ binary_path }}"
        Restart: on-failure
`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.OsSystemd == nil {
		t.Fatal("OsSystemd is nil; planner dropped the action")
	}
	got, ok := step.OsSystemd.Service["ExecStart"]
	if !ok {
		t.Fatal("Service[ExecStart] missing from planned step")
	}
	if got != "/usr/local/bin/myapp" {
		t.Errorf("Service[ExecStart] = %q, want %q (pre-fix: literal %q passed through)",
			got, "/usr/local/bin/myapp", "{{ binary_path }}")
	}

	// Non-templated entries must still be preserved unchanged.
	if step.OsSystemd.Service["Restart"] != "on-failure" {
		t.Errorf("Service[Restart] = %v, want \"on-failure\"", step.OsSystemd.Service["Restart"])
	}
}

// Non-string entries (numbers, bools) must pass through without
// reflect panics. F024's fix only touches string-valued map entries;
// anything else is left as-is so a future test for typed entries
// can be added without churning this one.
func TestPlanner_OsSystemdNonStringValuesPreserved(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	body := `version: "1.0"
steps:
  - name: mixed types
    os.systemd:
      name: myapp.service
      service:
        ExecStart: /bin/true
        TimeoutStopSec: 30
        Restart: on-failure
`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	svc := plan.Steps[0].OsSystemd.Service
	if svc["ExecStart"] != "/bin/true" {
		t.Errorf("ExecStart = %v", svc["ExecStart"])
	}
	if svc["TimeoutStopSec"] != 30 {
		t.Errorf("TimeoutStopSec = %v (kind=%T); want 30 preserved as int", svc["TimeoutStopSec"], svc["TimeoutStopSec"])
	}
}
