package executor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
)

// silentLogger is a no-op logger for tests so InspectPlan output
// doesn't clutter test runs.
type silentLogger struct{}

func (silentLogger) Infof(string, ...interface{})       {}
func (silentLogger) Debugf(string, ...interface{})      {}
func (silentLogger) Errorf(string, ...interface{})      {}
func (silentLogger) Codef(string, ...interface{})       {}
func (silentLogger) Textf(string, ...interface{})       {}
func (silentLogger) Mooncake()                          {}
func (silentLogger) SetLogLevel(int)                    {}
func (silentLogger) SetLogLevelStr(string) error        { return nil }
func (silentLogger) WithPadLevel(int) logger.Logger     { return silentLogger{} }
func (silentLogger) LogStep(logger.StepInfo)            {}
func (silentLogger) Complete(logger.ExecutionStats)     {}
func (silentLogger) SetRedactor(logger.Redactor)        {}

// TestInspectPlan_FileStep verifies that InspectPlan reports an
// accurate would-change prediction for a file directory step against a
// non-existent target, and reports already-ok against an existing one.
//
// This is the user-facing payoff of Spec 16: `mooncake plan` shows what
// would actually change without making changes.
func TestInspectPlan_FileStep(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "wofi")
	existing := t.TempDir() // already exists with default mode

	p := &plan.Plan{
		Version:     "1.0",
		GeneratedAt: time.Now(),
		RootFile:    "<test>",
		Steps: []config.Step{
			{
				ID:   "step-0001",
				Name: "create missing dir",
				FileWrite: &config.File{
					Path:  missing,
					State: "directory",
					Mode:  "0755",
				},
			},
			{
				ID:   "step-0002",
				Name: "create existing dir",
				FileWrite: &config.File{
					Path:  existing,
					State: "directory",
					Mode:  "0755",
				},
			},
		},
		InitialVars: map[string]interface{}{},
	}

	// Force the existing dir to 0755 so the second inspection reports
	// already-ok.
	if err := os.Chmod(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := InspectPlan(p, "", silentLogger{})
	if err != nil {
		t.Fatalf("InspectPlan: %v", err)
	}

	if n := len(got); n != 2 {
		t.Fatalf("expected 2 inspections, got %d", n)
	}

	// step-0001: would create (missing target)
	if !got[0].Checkable {
		t.Errorf("step-0001 should be Checkable; got %+v", got[0])
	}
	if !got[0].WouldChange {
		t.Errorf("step-0001 should WouldChange (target missing); got %+v", got[0])
	}

	// step-0002: already ok (target exists, mode matches)
	if !got[1].Checkable {
		t.Errorf("step-0002 should be Checkable; got %+v", got[1])
	}
	if got[1].WouldChange {
		t.Errorf("step-0002 should NOT WouldChange (target already correct); got %+v", got[1])
	}

	// Neither should have created or modified anything on disk.
	if _, statErr := os.Stat(missing); !os.IsNotExist(statErr) {
		t.Errorf("InspectPlan must not create the missing target")
	}
}

// TestInspectPlan_TolerantWhenInPlanMode verifies that a step whose
// `when` expression can't be evaluated (because it references a
// registered result from a step that didn't run) is reported as
// "when unevaluable in plan mode" rather than aborting the whole
// inspection.
func TestInspectPlan_TolerantWhenInPlanMode(t *testing.T) {
	target := filepath.Join(t.TempDir(), "dir")
	p := &plan.Plan{
		Version:     "1.0",
		GeneratedAt: time.Now(),
		RootFile:    "<test>",
		Steps: []config.Step{
			{
				ID:   "step-0001",
				Name: "file step",
				FileWrite: &config.File{Path: target, State: "directory", Mode: "0755"},
			},
			{
				ID:   "step-0002",
				Name: "conditional",
				When: "{{ no_such_var.changed }}",
				FileWrite: &config.File{Path: target, State: "directory", Mode: "0755"},
			},
		},
		InitialVars: map[string]interface{}{},
	}

	got, err := InspectPlan(p, "", silentLogger{})
	if err != nil {
		t.Fatalf("InspectPlan should not error on unevaluable when: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 inspections, got %d", len(got))
	}
	// step-0002 should be marked skipped with the "when unevaluable"
	// reason since the variable isn't in scope at plan time.
	if !got[1].Skipped {
		t.Errorf("step-0002 should be Skipped; got %+v", got[1])
	}
	if got[1].Reason == "" {
		t.Errorf("step-0002 reason should explain unevaluable when; got %+v", got[1])
	}
}
