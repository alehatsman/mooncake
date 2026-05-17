package actions

import "fmt"

// EvaluateBoolExpression renders `expression` through the Context's
// template engine, evaluates the rendered string through the Context's
// expression evaluator, and asserts the result is a bool. The
// `fieldName` argument tags the error message so callers (failed_when /
// changed_when / unless / etc.) don't have to wrap the error with the
// same prefix.
//
// Previously each of the shell, command, and assert handlers carried
// its own copy of this helper. The shell variant tagged errors with a
// field name; command + assert wrapped the inner error with the field
// name at the call site. Both shapes collapse here: handler callers
// pass the field name, the helper produces a single uniform error
// format, and the three copies become one.
func EvaluateBoolExpression(ctx Context, fieldName, expression string, evalContext map[string]interface{}) (bool, error) {
	rendered, err := ctx.GetTemplate().Render(expression, evalContext)
	if err != nil {
		return false, fmt.Errorf("failed to render %s: %w", fieldName, err)
	}
	result, err := ctx.GetEvaluator().Evaluate(rendered, evalContext)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate %s: %w", fieldName, err)
	}
	b, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("%s expression evaluated to %T, expected bool", fieldName, result)
	}
	return b, nil
}
