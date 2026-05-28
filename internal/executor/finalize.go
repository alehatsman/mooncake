package executor

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// applyResultOverrides applies changed_when / failed_when expressions
// to a step's result post-retry. Lifted from internal/actions/shell/
// handler.go (spec-69 phase 3) so the same logic doesn't have to be
// duplicated across shell + command + every future action that wants
// override semantics.
//
// The function mutates result.Changed / result.Failed in place and
// returns a non-nil error iff result.Failed is still true after the
// failed_when evaluation. result is unchanged when both override
// fields are empty.
//
// Issue #21: do NOT fabricate Rc=1 to signal failure — that lied to
// the operator ("command failed with exit code 1") when the
// underlying command exited 0. Leave result.Rc reflecting the actual
// underlying exit code; the caller's error-message path detects
// Rc==0 && Failed==true and surfaces a failed-by-failed_when message.
func applyResultOverrides(ctx actions.Context, step *config.Step, result *Result) error {
	if step.ChangedWhen == "" && step.FailedWhen == "" {
		return nil
	}

	evalContext := make(map[string]interface{}, len(ctx.Variables())+1)
	for k, v := range ctx.Variables() {
		evalContext[k] = v
	}
	evalContext["result"] = result.ToMap()

	if step.ChangedWhen != "" {
		boolResult, err := evalOverrideBool(ctx, step.ChangedWhen, "changed_when", evalContext)
		if err != nil {
			return err
		}
		result.Changed = boolResult
	}
	if step.FailedWhen != "" {
		boolResult, err := evalOverrideBool(ctx, step.FailedWhen, "failed_when", evalContext)
		if err != nil {
			return err
		}
		result.Failed = boolResult
	}
	return nil
}

// evalOverrideBool renders and evaluates a single override expression.
// The two-step (render + evaluate) shape mirrors what the shell handler
// did pre-spec-69 so existing expressions (including Pongo2 filters in
// the expression text) keep working unchanged.
func evalOverrideBool(ctx actions.Context, expression, fieldName string, evalContext map[string]interface{}) (bool, error) {
	rendered, err := ctx.Template().Render(expression, evalContext)
	if err != nil {
		return false, fmt.Errorf("failed to render %s: %w", fieldName, err)
	}
	out, err := ctx.Evaluator().Evaluate(rendered, evalContext)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate %s: %w", fieldName, err)
	}
	b, ok := out.(bool)
	if !ok {
		return false, fmt.Errorf("%s expression evaluated to %T, expected bool", fieldName, out)
	}
	return b, nil
}
