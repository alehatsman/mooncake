package fleet

// Unit tests for the pure helpers underpinning Apply's path-guarding and
// event-draining. The integration test in apply_test.go exercises the happy
// path through a live agentd; these pin the branch-level behaviour of the
// helpers in isolation (R2.1c-churned package, coverage catch-up — issue #33).

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

func TestIsUnderDir(t *testing.T) {
	dir := filepath.FromSlash("/srv/plan")
	tests := []struct {
		name string
		p    string
		want bool
	}{
		{"direct child file", filepath.FromSlash("/srv/plan/main.yml"), true},
		{"nested child", filepath.FromSlash("/srv/plan/vars/prod.yml"), true},
		{"equal to dir", filepath.FromSlash("/srv/plan"), false},
		{"parent dir", filepath.FromSlash("/srv"), false},
		{"sibling escaping via ..", filepath.FromSlash("/srv/other/x.yml"), false},
		{"explicit traversal out", filepath.FromSlash("/srv/plan/../secret.yml"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnderDir(dir, tt.p); got != tt.want {
				t.Errorf("isUnderDir(%q, %q) = %v, want %v", dir, tt.p, got, tt.want)
			}
		})
	}
}

func TestValidateApplyPaths(t *testing.T) {
	dir := filepath.FromSlash("/srv/plan")
	plan := filepath.FromSlash("/srv/plan/main.yml")
	vars := filepath.FromSlash("/srv/plan/vars.yml")

	tests := []struct {
		name    string
		opts    ApplyOptions
		wantErr bool
	}{
		{
			name: "valid plan + vars under dir",
			opts: ApplyOptions{PlanDir: dir, PlanPath: plan, VarsFiles: []string{vars}},
		},
		{
			name:    "empty PlanDir",
			opts:    ApplyOptions{PlanPath: plan},
			wantErr: true,
		},
		{
			name:    "empty PlanPath",
			opts:    ApplyOptions{PlanDir: dir},
			wantErr: true,
		},
		{
			name:    "relative PlanDir",
			opts:    ApplyOptions{PlanDir: filepath.FromSlash("plan"), PlanPath: plan},
			wantErr: true,
		},
		{
			name:    "relative PlanPath",
			opts:    ApplyOptions{PlanDir: dir, PlanPath: filepath.FromSlash("main.yml")},
			wantErr: true,
		},
		{
			name:    "PlanPath outside PlanDir",
			opts:    ApplyOptions{PlanDir: dir, PlanPath: filepath.FromSlash("/srv/other/main.yml")},
			wantErr: true,
		},
		{
			name:    "relative vars file",
			opts:    ApplyOptions{PlanDir: dir, PlanPath: plan, VarsFiles: []string{filepath.FromSlash("vars.yml")}},
			wantErr: true,
		},
		{
			name:    "vars file outside PlanDir",
			opts:    ApplyOptions{PlanDir: dir, PlanPath: plan, VarsFiles: []string{filepath.FromSlash("/etc/secret.yml")}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApplyPaths(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateApplyPaths() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTerminalStatus(t *testing.T) {
	tests := []struct {
		name       string
		ev         transport.Event
		wantStatus string
		wantOK     bool
	}{
		{
			name:   "non-terminal event",
			ev:     transport.Event{Type: "run.started"},
			wantOK: false,
		},
		{
			name:       "completed success=true",
			ev:         transport.Event{Type: "run.completed", Data: json.RawMessage(`{"success":true}`)},
			wantStatus: "success",
			wantOK:     true,
		},
		{
			name:       "completed success=false",
			ev:         transport.Event{Type: "run.completed", Data: json.RawMessage(`{"success":false}`)},
			wantStatus: "failed",
			wantOK:     true,
		},
		{
			name:       "completed with malformed data",
			ev:         transport.Event{Type: "run.completed", Data: json.RawMessage(`not-json`)},
			wantStatus: "completed",
			wantOK:     true,
		},
		{
			name:       "completed with no data is unparseable, falls back to completed",
			ev:         transport.Event{Type: "run.completed"},
			wantStatus: "completed",
			wantOK:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, ok := terminalStatus(tt.ev)
			if ok != tt.wantOK || status != tt.wantStatus {
				t.Errorf("terminalStatus() = (%q, %v), want (%q, %v)", status, ok, tt.wantStatus, tt.wantOK)
			}
		})
	}
}

func TestDrainEvents(t *testing.T) {
	t.Run("drains buffered events, counts, and applies terminal status", func(t *testing.T) {
		sink := make(chan transport.Event, 3)
		sink <- transport.Event{Type: "run.started"}
		sink <- transport.Event{Type: "step.completed"}
		sink <- transport.Event{Type: "run.completed", Data: json.RawMessage(`{"success":true}`)}

		var emitted []PeerEvent
		emit := func(pe PeerEvent) { emitted = append(emitted, pe) }
		result := &ApplyResult{}

		drainEvents(sink, emit, result)

		if result.Events != 3 {
			t.Errorf("result.Events = %d, want 3", result.Events)
		}
		if result.Status != "success" {
			t.Errorf("result.Status = %q, want %q", result.Status, "success")
		}
		if len(emitted) != 3 {
			t.Fatalf("emitted %d events, want 3", len(emitted))
		}
		for i, pe := range emitted {
			if pe.Kind != KindEvent {
				t.Errorf("emitted[%d].Kind = %v, want KindEvent", i, pe.Kind)
			}
		}
	})

	t.Run("empty channel returns immediately without side effects", func(t *testing.T) {
		sink := make(chan transport.Event)
		var emitted []PeerEvent
		emit := func(pe PeerEvent) { emitted = append(emitted, pe) }
		result := &ApplyResult{}

		drainEvents(sink, emit, result)

		if result.Events != 0 || result.Status != "" || len(emitted) != 0 {
			t.Errorf("drainEvents on empty channel mutated state: Events=%d Status=%q emitted=%d",
				result.Events, result.Status, len(emitted))
		}
	})
}
