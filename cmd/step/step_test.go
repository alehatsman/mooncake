package step

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// MT-22 regression tests: `mooncake step` must surface the full
// RegisteredResult.ToMap() payload, not a hardcoded subset. Typed
// actions (`repo.search`, `read.json`, `repo.tree`, …) populate
// `Result.Data` via `SetData`; the prior step JSON dropped Data
// entirely so agents got no way to consume what the action found.
//
// Proposal-01: Data lives nested under the top-level `data` key
// rather than being flattened into the envelope. The MT-22 contract
// (typed action payload round-trips) still holds — just one level
// deeper.

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
	// The headline assertion: action-specific Data must round-trip
	// through the nested `data` key (proposal-01 envelope).
	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data map missing or wrong type: %T %v", payload["data"], payload["data"])
	}
	if data["total_files"] != 1 {
		t.Errorf("data.total_files missing: got %v in %v", data["total_files"], data)
	}
	if data["total_matches"] != 1 {
		t.Errorf("data.total_matches missing: got %v", data["total_matches"])
	}
	results, ok := data["results"].([]map[string]any)
	if !ok || len(results) != 1 {
		t.Errorf("data.results[] missing or wrong type: %T %v", data["results"], data["results"])
	}
	// Envelope scalars stay at the top level.
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

// TestBuildStepJSON_FailedSetWhenExecErr is the MT-61 regression:
// a handler-returned error (the common shape: wait.* timeouts, any
// handler that errors without first calling SetFailed) used to come
// out as {"failed": false, "error": "..."} — internally inconsistent
// and easily misread by agents as success. Apply mode already marked
// the step failed; step mode now matches.
func TestBuildStepJSON_FailedSetWhenExecErr(t *testing.T) {
	r := executor.NewResult()
	payload := buildStepJSON("wait.http", r, errors.New("wait.http timeout after 1.001s"))

	if payload["failed"] != true {
		t.Errorf("failed = %v; want true when execErr is set", payload["failed"])
	}
	if payload["error"] != "wait.http timeout after 1.001s" {
		t.Errorf("error = %v", payload["error"])
	}
}

func TestBuildStepJSON_ErrorEmptyWhenNoExecErr(t *testing.T) {
	// Proposal-06 envelope: `error` is always present at the top level
	// and is the empty string when Failed=false. The pre-proposal
	// contract was "key absent unless populated"; the envelope makes
	// it always-present so consumers don't branch on key presence.
	r := executor.NewResult()
	r.Changed = true
	payload := buildStepJSON("shell", r, nil)
	if got, ok := payload["error"]; !ok {
		t.Errorf("error key should be present (empty) when execErr == nil: %v", payload)
	} else if got != "" {
		t.Errorf("error = %q; want empty string when execErr == nil", got)
	}
}

func TestBuildStepJSON_DataDoesNotShadowSharedScalars(t *testing.T) {
	// Proposal-01 envelope: action-specific Data is nested under the
	// `data` key, so it cannot collide with envelope scalars. Pre-
	// proposal, Data{changed: "yes"} would shadow the scalar changed:
	// because ToMap flattened. Now the two live in separate namespaces.
	r := executor.NewResult()
	r.Changed = false
	r.SetData(map[string]any{"changed": "stringified"})
	payload := buildStepJSON("custom", r, nil)
	if payload["changed"] != false {
		t.Errorf("envelope changed should be false; got %v", payload["changed"])
	}
	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data missing or wrong type: %T", payload["data"])
	}
	if data["changed"] != "stringified" {
		t.Errorf("data.changed = %v; want \"stringified\"", data["changed"])
	}
}

// strictDecodeStep mirrors the decode path inside Command's
// Action func: yaml.NewDecoder with KnownFields(true) into a
// config.Step. Exported here so the regression tests in this package
// can assert the strict shape without rebuilding the cli.Context.
func strictDecodeStep(raw string) error {
	var step config.Step
	return config.DecodeAutoStrict([]byte(raw), &step)
}

// MT-83: `mooncake step` must reject unknown fields the way `apply`
// does (via MT-44's strict reader pass). The headline repro used
// `expected_exit:` (the correct key is `expect_exit:`) — pre-fix
// it was silently dropped and the wait.command handler ran with
// default expectations, producing a confusing timeout.
func TestStepStrictDecode_RejectsUnknownNestedField(t *testing.T) {
	raw := `wait.command: { cmd: 'exit 42', expected_exit: 42 }`
	err := strictDecodeStep(raw)
	if err == nil {
		t.Fatal("expected error for unknown field expected_exit, got nil")
	}
	if !errContainsAll(err, "expected_exit", "not found") {
		t.Errorf("error should name the unknown field; got: %v", err)
	}
}

// MT-83: strict decode also rejects unknown step-level fields — the
// MT-15 / MT-77 aliases (creates:/unless:) and the documented
// universal fields stay accepted; everything else errors.
func TestStepStrictDecode_RejectsUnknownStepLevelField(t *testing.T) {
	raw := `shell: { cmd: 'true' }
not_a_real_field: true`
	err := strictDecodeStep(raw)
	if err == nil {
		t.Fatal("expected error for unknown step-level field, got nil")
	}
	if !errContainsAll(err, "not_a_real_field", "not found") {
		t.Errorf("error should name the unknown field; got: %v", err)
	}
}

// MT-83 regression guard: the correct field name parses cleanly.
// Without this, a typo in the "fixed" code (e.g. wrong yaml tag in
// the WaitCommand struct) would only surface as a runtime regression.
func TestStepStrictDecode_AcceptsKnownField(t *testing.T) {
	raw := `wait.command: { cmd: 'true', expect_exit: 0 }`
	if err := strictDecodeStep(raw); err != nil {
		t.Errorf("expected clean decode for canonical field expect_exit, got: %v", err)
	}
}

// errContainsAll reports whether err's message contains every needle.
// Avoids fmt.Sprintf("%v", err) → strings.Contains repetition.
func errContainsAll(err error, needles ...string) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for _, n := range needles {
		if !bytesContains(s, n) {
			return false
		}
	}
	return true
}

func bytesContains(s, n string) bool { return len(n) == 0 || bytes.Contains([]byte(s), []byte(n)) }

// config-json-input: `mooncake step` accepts JSON as well as YAML. Agents
// emit compact JSON to save tokens; strict-mode (MT-83) still applies.
func TestStepStrictDecode_AcceptsJSONInput(t *testing.T) {
	raw := `{"shell":{"cmd":"echo hi"}}`
	if err := strictDecodeStep(raw); err != nil {
		t.Errorf("expected clean decode for JSON step, got: %v", err)
	}
}

func TestStepStrictDecode_RejectsUnknownFieldFromJSON(t *testing.T) {
	raw := `{"wait.command":{"cmd":"exit 42","expected_exit":42}}`
	err := strictDecodeStep(raw)
	if err == nil {
		t.Fatal("expected error for unknown field expected_exit in JSON input, got nil")
	}
	if !errContainsAll(err, "expected_exit", "not found") {
		t.Errorf("error should name the unknown field; got: %v", err)
	}
}
