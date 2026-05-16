package pilot

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParseResponse(t *testing.T) {
	cases := []struct {
		in     string
		want   ResponseKind
		wantN  int
		reason string
	}{
		{"", RespReject, 0, "empty defaults to N (spec-67 §10)"},
		{"y", RespApply, 0, "y is apply"},
		{"Y", RespApply, 0, "case-insensitive"},
		{"yes", RespApply, 0, "long form"},
		{"apply", RespApply, 0, "verbose form"},
		{"n", RespReject, 0, "n is reject"},
		{"no", RespReject, 0, "long form reject"},
		{"reject", RespReject, 0, "verbose reject"},
		{"edit", RespEdit, 0, "edit triggers editor"},
		{"e", RespEdit, 0, "e shorthand"},
		{"abort", RespAbort, 0, "abort stops loop"},
		{"q", RespAbort, 0, "q quits"},
		{"quit", RespAbort, 0, "quit verb"},
		{"explain 3", RespExplain, 3, "explain takes a step index"},
		{"EXPLAIN 12", RespExplain, 12, "explain is case-insensitive"},
		{"  explain   2  ", RespExplain, 2, "whitespace tolerated"},
		{"explain", RespInvalid, 0, "bare explain without N is invalid"},
		{"explain abc", RespInvalid, 0, "non-numeric N is invalid"},
		{"explain 0", RespInvalid, 0, "step numbers are 1-based"},
		{"explain -1", RespInvalid, 0, "negative is invalid"},
		{"yes please", RespInvalid, 0, "no fuzzy yes/no — exact tokens only"},
		{"bogus", RespInvalid, 0, "unknown command"},
	}
	for _, tc := range cases {
		got := ParseResponse(tc.in)
		if got.Kind != tc.want || got.N != tc.wantN {
			t.Errorf("ParseResponse(%q) = (%v, %d); want (%v, %d) — %s",
				tc.in, got.Kind, got.N, tc.want, tc.wantN, tc.reason)
		}
	}
}

const samplePlan = `- name: write hello
  file.write:
    path: /tmp/hello.txt
    content: hi
- shell: echo done
`

func TestConfirmPlan_ApplyYes(t *testing.T) {
	in := strings.NewReader("y\n")
	out := &bytes.Buffer{}
	result, err := ConfirmPlan(in, out, []byte(samplePlan))
	if err != nil {
		t.Fatalf("ConfirmPlan: %v", err)
	}
	if result.Outcome != OutcomeApply {
		t.Errorf("Outcome = %v, want Apply", result.Outcome)
	}
	if !bytes.Equal(result.PlanBytes, []byte(samplePlan)) {
		t.Error("PlanBytes should be unchanged on plain apply")
	}
	if !strings.Contains(out.String(), "Plan: 2 steps") {
		t.Errorf("expected recap line, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Apply? [y/N/edit/explain N/abort]:") {
		t.Errorf("expected prompt, got: %s", out.String())
	}
}

func TestConfirmPlan_RejectDefault(t *testing.T) {
	// Empty line — should map to N (reject).
	in := strings.NewReader("\n")
	out := &bytes.Buffer{}
	result, err := ConfirmPlan(in, out, []byte(samplePlan))
	if err != nil {
		t.Fatalf("ConfirmPlan: %v", err)
	}
	if result.Outcome != OutcomeReject {
		t.Errorf("empty input should reject, got Outcome=%v", result.Outcome)
	}
}

func TestConfirmPlan_RejectExplicit(t *testing.T) {
	in := strings.NewReader("n\n")
	out := &bytes.Buffer{}
	result, _ := ConfirmPlan(in, out, []byte(samplePlan))
	if result.Outcome != OutcomeReject {
		t.Errorf("n should reject, got %v", result.Outcome)
	}
}

func TestConfirmPlan_Abort(t *testing.T) {
	in := strings.NewReader("abort\n")
	out := &bytes.Buffer{}
	result, _ := ConfirmPlan(in, out, []byte(samplePlan))
	if result.Outcome != OutcomeAbort {
		t.Errorf("abort should abort, got %v", result.Outcome)
	}
}

func TestConfirmPlan_ExplainThenApply(t *testing.T) {
	in := strings.NewReader("explain 1\ny\n")
	out := &bytes.Buffer{}
	result, _ := ConfirmPlan(in, out, []byte(samplePlan))
	if result.Outcome != OutcomeApply {
		t.Errorf("apply after explain should apply, got %v", result.Outcome)
	}
	if !strings.Contains(out.String(), "--- step 1 ---") {
		t.Errorf("explain should print step header, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "/tmp/hello.txt") {
		t.Errorf("explain should print step body, got: %s", out.String())
	}
}

func TestConfirmPlan_ExplainOutOfRange(t *testing.T) {
	in := strings.NewReader("explain 99\nn\n")
	out := &bytes.Buffer{}
	ConfirmPlan(in, out, []byte(samplePlan))
	if !strings.Contains(out.String(), "out of range") {
		t.Errorf("out-of-range explain should warn, got: %s", out.String())
	}
}

func TestConfirmPlan_InvalidThenAbort(t *testing.T) {
	in := strings.NewReader("bogus\nabort\n")
	out := &bytes.Buffer{}
	result, _ := ConfirmPlan(in, out, []byte(samplePlan))
	if result.Outcome != OutcomeAbort {
		t.Errorf("invalid should re-prompt, abort should win, got %v", result.Outcome)
	}
	if !strings.Contains(out.String(), "did not understand") {
		t.Errorf("invalid should print help, got: %s", out.String())
	}
}

func TestConfirmPlan_EditApply(t *testing.T) {
	const edited = `- name: edited step
  shell: echo edited
`
	// Stub the editor to rewrite the file with `edited` plan.
	orig := editorRunner
	editorRunner = func(_, path string) error {
		return os.WriteFile(path, []byte(edited), 0o600)
	}
	defer func() { editorRunner = orig }()

	in := strings.NewReader("edit\ny\n")
	out := &bytes.Buffer{}
	result, err := ConfirmPlan(in, out, []byte(samplePlan))
	if err != nil {
		t.Fatalf("ConfirmPlan: %v", err)
	}
	if result.Outcome != OutcomeApply {
		t.Fatalf("expected Apply after edit, got %v", result.Outcome)
	}
	if !bytes.Equal(result.PlanBytes, []byte(edited)) {
		t.Errorf("PlanBytes should be the edited contents.\nwant:\n%s\ngot:\n%s",
			edited, string(result.PlanBytes))
	}
	if !strings.Contains(out.String(), "edited step") {
		t.Error("edited plan should re-render after edit before re-prompting")
	}
}

func TestConfirmPlan_EditInvalidThenReject(t *testing.T) {
	const broken = `not a valid plan at all : :
nope
`
	orig := editorRunner
	editorRunner = func(_, path string) error {
		return os.WriteFile(path, []byte(broken), 0o600)
	}
	defer func() { editorRunner = orig }()

	// edit (writes broken), gate should print validation error and re-prompt;
	// operator rejects.
	in := strings.NewReader("edit\nn\n")
	out := &bytes.Buffer{}
	result, _ := ConfirmPlan(in, out, []byte(samplePlan))
	if result.Outcome != OutcomeReject {
		t.Errorf("reject after broken edit should reject, got %v", result.Outcome)
	}
	if !strings.Contains(out.String(), "failed validation") {
		t.Errorf("broken edit should surface validation error, got: %s", out.String())
	}
}

func TestRenderPlan_IrreversibleCount(t *testing.T) {
	// Note: actual irreversible count depends on which handlers implement
	// Reverser. We verify the recap line *shape* — that an irreversible
	// count is rendered when present — rather than pinning a specific N
	// (handler reversibility shifts as more handlers grow Reverser
	// methods).
	out := &bytes.Buffer{}
	if err := renderPlan(out, []byte(samplePlan)); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	if !strings.HasPrefix(strings.TrimLeft(out.String(), "\n"), "Plan: 2 step") {
		t.Errorf("recap should lead with step count, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "transaction") {
		t.Errorf("recap should mention transaction wrap, got: %q", out.String())
	}
}

func TestRenderPlan_HandlesMalformed(t *testing.T) {
	out := &bytes.Buffer{}
	// renderPlan should not error on garbage; the gate still needs to
	// run so the operator can edit/abort.
	err := renderPlan(out, []byte("this is { not yaml ["))
	if err != nil {
		t.Errorf("renderPlan on malformed input returned error: %v (should fall through)", err)
	}
}

func TestEnsureInteractive_NonFile(t *testing.T) {
	// Strings.Reader is not *os.File, so this must refuse.
	err := EnsureInteractive(strings.NewReader("anything"))
	if err == nil {
		t.Error("EnsureInteractive should refuse non-*os.File readers")
	}
}

func TestEnsureInteractive_NonTTYFile(t *testing.T) {
	// A regular file is *os.File but not a TTY — must refuse.
	f, err := os.CreateTemp("", "pilot-tty-test-*")
	if err != nil {
		t.Fatalf("tempfile: %v", err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	if err := EnsureInteractive(f); err == nil {
		t.Error("regular file is not a TTY; EnsureInteractive should refuse")
	}
}
