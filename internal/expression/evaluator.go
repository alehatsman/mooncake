// Package expression provides expression evaluation capabilities for conditional execution.
package expression

import (
	"fmt"
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

	// Build compilation options
	opts := []expr.Option{
		expr.Env(variables),
		// Allow undefined variables - they will evaluate to nil
		// This is important for when conditions that reference variables
		// from steps that haven't run yet or were skipped
		expr.AllowUndefinedVariables(),
	}

	// Add custom functions
	// Legacy "has" function for string containment (kept for backward compatibility)
	opts = append(opts, expr.Function("has", func(params ...interface{}) (interface{}, error) {
		if len(params) != 2 {
			return false, nil
		}
		str, ok1 := params[0].(string)
		substr, ok2 := params[1].(string)
		if !ok1 || !ok2 {
			return false, nil
		}
		return strings.Contains(str, substr), nil
	}))

	// Add all enhanced function library
	allFunctions := AllFunctions()
	for name, fn := range allFunctions {
		// Type assertion: all our functions have this signature
		funcImpl, ok := fn.(func(...interface{}) (interface{}, error))
		if !ok {
			return nil, fmt.Errorf("internal error: function %s has invalid signature", name)
		}
		opts = append(opts, expr.Function(name, funcImpl))
	}

	// Compile the expression
	program, err := expr.Compile(expression, opts...)
	if err != nil {
		return nil, enhanceCompileError(err, expression)
	}

	// Run the expression
	result, err := expr.Run(program, variables)
	if err != nil {
		return nil, enhanceRuntimeError(err, expression)
	}

	return result, nil
}

// enhanceCompileError provides better error messages for compilation errors
func enhanceCompileError(err error, expression string) error {
	errMsg := err.Error()

	// Common error patterns and their enhanced messages
	if strings.Contains(errMsg, "unexpected token") {
		return fmt.Errorf("syntax error in expression %q: %w\nCheck for mismatched parentheses, quotes, or operators", expression, err)
	}
	if strings.Contains(errMsg, "undefined") {
		return fmt.Errorf("undefined function or variable in expression %q: %w\nAvailable functions: has, starts_with, ends_with, lower, upper, trim, split, join, replace, regex_match, min, max, abs, floor, ceil, round, pow, sqrt, len, includes, empty, first, last, is_string, is_number, is_bool, is_array, is_map, is_defined, default, env, has_env, coalesce, ternary", expression, err)
	}

	return fmt.Errorf("failed to compile expression %q: %w", expression, err)
}

// enhanceRuntimeError provides better error messages for runtime errors
func enhanceRuntimeError(err error, expression string) error {
	errMsg := err.Error()

	// Common error patterns and their enhanced messages
	if strings.Contains(errMsg, "index out of range") {
		return fmt.Errorf("array index out of bounds in expression %q: %w", expression, err)
	}
	if strings.Contains(errMsg, "cannot") && strings.Contains(errMsg, "type") {
		return fmt.Errorf("type error in expression %q: %w\nCheck that operations match their operand types", expression, err)
	}
	if strings.Contains(errMsg, "nil") {
		return fmt.Errorf("nil value error in expression %q: %w\nUse is_defined() or default() to handle nil values", expression, err)
	}

	return fmt.Errorf("failed to evaluate expression %q: %w", expression, err)
}
