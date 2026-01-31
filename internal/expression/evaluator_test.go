package expression

import (
	"testing"
)

func TestNewExprEvaluator(t *testing.T) {
	evaluator := NewExprEvaluator()
	if evaluator == nil {
		t.Error("NewExprEvaluator() should not return nil")
	}
}

func TestNewGovaluateEvaluator(t *testing.T) {
	evaluator := NewGovaluateEvaluator()
	if evaluator == nil {
		t.Error("NewGovaluateEvaluator() should not return nil")
	}
}

func TestEvaluate_Success(t *testing.T) {
	evaluator := NewExprEvaluator()

	tests := []struct {
		name       string
		expression string
		variables  map[string]interface{}
		want       interface{}
	}{
		{
			name:       "simple true",
			expression: "true",
			variables:  nil,
			want:       true,
		},
		{
			name:       "simple false",
			expression: "false",
			variables:  map[string]interface{}{},
			want:       false,
		},
		{
			name:       "variable equals",
			expression: "x == 5",
			variables:  map[string]interface{}{"x": 5},
			want:       true,
		},
		{
			name:       "variable not equals",
			expression: "x == 10",
			variables:  map[string]interface{}{"x": 5},
			want:       false,
		},
		{
			name:       "arithmetic",
			expression: "x + y",
			variables:  map[string]interface{}{"x": 10, "y": 5},
			want:       int(15),
		},
		{
			name:       "string comparison",
			expression: "name == 'test'",
			variables:  map[string]interface{}{"name": "test"},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, tt.variables)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if result != tt.want {
				t.Errorf("Evaluate() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestEvaluate_CompileError(t *testing.T) {
	evaluator := NewExprEvaluator()

	// Invalid syntax should cause compile error
	_, err := evaluator.Evaluate("invalid syntax {{", nil)
	if err == nil {
		t.Error("Evaluate() should return error for invalid syntax")
	}
}

func TestEvaluate_RuntimeError(t *testing.T) {
	evaluator := NewExprEvaluator()

	// Invalid type operation should cause runtime error
	_, err := evaluator.Evaluate("x.nonexistent", map[string]interface{}{"x": 5})
	if err == nil {
		t.Error("Evaluate() should return error for invalid operation")
	}
}

func TestEvaluate_NilVariables(t *testing.T) {
	evaluator := NewExprEvaluator()

	// Should handle nil variables by creating empty map
	result, err := evaluator.Evaluate("true", nil)
	if err != nil {
		t.Errorf("Evaluate() with nil variables error = %v", err)
	}
	if result != true {
		t.Errorf("Evaluate() = %v, want true", result)
	}
}

func TestEvaluate_AllErrorPaths(t *testing.T) {
	evaluator := NewExprEvaluator()

	// Test compile error path (line 36)
	_, err := evaluator.Evaluate("(((", nil)
	if err == nil {
		t.Error("Should error on malformed expression")
	}

	// Test runtime error with type mismatch
	_, err = evaluator.Evaluate("'string' + 5", nil)
	if err == nil {
		t.Error("Should error on type mismatch")
	}
}
