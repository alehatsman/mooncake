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

func TestEvaluate_RuntimeErrors(t *testing.T) {
	evaluator := NewExprEvaluator()

	tests := []struct {
		name       string
		expression string
		variables  map[string]interface{}
		wantErr    bool
	}{
		{
			name:       "array index out of bounds",
			expression: "arr[10]",
			variables:  map[string]interface{}{"arr": []int{1, 2, 3}},
			wantErr:    true,
		},
		{
			name:       "nil pointer access",
			expression: "obj.field",
			variables:  map[string]interface{}{"obj": nil},
			wantErr:    true,
		},
		{
			name:       "undefined function call",
			expression: "unknownFunc(x)",
			variables:  map[string]interface{}{"x": 5},
			wantErr:    true,
		},
		{
			name:       "invalid method call on string",
			expression: "str.nonExistentMethod()",
			variables:  map[string]interface{}{"str": "test"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evaluator.Evaluate(tt.expression, tt.variables)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainsFunction(t *testing.T) {
	evaluator := NewExprEvaluator()

	tests := []struct {
		name      string
		expr      string
		vars      map[string]interface{}
		want      bool
		wantError bool
	}{
		{
			name: "contains with string found",
			expr: `has(result.stdout, "hello")`,
			vars: map[string]interface{}{
				"result": map[string]interface{}{
					"stdout": "hello world",
				},
			},
			want: true,
		},
		{
			name: "contains not found",
			expr: `has(result.stdout, "goodbye")`,
			vars: map[string]interface{}{
				"result": map[string]interface{}{
					"stdout": "hello world",
				},
			},
			want: false,
		},
		{
			name: "contains empty substring",
			expr: `has("test", "")`,
			vars: map[string]interface{}{},
			want: true,
		},
		{
			name: "contains case sensitive",
			expr: `has("Hello", "hello")`,
			vars: map[string]interface{}{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expr, tt.vars)
			if tt.wantError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if result != tt.want {
				t.Errorf("Evaluate() = %v, want %v", result, tt.want)
			}
		})
	}
}
