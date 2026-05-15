package main

import (
	"errors"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/executor"
)

// MT-22 regression tests: `mooncake step` must surface the full
// RegisteredResult.ToMap() payload, not a hardcoded subset. Typed
// actions (`repo.search`, `read.json`, `repo.tree`, …) populate
// `Result.Data` via `SetData`; the prior step JSON dropped Data
// entirely so agents got no way to consume what the action found.

// makeResultWithData returns a *executor.Result mimicking what a typed
// handler builds. Mirrors the repo.search shape from the manual-test
// repro: top-level result map carrying total_files / total_matches /
// results.
func makeResultWithData() *executor.Result {
	r := executor.NewResult()
	r.Changed = false
	r.Duration = 7 * time.Millisecond
	r.SetData(map[string]any{
		"total_files":   1,
		"total_matches": 1,
		"results": []map[string]any{
			{"file": "probe.txt", "line": 1, "column": 1, "match": "hello", "context": "hello"},
		},
	})
	return r
}

func TestBuildStepJSON_SurfacesActionDataMap(t *testing.T) {
	res := makeResultWithData()
	payload := buildStepJSON("repo.search", res, nil)

	if payload["action"] != "repo.search" {
		t.Errorf("action = %v", payload["action"])
	}
	// The headline assertion: action-specific Data must round-trip.
	if payload["total_files"] != 1 {
		t.Errorf("total_files missing: got %v in %v", payload["total_files"], payload)
	}
	if payload["total_matches"] != 1 {
		t.Errorf("total_matches missing: got %v", payload["total_matches"])
	}
	results, ok := payload["results"].([]map[string]any)
	if !ok || len(results) != 1 {
		t.Errorf("results[] missing or wrong type: %T %v", payload["results"], payload["results"])
	}
	// Scalars from the shared fields stay.
	if payload["changed"] != false {
		t.Errorf("changed = %v", payload["changed"])
	}
}

func TestBuildStepJSON_ShellShapePreserved(t *testing.T) {
	// Regression case from the repro: shell actions worked by accident
	// because stdout was in the hardcoded subset. They must still work
	// after the generalization — stdout/stderr come from
	// RegisteredResult.ToMap()'s top-level scalar fields.
	r := executor.NewResult()
	r.Stdout = "hi\n"
	r.Stderr = ""
	r.Rc = 0
	r.Changed = true

	payload := buildStepJSON("shell", r, nil)
	if payload["stdout"] != "hi\n" {
		t.Errorf("stdout = %q", payload["stdout"])
	}
	if payload["changed"] != true {
		t.Errorf("changed = %v", payload["changed"])
	}
	if payload["rc"] != 0 {
		t.Errorf("rc = %v", payload["rc"])
	}
}

func TestBuildStepJSON_PopulatesActionEvenOnNilResult(t *testing.T) {
	// Dispatch can fail before the handler ever populates ec.CurrentResult
	// (e.g. validation error). The JSON must still carry the action+error
	// so the agent can see what was attempted.
	payload := buildStepJSON("file.write", nil, errors.New("boom"))

	if payload["action"] != "file.write" {
		t.Errorf("action = %v", payload["action"])
	}
	if payload["error"] != "boom" {
		t.Errorf("error = %v", payload["error"])
	}
}

func TestBuildStepJSON_OmitsErrorWhenNil(t *testing.T) {
	r := executor.NewResult()
	r.Changed = true
	payload := buildStepJSON("shell", r, nil)
	if _, present := payload["error"]; present {
		t.Errorf("error key should be absent when execErr == nil: %v", payload)
	}
}

func TestBuildStepJSON_DataDoesNotShadowSharedScalars(t *testing.T) {
	// If a misbehaving handler set Data["changed"] = "yes", the shared
	// scalar from RegisteredResult.ToMap() is built BEFORE Data merges
	// in, so the Data entry wins. Documenting current behavior so we
	// notice if it ever flips silently — agents may rely on the schema
	// matching apply's, where Data also overrides via the same ToMap.
	r := executor.NewResult()
	r.Changed = false
	r.SetData(map[string]any{"changed": "stringified"})
	payload := buildStepJSON("custom", r, nil)
	if payload["changed"] != "stringified" {
		t.Errorf("expected Data override (mirroring apply's behavior), got %v", payload["changed"])
	}
}
