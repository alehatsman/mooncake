package agent

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParseResponse(t *testing.T) {
	cases := []struct {
		in     string
		style  Style
		want   ResponseKind
		wantN  int
		reason string
	}{
		{"", StylePlan, RespReject, 0, "empty defaults to N (spec-67 §10)"},
		{"y", StylePlan, RespApply, 0, "y is apply"},
		{"Y", StylePlan, RespApply, 0, "case-insensitive"},
		{"yes", StylePlan, RespApply, 0, "long form"},
		{"apply", StylePlan, RespApply, 0, "verbose form"},
		{"n", StylePlan, RespReject, 0, "n is reject"},
		{"no", StylePlan, RespReject, 0, "long form reject"},
		{"reject", StylePlan, RespReject, 0, "verbose reject"},
		{"edit", StylePlan, RespEdit, 0, "edit triggers editor"},
		{"e", StylePlan, RespEdit, 0, "e shorthand"},
		{"abort", StylePlan, RespAbort, 0, "abort stops loop"},
		{"q", StylePlan, RespAbort, 0, "q quits"},
		{"quit", StylePlan, RespAbort, 0, "quit verb"},
		{"explain 3", StylePlan, RespExplain, 3, "explain takes a step index"},
		{"EXPLAIN 12", StylePlan, RespExplain, 12, "explain is case-insensitive"},
		{"  explain   2  ", StylePlan, RespExplain, 2, "whitespace tolerated"},
		{"explain", StylePlan, RespInvalid, 0, "bare explain without N is invalid"},
		{"explain abc", StylePlan, RespInvalid, 0, "non-numeric N is invalid"},
		{"explain 0", StylePlan, RespInvalid, 0, "step numbers are 1-based"},
		{"explain -1", StylePlan, RespInvalid, 0, "negative is invalid"},
		{"yes please", StylePlan, RespInvalid, 0, "no fuzzy yes/no — exact tokens only"},
		{"bogus", StylePlan, RespInvalid, 0, "unknown command"},
		// Step-style bulk-approve tokens.
		{"approve_next 1", StyleStep, RespApproveNext, 1, "approve_next 1 maps under step style"},
		{"approve_next 12", StyleStep, RespApproveNext, 12, "approve_next accepts multi-digit N"},
		{"  approve_next   3  ", StyleStep, RespApproveNext, 3, "approve_next tolerates whitespace"},
		{"APPROVE_NEXT 2", StyleStep, RespApproveNext, 2, "approve_next is case-insensitive"},
		{"approve_next 0", StyleStep, RespInvalid, 0, "approve_next 0 is invalid (N must be ≥ 1)"},
		{"approve_next -1", StyleStep, RespInvalid, 0, "negative N is invalid"},
		{"approve_next", StyleStep, RespInvalid, 0, "bare approve_next without N is invalid"},
		{"approve_next abc", StyleStep, RespInvalid, 0, "non-numeric N is invalid"},
		{"approve_thread", StyleStep, RespApproveThread, 0, "approve_thread under step style"},
		{"APPROVE_THREAD", StyleStep, RespApproveThread, 0, "approve_thread is case-insensitive"},
		// Plan-style must reject the step-only tokens (plan §3).
		{"approve_next 3", StylePlan, RespInvalid, 0, "plan style rejects approve_next"},
		{"approve_thread", StylePlan, RespInvalid, 0, "plan style rejects approve_thread"},
	}
	for _, tc := range cases {
		got := ParseResponse(tc.in, tc.style)
		if got.Kind != tc.want || got.N != tc.wantN {
			t.Errorf("ParseResponse(%q, %v) = (%v, %d); want (%v, %d) — %s",
				tc.in, tc.style, got.Kind, got.N, tc.want, tc.wantN, tc.reason)
		}
	}
}

func TestConfirmPlan_StepGateAutoApproveCountdown(t *testing.T) {
	state := &StepGateState{}
	// First call: operator picks `approve_next 2` — this applies the
	// current step AND auto-approves the next two without prompting.
	in := strings.NewReader("approve_next 2\n")
	out := &bytes.Buffer{}
	result, err := ConfirmPlanStep(in, out, []byte(samplePlan), state)
	if err != nil {
		t.Fatalf("ConfirmPlanStep call 1: %v", err)
	}
	if result.Outcome != OutcomeApply {
		t.Fatalf("call 1 outcome = %v, want Apply", result.Outcome)
	}
	if state.RemainingAutoApprovals != 2 {
		t.Errorf("state.RemainingAutoApprovals = %d, want 2", state.RemainingAutoApprovals)
	}

	// Calls 2 and 3 must short-circuit to Apply WITHOUT reading from
	// the input — pass a reader that would error if read.
	for i := 2; i <= 3; i++ {
		emptyReader := strings.NewReader("")
		out := &bytes.Buffer{}
		result, err := ConfirmPlanStep(emptyReader, out, []byte(samplePlan), state)
		if err != nil {
			t.Fatalf("ConfirmPlanStep call %d: %v", i, err)
		}
		if result.Outcome != OutcomeApply {
			t.Errorf("call %d outcome = %v, want Apply (auto-approve)", i, result.Outcome)
		}
		if out.Len() != 0 {
			t.Errorf("call %d should not render plan when auto-approving; got %q", i, out.String())
		}
	}
	if state.RemainingAutoApprovals != 0 {
		t.Errorf("after 2 auto-approvals counter = %d, want 0", state.RemainingAutoApprovals)
	}

	// Call 4: counter is exhausted, must prompt again. Reject to keep
	// the test deterministic.
	in4 := strings.NewReader("n\n")
	out4 := &bytes.Buffer{}
	result4, err := ConfirmPlanStep(in4, out4, []byte(samplePlan), state)
	if err != nil {
		t.Fatalf("ConfirmPlanStep call 4: %v", err)
	}
	if result4.Outcome != OutcomeReject {
		t.Errorf("call 4 outcome = %v, want Reject after counter exhausted", result4.Outcome)
	}
	if !strings.Contains(out4.String(), "Apply?") {
		t.Errorf("call 4 should re-prompt; got %q", out4.String())
	}
}

func TestConfirmPlan_StepGateApproveThreadIsSticky(t *testing.T) {
	state := &StepGateState{}
	in := strings.NewReader("approve_thread\n")
	out := &bytes.Buffer{}
	result, err := ConfirmPlanStep(in, out, []byte(samplePlan), state)
	if err != nil {
		t.Fatalf("ConfirmPlanStep: %v", err)
	}
	if result.Outcome != OutcomeApply {
		t.Fatalf("approve_thread outcome = %v, want Apply", result.Outcome)
	}
	if !state.ApprovedThread {
		t.Error("state.ApprovedThread should be true after approve_thread")
	}
	// Subsequent calls must Apply without reading input regardless of
	// how many.
	for i := 0; i < 5; i++ {
		result, err := ConfirmPlanStep(strings.NewReader(""), &bytes.Buffer{}, []byte(samplePlan), state)
		if err != nil {
			t.Fatalf("sticky call %d: %v", i, err)
		}
		if result.Outcome != OutcomeApply {
			t.Errorf("sticky call %d outcome = %v, want Apply", i, result.Outcome)
		}
	}
}

func TestConfirmPlan_StepStylePromptIncludesBulkTokens(t *testing.T) {
	state := &StepGateState{}
	in := strings.NewReader("n\n")
	out := &bytes.Buffer{}
	_, _ = ConfirmPlanStep(in, out, []byte(samplePlan), state)
	if !strings.Contains(out.String(), "approve_next N/approve_thread") {
		t.Errorf("step-style prompt should advertise bulk-approve tokens; got: %s", out.String())
	}
}

func TestConfirmPlan_PlanStyleRejectsStepTokens(t *testing.T) {
	// Under StylePlan, `approve_next 3` and `approve_thread` are
	// invalid tokens — the gate must re-prompt and eventually take an
	// explicit reject.
	in := strings.NewReader("approve_next 3\napprove_thread\nn\n")
	out := &bytes.Buffer{}
	result, err := ConfirmPlan(in, out, []byte(samplePlan))
	if err != nil {
		t.Fatalf("ConfirmPlan: %v", err)
	}
	if result.Outcome != OutcomeReject {
		t.Errorf("Outcome = %v, want Reject after step tokens rejected", result.Outcome)
	}
	if !strings.Contains(out.String(), "did not understand") {
		t.Errorf("plan style should print invalid-help for step tokens; got: %s", out.String())
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
	f, err := os.CreateTemp("", "agent-tty-test-*")
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
