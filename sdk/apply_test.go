package mooncake_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// writeFile drops content at <dir>/<name> and returns the path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// recordingSubscriber (events_test.go) is the in-process tap reused here to
// assert the kernel lifecycle reached a caller-supplied Subscriber.

// markerHandler is a consumer-defined typed action with a real side effect:
// running it writes a marker file. It carries no config-struct binding, so it
// exercises the #111 generic carrier — the path that lets a registered custom
// action dispatch from a real plan exactly like a built-in.
type markerHandler struct{ path string }

func (markerHandler) Metadata() mooncake.ActionMetadata {
	return mooncake.ActionMetadata{
		Name:        "demo.marker",
		Description: "write a marker file (custom framework action)",
	}
}

func (markerHandler) Validate(*mooncake.Step) error { return nil }

func (h markerHandler) Run(_ mooncake.Context, _ *mooncake.Step) (mooncake.Result, error) {
	if err := os.WriteFile(h.path, []byte("ran\n"), 0o644); err != nil {
		return nil, err
	}
	return mooncake.NewResult(), nil
}

// TestApply_CustomActionOffline is the #122 acceptance: run a config built on
// a consumer's own typed action, with no LLM, and observe events through a
// caller-supplied Subscriber. The custom handler lives only in the passed
// Registry — never the global — and dispatches via the #111 carrier because
// Apply threads that Registry into the planner and executor.
func TestApply_CustomActionOffline(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")

	reg := mooncake.DefaultRegistry()
	if err := reg.Register(markerHandler{path: marker}); err != nil {
		t.Fatalf("register custom handler: %v", err)
	}

	cfg := writeFile(t, dir, "play.yml", `
- name: run custom marker action
  demo.marker:
    note: hi
`)

	rec := &recordingSubscriber{}
	res, err := mooncake.Apply(context.Background(), mooncake.ApplyOptions{
		ConfigPath:  cfg,
		Registry:    reg,
		Subscribers: []mooncake.Subscriber{rec},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false; ErrorMessage=%q", res.Summary.ErrorMessage)
	}

	// The custom action actually ran (real side effect, no LLM).
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("custom action did not run: marker %s missing: %v", marker, statErr)
	}

	// The caller's Subscriber saw the run lifecycle.
	if !rec.saw(mooncake.EventRunStarted) || !rec.saw(mooncake.EventRunCompleted) {
		t.Errorf("subscriber missed run lifecycle; saw %v", rec.types)
	}

	// The custom action is NOT in the global registry — isolation holds.
	if mooncake.GlobalRegistry().Has("demo.marker") {
		t.Error("custom action leaked into the global registry")
	}
}

// TestApply_BuiltinFileWrite proves the no-LLM apply path against a built-in
// action and checks the typed result counters.
func TestApply_BuiltinFileWrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hello.txt")
	cfg := writeFile(t, dir, "play.yml", `
- name: write hello
  file.write:
    path: `+target+`
    state: file
    content: "hello\n"
    mode: "0644"
`)

	res, err := mooncake.Apply(context.Background(), mooncake.ApplyOptions{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false; ErrorMessage=%q", res.Summary.ErrorMessage)
	}
	if res.Summary.Changed < 1 {
		t.Errorf("Summary.Changed=%d; want >=1 (file created)", res.Summary.Changed)
	}
	if got, _ := os.ReadFile(target); string(got) != "hello\n" {
		t.Errorf("file content = %q; want %q", got, "hello\n")
	}
	if len(res.Steps) == 0 {
		t.Error("KernelResult.Steps empty; want the file.write record")
	}
}

// TestPlan_DryRunNoSideEffects is the #122 dry-run acceptance: compile + inspect
// a config in plan mode and get per-step predictions (would-change + a
// structural diff for a Differ handler) with NO side effects on disk.
func TestPlan_DryRunNoSideEffects(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hello.txt")
	cfg := writeFile(t, dir, "play.yml", `
- name: write hello
  file.write:
    path: `+target+`
    state: file
    content: "hello\n"
    mode: "0644"
`)

	preview, err := mooncake.Plan(context.Background(), mooncake.PlanOptions{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// No side effects — the preview must not touch the filesystem.
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatalf("Plan created %s; dry-run must not mutate state", target)
	}

	if len(preview.Inspections) == 0 {
		t.Fatalf("preview has no inspections")
	}

	var sawWriteWithDiff bool
	for _, in := range preview.Inspections {
		if in.ActionType == "file.write" {
			if !in.WouldChange {
				t.Errorf("file.write WouldChange=false; want true (target absent)")
			}
			if in.Diff != nil {
				sawWriteWithDiff = true
			}
		}
	}
	if !sawWriteWithDiff {
		t.Errorf("file.write inspection carried no Diff; inspections=%+v", preview.Inspections)
	}
}

// TestPlan_InlineVars exercises inline Vars overlay: a templated path resolves
// from the caller-supplied map with no vars file on disk.
func TestPlan_InlineVars(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "play.yml", `
- name: write via var
  file.write:
    path: "{{ out_dir }}/v.txt"
    state: file
    content: "x\n"
    mode: "0644"
`)

	preview, err := mooncake.Plan(context.Background(), mooncake.PlanOptions{
		ConfigPath: cfg,
		Vars:       map[string]interface{}{"out_dir": dir},
	})
	if err != nil {
		t.Fatalf("Plan with inline vars: %v", err)
	}
	if len(preview.Inspections) == 0 {
		t.Fatalf("preview has no inspections")
	}
}
