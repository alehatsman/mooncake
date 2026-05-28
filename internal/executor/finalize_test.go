package executor

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
)

// finalizeTestContext constructs an ExecutionContext wired with the
// template renderer + expression evaluator that applyResultOverrides
// needs. Tests share this helper because the override path's only
// real dependencies on ctx are Template() / Evaluator() /
// Variables().
func finalizeTestContext(vars map[string]interface{}) *ExecutionContext {
	scope := NewVariableScope()
	if vars != nil {
		scope.User = vars
	}
	return &ExecutionContext{
		Svc: &RunServices{
			Evaluator: expression.NewGovaluateEvaluator(),
			Template:  mustNewRenderer(),
			Logger:    logger.NewTestLogger(),
		},
		Scope: scope,
	}
}

// TestApplyResultOverrides_NoOverrides — empty changed_when /
// failed_when leaves the result untouched. The check has to be
// short-circuiting so handlers that never declare overrides don't
// pay the template-render cost.
func TestApplyResultOverrides_NoOverrides(t *testing.T) {
	ctx := finalizeTestContext(nil)
	step := &config.Step{}
	r := NewResult()
	r.Changed = true
	r.Failed = true
	r.Rc = 42

	if err := applyResultOverrides(ctx, step, r); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !r.Changed || !r.Failed || r.Rc != 42 {
		t.Errorf("result mutated despite no overrides: %+v", r)
	}
}

// TestApplyResultOverrides_FailedWhenFalseMasks — the documented
// "fail-the-step-no-matter-what is the operator's choice" pattern.
// Underlying result.Failed = true, but failed_when:false makes
// applyResultOverrides report success.
func TestApplyResultOverrides_FailedWhenFalseMasks(t *testing.T) {
	ctx := finalizeTestContext(nil)
	step := &config.Step{FailedWhen: "false"}
	r := NewResult()
	r.Failed = true
	r.Rc = 1

	if err := applyResultOverrides(ctx, step, r); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r.Failed {
		t.Errorf("failed_when:false should clear Failed; got %+v", r)
	}
	// Rc stays at 1 — Issue #21 explicitly forbids fabricating Rc to
	// signal failure. The override flips the verdict bit, nothing else.
	if r.Rc != 1 {
		t.Errorf("Rc must not change in override path; got %d", r.Rc)
	}
}

// TestApplyResultOverrides_FailedWhenTruePromotes — a clean
// exit-zero command can still be marked failed when failed_when
// evaluates true (e.g. the operator pattern-matches stdout for an
// error string the binary doesn't return non-zero on).
func TestApplyResultOverrides_FailedWhenTruePromotes(t *testing.T) {
	ctx := finalizeTestContext(nil)
	step := &config.Step{FailedWhen: "true"}
	r := NewResult()
	r.Failed = false
	r.Rc = 0

	if err := applyResultOverrides(ctx, step, r); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !r.Failed {
		t.Errorf("failed_when:true should set Failed; got %+v", r)
	}
	if r.Rc != 0 {
		t.Errorf("Rc must not change to 'signal' failure; got %d", r.Rc)
	}
}

// TestApplyResultOverrides_ChangedWhenInverts — changed_when lets
// the operator declare "this didn't really change anything" even if
// the action's default would have set Changed=true.
func TestApplyResultOverrides_ChangedWhenInverts(t *testing.T) {
	ctx := finalizeTestContext(nil)
	r := NewResult()
	r.Changed = true

	if err := applyResultOverrides(ctx, &config.Step{ChangedWhen: "false"}, r); err != nil {
		t.Fatal(err)
	}
	if r.Changed {
		t.Errorf("changed_when:false should clear Changed; got %+v", r)
	}

	r.Changed = false
	if err := applyResultOverrides(ctx, &config.Step{ChangedWhen: "true"}, r); err != nil {
		t.Fatal(err)
	}
	if !r.Changed {
		t.Errorf("changed_when:true should set Changed; got %+v", r)
	}
}

// TestApplyResultOverrides_ExpressionSeesResult — the evaluation
// context exposes result.* via result.ToMap(), so expressions can
// branch on Rc / Stdout / etc. The shell tests historically asserted
// this; lifting that contract into the executor's test suite makes
// the dependency explicit.
func TestApplyResultOverrides_ExpressionSeesResult(t *testing.T) {
	ctx := finalizeTestContext(nil)
	step := &config.Step{FailedWhen: "result.rc == 42"}
	r := NewResult()
	r.Rc = 42

	if err := applyResultOverrides(ctx, step, r); err != nil {
		t.Fatal(err)
	}
	if !r.Failed {
		t.Errorf("failed_when on result.rc didn't fire: %+v", r)
	}
}

// TestApplyResultOverrides_ExpressionRenderError — a malformed
// template surfaces a structured error rather than crashing the
// step. Caller gets the rendered-failure error and stops; the
// result is unchanged.
func TestApplyResultOverrides_ExpressionRenderError(t *testing.T) {
	ctx := finalizeTestContext(nil)
	// {{ broken syntax — pongo2 surfaces a parse error from Render.
	step := &config.Step{FailedWhen: "{{ broken"}
	r := NewResult()

	err := applyResultOverrides(ctx, step, r)
	if err == nil {
		t.Error("expected render error on malformed expression")
	}
}

// TestApplyResultOverrides_NonBoolEvaluation — the override
// expression must evaluate to bool. If it returns a string or
// number, surface a typed error rather than silently coercing.
func TestApplyResultOverrides_NonBoolEvaluation(t *testing.T) {
	ctx := finalizeTestContext(nil)
	step := &config.Step{FailedWhen: "result.rc"} // int, not bool
	r := NewResult()
	r.Rc = 7

	err := applyResultOverrides(ctx, step, r)
	if err == nil {
		t.Error("expected error on non-bool override evaluation")
	}
}
