package expression

import (
	"os"
	"strings"
	"testing"
)

func TestStringFunctions(t *testing.T) {
	evaluator := NewExprEvaluator()

	tests := []struct {
		name  string
		expr  string
		vars  map[string]interface{}
		want  interface{}
		error bool
	}{
		// startsWith
		{
			name: "starts_with true",
			expr: `starts_with("hello world", "hello")`,
			want: true,
		},
		{
			name: "starts_with false",
			expr: `starts_with("hello world", "world")`,
			want: false,
		},

		// endsWith
		{
			name: "ends_with true",
			expr: `ends_with("hello world", "world")`,
			want: true,
		},
		{
			name: "ends_with false",
			expr: `ends_with("hello world", "hello")`,
			want: false,
		},

		// lower
		{
			name: "lower",
			expr: `lower("HELLO World")`,
			want: "hello world",
		},

		// upper
		{
			name: "upper",
			expr: `upper("hello World")`,
			want: "HELLO WORLD",
		},

		// trim
		{
			name: "trim",
			expr: `trim("  hello world  ")`,
			want: "hello world",
		},

		// split
		{
			name: "split",
			expr: `len(split("a,b,c", ","))`,
			want: 3,
		},

		// join
		{
			name: "join",
			expr: `join([1, 2, 3], ",")`,
			want: "1,2,3",
		},

		// replace
		{
			name: "replace",
			expr: `replace("hello world", "world", "golang")`,
			want: "hello golang",
		},

		// matches
		{
			name: "regex_match true",
			expr: `regex_match("test123", "^test[0-9]+$")`,
			want: true,
		},
		{
			name: "regex_match false",
			expr: `regex_match("test", "^test[0-9]+$")`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expr, tt.vars)
			if tt.error {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if result != tt.want {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

func TestMathFunctions(t *testing.T) {
	evaluator := NewExprEvaluator()

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{
			name: "min",
			expr: `min(5.0, 3.0)`,
			want: 3.0,
		},
		{
			name: "max",
			expr: `max(5.0, 3.0)`,
			want: 5.0,
		},
		{
			name: "abs positive",
			expr: `abs(5.0)`,
			want: 5.0,
		},
		{
			name: "abs negative",
			expr: `abs(-5.0)`,
			want: 5.0,
		},
		{
			name: "floor",
			expr: `floor(5.7)`,
			want: 5.0,
		},
		{
			name: "ceil",
			expr: `ceil(5.3)`,
			want: 6.0,
		},
		{
			name: "round down",
			expr: `round(5.4)`,
			want: 5.0,
		},
		{
			name: "round up",
			expr: `round(5.6)`,
			want: 6.0,
		},
		{
			name: "pow",
			expr: `pow(2.0, 3.0)`,
			want: 8.0,
		},
		{
			name: "sqrt",
			expr: `sqrt(16.0)`,
			want: 4.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expr, nil)
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

func TestCollectionFunctions(t *testing.T) {
	evaluator := NewExprEvaluator()

	tests := []struct {
		name string
		expr string
		vars map[string]interface{}
		want interface{}
	}{
		{
			name: "len string",
			expr: `len("hello")`,
			want: 5,
		},
		{
			name: "len array",
			expr: `len(arr)`,
			vars: map[string]interface{}{"arr": []int{1, 2, 3}},
			want: 3,
		},
		{
			name: "len map",
			expr: `len(m)`,
			vars: map[string]interface{}{"m": map[string]int{"a": 1, "b": 2}},
			want: 2,
		},
		{
			name: "len nil",
			expr: `len(nil)`,
			want: 0,
		},
		{
			name: "includestrue",
			expr: `includes(2, arr)`,
			vars: map[string]interface{}{"arr": []int{1, 2, 3}},
			want: true,
		},
		{
			name: "includesfalse",
			expr: `includes(5, arr)`,
			vars: map[string]interface{}{"arr": []int{1, 2, 3}},
			want: false,
		},
		{
			name: "empty string true",
			expr: `empty("")`,
			want: true,
		},
		{
			name: "empty string false",
			expr: `empty("hello")`,
			want: false,
		},
		{
			name: "empty array true",
			expr: `empty([])`,
			want: true,
		},
		{
			name: "empty array false",
			expr: `empty([1, 2])`,
			want: false,
		},
		{
			name: "empty nil",
			expr: `empty(nil)`,
			want: true,
		},
		{
			name: "first",
			expr: `first(arr)`,
			vars: map[string]interface{}{"arr": []int{10, 20, 30}},
			want: 10,
		},
		{
			name: "first empty",
			expr: `first([])`,
			want: nil,
		},
		{
			name: "last",
			expr: `last(arr)`,
			vars: map[string]interface{}{"arr": []int{10, 20, 30}},
			want: 30,
		},
		{
			name: "last empty",
			expr: `last([])`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expr, tt.vars)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if result != tt.want {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

func TestTypeFunctions(t *testing.T) {
	evaluator := NewExprEvaluator()

	tests := []struct {
		name string
		expr string
		vars map[string]interface{}
		want bool
	}{
		{
			name: "is_string true",
			expr: `is_string("hello")`,
			want: true,
		},
		{
			name: "is_string false",
			expr: `is_string(123)`,
			want: false,
		},
		{
			name: "is_number true int",
			expr: `is_number(123)`,
			want: true,
		},
		{
			name: "is_number true float",
			expr: `is_number(123.45)`,
			want: true,
		},
		{
			name: "is_number false",
			expr: `is_number("123")`,
			want: false,
		},
		{
			name: "is_bool true",
			expr: `is_bool(true)`,
			want: true,
		},
		{
			name: "is_bool false",
			expr: `is_bool(1)`,
			want: false,
		},
		{
			name: "is_array true",
			expr: `is_array([1, 2, 3])`,
			want: true,
		},
		{
			name: "is_array false",
			expr: `is_array("123")`,
			want: false,
		},
		{
			name: "is_map true",
			expr: `is_map(m)`,
			vars: map[string]interface{}{"m": map[string]int{"a": 1}},
			want: true,
		},
		{
			name: "is_map false",
			expr: `is_map([1, 2])`,
			want: false,
		},
		{
			name: "is_defined true",
			expr: `is_defined(x)`,
			vars: map[string]interface{}{"x": 123},
			want: true,
		},
		{
			name: "is_defined false",
			expr: `is_defined(nil)`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expr, tt.vars)
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

func TestUtilityFunctions(t *testing.T) {
	evaluator := NewExprEvaluator()

	// Set test environment variable
	os.Setenv("TEST_ENV_VAR", "test_value")
	defer os.Unsetenv("TEST_ENV_VAR")

	tests := []struct {
		name string
		expr string
		vars map[string]interface{}
		want interface{}
	}{
		{
			name: "default with nil",
			expr: `default(nil, "fallback")`,
			want: "fallback",
		},
		{
			name: "default with value",
			expr: `default("value", "fallback")`,
			want: "value",
		},
		{
			name: "default with empty string",
			expr: `default("", "fallback")`,
			want: "fallback",
		},
		{
			name: "env existing",
			expr: `env("TEST_ENV_VAR")`,
			want: "test_value",
		},
		{
			name: "env non-existing",
			expr: `env("NON_EXISTING_VAR")`,
			want: "",
		},
		{
			name: "has_env true",
			expr: `has_env("TEST_ENV_VAR")`,
			want: true,
		},
		{
			name: "has_env false",
			expr: `has_env("NON_EXISTING_VAR")`,
			want: false,
		},
		{
			name: "coalesce first",
			expr: `coalesce("first", "second", "third")`,
			want: "first",
		},
		{
			name: "coalesce second",
			expr: `coalesce(nil, "second", "third")`,
			want: "second",
		},
		{
			name: "coalesce all nil",
			expr: `coalesce(nil, nil, nil)`,
			want: nil,
		},
		{
			name: "ternary true",
			expr: `ternary(true, "yes", "no")`,
			want: "yes",
		},
		{
			name: "ternary false",
			expr: `ternary(false, "yes", "no")`,
			want: "no",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expr, tt.vars)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if result != tt.want {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

func TestComplexExpressions(t *testing.T) {
	evaluator := NewExprEvaluator()

	tests := []struct {
		name string
		expr string
		vars map[string]interface{}
		want interface{}
	}{
		{
			name: "complex condition with string and math",
			expr: `(os == "linux" and cpu_cores >= 4) or memory_total_mb > 16000`,
			vars: map[string]interface{}{
				"os":              "linux",
				"cpu_cores":       8,
				"memory_total_mb": 8000,
			},
			want: true,
		},
		{
			name: "string manipulation chain",
			expr: `upper(trim(replace(str, "world", "golang")))`,
			vars: map[string]interface{}{
				"str": "  hello world  ",
			},
			want: "HELLO GOLANG",
		},
		{
			name: "array operations",
			expr: `len(items) > 0 and includes("required", items) and first(items) != ""`,
			vars: map[string]interface{}{
				"items": []string{"required", "optional", "extra"},
			},
			want: true,
		},
		{
			name: "type checking and defaults",
			expr: `is_defined(config) and is_string(config.name) and default(config.port, 8080)`,
			vars: map[string]interface{}{
				"config": map[string]interface{}{
					"name": "myapp",
				},
			},
			want: 8080,
		},
		{
			name: "math and comparison",
			expr: `max(min(value, 100.0), 0.0) > 50.0`,
			vars: map[string]interface{}{
				"value": 75.0,
			},
			want: true,
		},
		{
			name: "ternary with complex condition",
			expr: `ternary(len(items) > 5, "many", "few")`,
			vars: map[string]interface{}{
				"items": []int{1, 2, 3, 4, 5, 6},
			},
			want: "many",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expr, tt.vars)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if result != tt.want {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

func TestEnhancedErrorMessages(t *testing.T) {
	evaluator := NewExprEvaluator()

	tests := []struct {
		name        string
		expr        string
		vars        map[string]interface{}
		wantErr     bool
		errContains string
	}{
		{
			name:        "syntax error",
			expr:        "(((",
			wantErr:     true,
			errContains: "syntax error",
		},
		{
			name:        "undefined function",
			expr:        "unknownFunc(x)",
			vars:        map[string]interface{}{"x": 5},
			wantErr:     true,
			errContains: "nil value",
		},
		{
			name:        "array out of bounds",
			expr:        "arr[10]",
			vars:        map[string]interface{}{"arr": []int{1, 2, 3}},
			wantErr:     true,
			errContains: "out of bounds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evaluator.Evaluate(tt.expr, tt.vars)
			if !tt.wantErr {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Error("Expected error but got none")
				return
			}
			if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error %q does not contain %q", err.Error(), tt.errContains)
			}
		})
	}
}
