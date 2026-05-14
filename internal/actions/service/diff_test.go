package service

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestDiff_Service_AlwaysUpdate locks in the conservative shape:
// every os.service intent is a mutation, so Diff.Operation is always
// OpUpdate. A future enhancement may upgrade specific states to OpNoop
// by querying the service manager, but the contract floor is
// "OpUpdate or error".
func TestDiff_Service_AlwaysUpdate(t *testing.T) {
	h := &Handler{}
	cases := []*config.ServiceAction{
		{Name: "ssh", State: "started"},
		{Name: "ssh", State: "stopped"},
		{Name: "ssh", State: "restarted"},
		{Name: "ssh", State: "reloaded"},
		{Name: "ssh"}, // only Enabled/unit changes
		{Name: "ssh", DaemonReload: true},
	}
	for _, svc := range cases {
		step := &config.Step{OsService: svc}
		d, err := h.Diff(nil, step)
		if err != nil {
			t.Fatalf("Diff(%+v): %v", svc, err)
		}
		if d.Operation != actions.OpUpdate {
			t.Errorf("Diff(state=%q).Operation = %q, want update", svc.State, d.Operation)
		}
	}
}

// TestDiff_Service_ResourceShape — Resource.Kind must be
// ResourceService and Identifier the service Name. Without this,
// consumers that group by Kind would miss os.service entirely.
func TestDiff_Service_ResourceShape(t *testing.T) {
	h := &Handler{}
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx", State: "restarted"}}
	d, err := h.Diff(nil, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Resource.Kind != actions.ResourceService {
		t.Errorf("Resource.Kind = %q, want %q", d.Resource.Kind, actions.ResourceService)
	}
	if d.Resource.Identifier != "nginx" {
		t.Errorf("Resource.Identifier = %q, want nginx", d.Resource.Identifier)
	}
}

// TestDiff_Service_AfterCarriesIntent — After.ServiceSnapshot must
// reflect every intent-carrying field so consumers (UIs, the agent
// SDK, policy layers) can show what the step would do without
// re-parsing the step shape.
func TestDiff_Service_AfterCarriesIntent(t *testing.T) {
	h := &Handler{}
	enabled := true
	step := &config.Step{OsService: &config.ServiceAction{
		Name:         "myd",
		State:        "started",
		Enabled:      &enabled,
		DaemonReload: true,
	}}
	d, err := h.Diff(nil, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	after := d.After.(*ServiceSnapshot)
	if after.Name != "myd" {
		t.Errorf("After.Name = %q, want myd", after.Name)
	}
	if after.State != "started" {
		t.Errorf("After.State = %q, want started", after.State)
	}
	if after.Enabled == nil || !*after.Enabled {
		t.Errorf("After.Enabled = %v, want &true", after.Enabled)
	}
	if !after.DaemonReload {
		t.Error("After.DaemonReload = false, want true")
	}
}

// TestDiff_Service_BeforeAlwaysNil — pinning the "no measurement"
// contract. If a future change starts populating Before, this test
// is the early-warning system that the conservative semantics are
// changing.
func TestDiff_Service_BeforeAlwaysNil(t *testing.T) {
	h := &Handler{}
	step := &config.Step{OsService: &config.ServiceAction{Name: "ssh", State: "started"}}
	d, _ := h.Diff(nil, step)
	if d.Before != nil {
		t.Errorf("Before = %+v, want nil (no manager query in Diff)", d.Before)
	}
}

func TestDiff_Service_NilStep(t *testing.T) {
	h := &Handler{}
	if _, err := h.Diff(nil, nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := h.Diff(nil, &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

func TestDiff_Service_RegisteredAsDiffer(t *testing.T) {
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}
