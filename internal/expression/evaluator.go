package expression

import (
	"github.com/Knetic/govaluate"
)

// Evaluator defines the interface for evaluating expressions
type Evaluator interface {
	Evaluate(expression string, variables map[string]interface{}) (interface{}, error)
}

// GovaluateEvaluator implements Evaluator using the govaluate library
type GovaluateEvaluator struct {
	// Store any evaluator-specific state here if needed
}

// NewGovaluateEvaluator creates a new GovaluateEvaluator
func NewGovaluateEvaluator() Evaluator {
	return &GovaluateEvaluator{}
}

// Evaluate evaluates an expression with the given variables
func (e *GovaluateEvaluator) Evaluate(expression string, variables map[string]interface{}) (interface{}, error) {
	if variables == nil {
		variables = make(map[string]interface{})
	}

	evaluableExpression, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		return nil, err
	}

	evalResult, err := evaluableExpression.Evaluate(variables)
	if err != nil {
		return nil, err
	}

	return evalResult, nil
}
