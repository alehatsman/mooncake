package expression

import (
	"testing"
)

func TestGovaluateEvaluator_Evaluate(t *testing.T) {
	evaluator := NewGovaluateEvaluator()

	tests := []struct {
		name       string
		expression string
		vars       map[string]interface{}
		want       interface{}
		wantErr    bool
	}{
		{
			name:       "simple true",
			expression: "true",
			vars:       map[string]interface{}{},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "simple false",
			expression: "false",
			vars:       map[string]interface{}{},
			want:       false,
			wantErr:    false,
		},
		{
			name:       "equality true",
			expression: "x == 5",
			vars:       map[string]interface{}{"x": 5},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "equality false",
			expression: "x == 5",
			vars:       map[string]interface{}{"x": 10},
			want:       false,
			wantErr:    false,
		},
		{
			name:       "greater than",
			expression: "x > 5",
			vars:       map[string]interface{}{"x": 10},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "less than",
			expression: "x < 5",
			vars:       map[string]interface{}{"x": 3},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "and operator",
			expression: "x > 5 && y < 10",
			vars:       map[string]interface{}{"x": 7, "y": 8},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "or operator",
			expression: "x > 5 || y > 10",
			vars:       map[string]interface{}{"x": 3, "y": 15},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "not operator",
			expression: "!flag",
			vars:       map[string]interface{}{"flag": false},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "string equality",
			expression: "name == 'test'",
			vars:       map[string]interface{}{"name": "test"},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "arithmetic",
			expression: "x + y",
			vars:       map[string]interface{}{"x": 5.0, "y": 3.0},
			want:       8.0,
			wantErr:    false,
		},
		{
			name:       "arithmetic multiplication",
			expression: "x * y",
			vars:       map[string]interface{}{"x": 4.0, "y": 3.0},
			want:       12.0,
			wantErr:    false,
		},
		{
			name:       "nil variables map",
			expression: "true",
			vars:       nil,
			want:       true,
			wantErr:    false,
		},
		{
			name:       "missing variable",
			expression: "missing == 5",
			vars:       map[string]interface{}{},
			want:       nil,
			wantErr:    true,
		},
		{
			name:       "invalid expression",
			expression: "invalid syntax +++",
			vars:       map[string]interface{}{},
			want:       nil,
			wantErr:    true,
		},
		{
			name:       "parentheses",
			expression: "(x + y) * z",
			vars:       map[string]interface{}{"x": 2.0, "y": 3.0, "z": 4.0},
			want:       20.0,
			wantErr:    false,
		},
		{
			name:       "complex condition",
			expression: "(x > 5 && y < 10) || z == 'active'",
			vars:       map[string]interface{}{"x": 7, "y": 8, "z": "active"},
			want:       true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluator.Evaluate(tt.expression, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestGovaluateEvaluator_EvaluateOS(t *testing.T) {
	evaluator := NewGovaluateEvaluator()

	// Test OS-specific expressions (like used in mooncake)
	tests := []struct {
		name       string
		expression string
		vars       map[string]interface{}
		wantType   string
	}{
		{
			name:       "os equals darwin",
			expression: "os == 'darwin'",
			vars:       map[string]interface{}{"os": "darwin"},
			wantType:   "bool",
		},
		{
			name:       "os equals linux",
			expression: "os == 'linux'",
			vars:       map[string]interface{}{"os": "linux"},
			wantType:   "bool",
		},
		{
			name:       "complex os check",
			expression: "os == 'darwin' || os == 'linux'",
			vars:       map[string]interface{}{"os": "darwin"},
			wantType:   "bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, tt.vars)
			if err != nil {
				t.Errorf("Evaluate() unexpected error = %v", err)
				return
			}
			if tt.wantType == "bool" {
				if _, ok := result.(bool); !ok {
					t.Errorf("Evaluate() result type = %T, want bool", result)
				}
			}
		})
	}
}

func TestNewGovaluateEvaluator(t *testing.T) {
	evaluator := NewGovaluateEvaluator()
	if evaluator == nil {
		t.Error("NewGovaluateEvaluator() returned nil")
	}

	// Verify it returns the interface
	var _ Evaluator = evaluator
}

func TestGovaluateEvaluator_Concurrent(t *testing.T) {
	evaluator := NewGovaluateEvaluator()

	// Test that evaluator is safe for concurrent use
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			expression := "x == 5"
			vars := map[string]interface{}{"x": 5}
			result, err := evaluator.Evaluate(expression, vars)
			if err != nil {
				t.Errorf("Concurrent evaluate failed: %v", err)
			}
			if result != true {
				t.Errorf("Concurrent evaluate got %v, want true", result)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
