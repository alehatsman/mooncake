package package_handler

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestDiff_Pkg_StateToOperation locks in the canonical mapping from
// step.Pkg.State → Diff.Operation. Pkg's Diff is conservative — Before
// is always nil because we don't query the manager from plan time — so
// the test focuses on the intent classification, not measurement.
func TestDiff_Pkg_StateToOperation(t *testing.T) {
	h := &Handler{}
	tests := []struct {
		name   string
		state  string
		wantOp actions.Operation
	}{
		{"empty defaults to present → create", "", actions.OpCreate},
		{"present → create", "present", actions.OpCreate},
		{"absent → delete", "absent", actions.OpDelete},
		{"latest → update", "latest", actions.OpUpdate},
		{"unknown state → conservative update", "weird", actions.OpUpdate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{Pkg: &config.Package{Name: "vim", State: tt.state}}
			d, err := h.Diff(nil, step)
			if err != nil {
				t.Fatalf("Diff: %v", err)
			}
			if d.Operation != tt.wantOp {
				t.Errorf("Operation = %q, want %q", d.Operation, tt.wantOp)
			}
			if d.Before != nil {
				t.Errorf("Before = %+v, want nil (pkg doesn't measure current state)", d.Before)
			}
		})
	}
}

// TestDiff_Pkg_AfterSnapshotPopulated — After must always carry the
// names + (normalised) state + manager. Without this the consumer
// can't tell what packages a pkg step targets.
func TestDiff_Pkg_AfterSnapshotPopulated(t *testing.T) {
	h := &Handler{}
	step := &config.Step{Pkg: &config.Package{
		Name:    "vim",
		State:   "present",
		Manager: "apt",
	}}
	d, err := h.Diff(nil, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	after := d.After.(*actions.PackageDiff)
	if len(after.Names) != 1 || after.Names[0] != "vim" {
		t.Errorf("After.Names = %v, want [vim]", after.Names)
	}
	if after.State != "present" {
		t.Errorf("After.State = %q, want present", after.State)
	}
	if after.Manager != "apt" {
		t.Errorf("After.Manager = %q, want apt", after.Manager)
	}
}

// TestDiff_Pkg_MultipleNames — both `name:` and `names:` should
// surface as the After.Names slice. `names:` wins when both are set
// (mirrors the validator's preference). Resource.Identifier shows
// the first name with "(+N more)" suffix so the consumer doesn't
// need to drill into After for a one-line label.
func TestDiff_Pkg_MultipleNames(t *testing.T) {
	h := &Handler{}
	step := &config.Step{Pkg: &config.Package{
		Names: []string{"vim", "tmux", "git"},
		State: "present",
	}}
	d, err := h.Diff(nil, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	after := d.After.(*actions.PackageDiff)
	if len(after.Names) != 3 {
		t.Errorf("After.Names len = %d, want 3", len(after.Names))
	}
	// The identifier should show first name and indicate the
	// remainder.
	if !strings.Contains(d.Resource.Identifier, "vim") {
		t.Errorf("Resource.Identifier = %q, want it to contain 'vim'", d.Resource.Identifier)
	}
	if !strings.Contains(d.Resource.Identifier, "2 more") {
		t.Errorf("Resource.Identifier = %q, want '(+2 more)' suffix", d.Resource.Identifier)
	}
}

// TestDiff_Pkg_EmptyNamesProducesUnnamed — if neither Name nor Names
// is set, identifier falls back to "<unnamed>" rather than panicking.
// Validate would catch this at runtime; Diff still produces a
// well-formed response.
func TestDiff_Pkg_EmptyNamesProducesUnnamed(t *testing.T) {
	h := &Handler{}
	step := &config.Step{Pkg: &config.Package{State: "present"}}
	d, err := h.Diff(nil, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Resource.Identifier != "<unnamed>" {
		t.Errorf("Resource.Identifier = %q, want '<unnamed>'", d.Resource.Identifier)
	}
}

// TestDiff_Pkg_NilStep + RegisteredAsDiffer — defensive + capability.
func TestDiff_Pkg_NilStep(t *testing.T) {
	h := &Handler{}
	if _, err := h.Diff(nil, nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := h.Diff(nil, &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

func TestDiff_Pkg_RegisteredAsDiffer(t *testing.T) {
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}
