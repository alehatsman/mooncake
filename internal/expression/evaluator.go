// Package expression provides expression evaluation capabilities for conditional execution.
package expression

import (
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

	// Compile and evaluate the expression
	program, err := expr.Compile(expression, expr.Env(variables))
	if err != nil {
		return nil, err
	}

	result, err := expr.Run(program, variables)
	if err != nil {
		return nil, err
	}

	return result, nil
}
