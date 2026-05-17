package explain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/ops"
	"github.com/alehatsman/mooncake/internal/runlog"
)

// fixedRuns / fixedOps return fixture data without touching the
// filesystem. The resolver's Options.RunsReader / OpsReader hooks
// take these directly.
func fixedRuns(entries []runlog.Entry) func() ([]runlog.Entry, error) {
	return func() ([]runlog.Entry, error) { return entries, nil }
}

func fixedOps(entries []ops.Entry) func() ([]ops.Entry, error) {
	return func() ([]ops.Entry, error) { return entries, nil }
}

func TestResolveRun_Found(t *testing.T) {
	ts := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	entries := []runlog.Entry{{
		TS:         ts,
		Config:     "main.yml",
		Changed:    1,
		Ok:         2,
		DurationMs: 5000,
		RunID:      "r/01HVTEST",
		OpID:       "op/01HVTEST",
		Steps: []runlog.StepEntry{
			{Index: 1, Action: "file.write", Resource: "file:/etc/x", Result: "changed", Reversible: true},
			{Index: 2, Action: "shell", Result: "ok"},
		},
	}}
	opts := Options{RunsReader: fixedRuns(entries)}

	r := Resolve("r/01HVTEST", opts)
	if r.Kind != KindRun {
		t.Fatalf("kind = %q, want %q (notfound=%+v)", r.Kind, KindRun, r.NotFound)
	}
	if r.Run.RunID != "r/01HVTEST" {
		t.Errorf("RunID = %q", r.Run.RunID)
	}
	if r.Run.Totals.Changed != 1 || r.Run.Totals.Ok != 2 {
		t.Errorf("totals = %+v", r.Run.Totals)
	}
	if len(r.Run.Steps) != 2 {
		t.Fatalf("steps len = %d, want 2", len(r.Run.Steps))
	}
	// shell has no Reverser → irreversible count is 1.
	if r.Run.Caveats.IrreversibleStepCount != 1 {
		t.Errorf("irreversible step count = %d, want 1", r.Run.Caveats.IrreversibleStepCount)
	}
}

func TestResolveRun_NotFound(t *testing.T) {
	opts := Options{RunsReader: fixedRuns(nil)}
	r := Resolve("r/01HMISSING", opts)
	if r.Kind != KindNotFound {
		t.Fatalf("kind = %q, want not_found", r.Kind)
	}
}

func TestResolveOp_FoundWithDerivedRuns(t *testing.T) {
	ts := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	opsEntries := []ops.Entry{{
		TS:      ts,
		OpID:    "op/01HVTEST",
		Command: "apply",
		Args:    []string{"--config", "main.yml"},
		Actor:   "user:test",
		Config:  "main.yml",
	}}
	runEntries := []runlog.Entry{
		{TS: ts, RunID: "r/01HV1", OpID: "op/01HVTEST"},
		{TS: ts, RunID: "r/01HV2", OpID: "op/OTHER"}, // not for this op
	}
	opts := Options{
		RunsReader: fixedRuns(runEntries),
		OpsReader:  fixedOps(opsEntries),
	}

	r := Resolve("op/01HVTEST", opts)
	if r.Kind != KindOp {
		t.Fatalf("kind = %q, want %q (notfound=%+v)", r.Kind, KindOp, r.NotFound)
	}
	if r.Op.OpID != "op/01HVTEST" {
		t.Errorf("OpID = %q", r.Op.OpID)
	}
	if r.Op.Command != "apply" {
		t.Errorf("Command = %q", r.Op.Command)
	}
	if len(r.Op.Runs) != 1 || r.Op.Runs[0] != "r/01HV1" {
		t.Errorf("derived runs = %v, want [r/01HV1]", r.Op.Runs)
	}
}

func TestResolveOp_PlanOnly(t *testing.T) {
	opsEntries := []ops.Entry{{
		OpID:     "op/01HVPLAN",
		Command:  "plan",
		PlanOnly: true,
	}}
	opts := Options{
		OpsReader:  fixedOps(opsEntries),
		RunsReader: fixedRuns(nil),
	}
	r := Resolve("op/01HVPLAN", opts)
	if r.Kind != KindOp {
		t.Fatalf("kind = %q", r.Kind)
	}
	if !r.Op.PlanOnly {
		t.Errorf("PlanOnly = false, want true")
	}
	if len(r.Op.Runs) != 0 {
		t.Errorf("PlanOnly op has runs: %v", r.Op.Runs)
	}
}

func TestResolveOp_NotFound(t *testing.T) {
	opts := Options{OpsReader: fixedOps(nil)}
	r := Resolve("op/01HMISSING", opts)
	if r.Kind != KindNotFound {
		t.Fatalf("kind = %q", r.Kind)
	}
}

func TestResolveResource_HistoryNewestFirst(t *testing.T) {
	older := time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	entries := []runlog.Entry{
		// Oldest entry first (file order)
		{TS: older, RunID: "r/A", OpID: "op/A", Steps: []runlog.StepEntry{
			{Index: 1, Action: "file.write", Resource: "file:/etc/x", Result: "changed", Reversible: true},
		}},
		{TS: newer, RunID: "r/B", OpID: "op/B", Steps: []runlog.StepEntry{
			{Index: 1, Action: "file.write", Resource: "file:/etc/x", Result: "changed", Reversible: true},
			{Index: 2, Action: "shell", Resource: "", Result: "ok"},
		}},
	}
	opts := Options{RunsReader: fixedRuns(entries)}
	r := Resolve("file:/etc/x", opts)
	if r.Kind != KindResource {
		t.Fatalf("kind = %q, want resource (notfound=%+v)", r.Kind, r.NotFound)
	}
	if len(r.Resource.History) != 2 {
		t.Fatalf("history len = %d, want 2", len(r.Resource.History))
	}
	// Newest-first: r/B should come before r/A.
	if r.Resource.History[0].RunID != "r/B" || r.Resource.History[1].RunID != "r/A" {
		t.Errorf("history order = %v, want newest-first [r/B, r/A]",
			[]string{r.Resource.History[0].RunID, r.Resource.History[1].RunID})
	}
}

// F045: when one run touches the same resource multiple times, the
// emitted ResourceEvents must carry the step index so readers can
// tell them apart (same TS / RunID / Action / Result otherwise).
func TestResolveResource_PreservesStepIndex(t *testing.T) {
	ts := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	entries := []runlog.Entry{
		{TS: ts, RunID: "r/A", OpID: "op/A", Steps: []runlog.StepEntry{
			{Index: 1, Action: "file.write", Resource: "file:/tmp/multi", Result: "changed", Reversible: true},
			{Index: 2, Action: "file.write", Resource: "file:/tmp/multi", Result: "changed", Reversible: true},
			{Index: 3, Action: "file.write", Resource: "file:/tmp/multi", Result: "changed", Reversible: true},
		}},
	}
	opts := Options{RunsReader: fixedRuns(entries)}
	r := Resolve("file:/tmp/multi", opts)
	if r.Kind != KindResource {
		t.Fatalf("kind = %q, want resource (notfound=%+v)", r.Kind, r.NotFound)
	}
	if len(r.Resource.History) != 3 {
		t.Fatalf("history len = %d, want 3", len(r.Resource.History))
	}
	// History is newest-first; within a single run the file-order is
	// preserved through the reverse, so the *last* step in the run
	// ends up at history[0].
	wantIndices := []int{3, 2, 1}
	for i, want := range wantIndices {
		if got := r.Resource.History[i].StepIndex; got != want {
			t.Errorf("history[%d].StepIndex = %d, want %d", i, got, want)
		}
	}
}

// Spec-68 wave 2.5: per-step Diff + StartTS round-trip through the
// resolver into RunStep + ResourceEvent. Diff stays as opaque
// json.RawMessage; the resolver doesn't need to know the shape.
func TestResolveRun_CarriesDiffAndStartTS(t *testing.T) {
	runTS := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	stepTS := time.Date(2026, 5, 17, 12, 0, 2, 0, time.UTC)
	diffJSON := []byte(`{"resource":{"kind":"file","identifier":"/etc/x"},"operation":"update","before":{"size":100},"after":{"size":110}}`)
	entries := []runlog.Entry{{
		TS:     runTS,
		RunID:  "r/01HW25",
		OpID:   "op/01HW25",
		Config: "main.yml",
		Steps: []runlog.StepEntry{{
			Index:   1,
			Action:  "file.write",
			Result:  "changed",
			StartTS: stepTS,
			Diff:    json.RawMessage(diffJSON),
		}},
	}}
	opts := Options{RunsReader: fixedRuns(entries)}

	r := Resolve("r/01HW25", opts)
	if r.Kind != KindRun {
		t.Fatalf("kind = %q, want run (notfound=%+v)", r.Kind, r.NotFound)
	}
	if len(r.Run.Steps) != 1 {
		t.Fatalf("steps len = %d, want 1", len(r.Run.Steps))
	}
	got := r.Run.Steps[0]
	if !got.StartTS.Equal(stepTS) {
		t.Errorf("StartTS = %v, want %v", got.StartTS, stepTS)
	}
	if string(got.Diff) != string(diffJSON) {
		t.Errorf("Diff payload mismatch:\n got:  %s\n want: %s", got.Diff, diffJSON)
	}
}

func TestResolveResource_CarriesDiffAndStartTS(t *testing.T) {
	runTS := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	stepTS := time.Date(2026, 5, 17, 12, 0, 5, 0, time.UTC)
	diffJSON := []byte(`{"resource":{"kind":"pkg","identifier":"apt/curl"},"operation":"create"}`)
	entries := []runlog.Entry{{
		TS:    runTS,
		RunID: "r/A",
		OpID:  "op/A",
		Steps: []runlog.StepEntry{{
			Index:    1,
			Action:   "pkg",
			Resource: "pkg:apt/curl",
			Result:   "changed",
			StartTS:  stepTS,
			Diff:     json.RawMessage(diffJSON),
		}},
	}}
	opts := Options{RunsReader: fixedRuns(entries)}

	r := Resolve("pkg:apt/curl", opts)
	if r.Kind != KindResource {
		t.Fatalf("kind = %q, want resource (notfound=%+v)", r.Kind, r.NotFound)
	}
	if len(r.Resource.History) != 1 {
		t.Fatalf("history len = %d, want 1", len(r.Resource.History))
	}
	got := r.Resource.History[0]
	if !got.StartTS.Equal(stepTS) {
		t.Errorf("StartTS = %v, want %v", got.StartTS, stepTS)
	}
	if string(got.Diff) != string(diffJSON) {
		t.Errorf("Diff payload mismatch:\n got:  %s\n want: %s", got.Diff, diffJSON)
	}
}

func TestResolveResource_EmptyHistory(t *testing.T) {
	opts := Options{RunsReader: fixedRuns(nil)}
	r := Resolve("file:/etc/never-touched", opts)
	if r.Kind != KindResource {
		t.Fatalf("kind = %q, want resource (empty history is still a resource result)", r.Kind)
	}
	if len(r.Resource.History) != 0 {
		t.Errorf("history len = %d, want 0", len(r.Resource.History))
	}
}
