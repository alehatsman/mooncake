// Package expression provides expression evaluation capabilities for conditional execution.
package expression

import (
	"strings"

	"github.com/expr-lang/expr"
)

// Evaluator defines the interface for evaluating expressions
type Evaluator interface {
	Evaluate(expression string, variables map[string]interface{}) (interface{}, error)
}

// ExprEvaluator implements Evaluator using the expr-lang library.
type ExprEvaluator struct {
	// Store any evaluator-specific state here if needed
}

// NewExprEvaluator creates a new ExprEvaluator.
func NewExprEvaluator() Evaluator {
	return &ExprEvaluator{}
}

// NewGovaluateEvaluator is kept for backwards compatibility, now returns ExprEvaluator.
func NewGovaluateEvaluator() Evaluator {
	return NewExprEvaluator()
}

// Evaluate evaluates an expression with the given variables
func (e *ExprEvaluator) Evaluate(expression string, variables map[string]interface{}) (interface{}, error) {
	if variables == nil {
		variables = make(map[string]interface{})
	}

	// Compile and evaluate the expression with custom functions
	program, err := expr.Compile(expression,
		expr.Env(variables),
		// Allow undefined variables - they will evaluate to nil
		// This is important for when conditions that reference variables
		// from steps that haven't run yet or were skipped
		expr.AllowUndefinedVariables(),
		// Add string functions (note: "contains" is reserved as an operator in expr)
		// So we use "has" as a shorter alternative
		expr.Function("has", func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return false, nil
			}
			str, ok1 := params[0].(string)
			substr, ok2 := params[1].(string)
			if !ok1 || !ok2 {
				return false, nil
			}
			return strings.Contains(str, substr), nil
		}),
	)
	if err != nil {
		return nil, err
	}

	result, err := expr.Run(program, variables)
	if err != nil {
		return nil, err
	}

	return result, nil
}

