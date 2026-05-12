package template

import (
	"reflect"
	"testing"

	"github.com/alehatsman/mooncake/internal/expression"
)

func TestResolveList_TypedSliceVariable(t *testing.T) {
	vars := map[string]interface{}{
		"pkgs": []string{"a", "b", "c"},
	}
	got, err := ResolveList("pkgs", vars, expression.NewExprEvaluator())
	if err != nil {
		t.Fatalf("ResolveList: %v", err)
	}
	want := []interface{}{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveList_InterfaceSliceVariable(t *testing.T) {
	vars := map[string]interface{}{
		"pkgs": []interface{}{"x", "y"},
	}
	got, err := ResolveList("pkgs", vars, nil)
	if err != nil {
		t.Fatalf("ResolveList: %v", err)
	}
	if !reflect.DeepEqual(got, []interface{}{"x", "y"}) {
		t.Errorf("got %v, want [x y]", got)
	}
}

func TestResolveList_VarHoldingNonListIsStrict(t *testing.T) {
	vars := map[string]interface{}{
		"pkgs": "not a list",
	}
	_, err := ResolveList("pkgs", vars, nil)
	if err == nil {
		t.Fatal("expected error for non-list variable, got nil")
	}
}

func TestResolveList_DotNotation(t *testing.T) {
	vars := map[string]interface{}{
		"parameters": map[string]interface{}{
			"items": []interface{}{"a", "b"},
		},
	}
	got, err := ResolveList("parameters.items", vars, expression.NewExprEvaluator())
	if err != nil {
		t.Fatalf("ResolveList: %v", err)
	}
	if !reflect.DeepEqual(got, []interface{}{"a", "b"}) {
		t.Errorf("got %v, want [a b]", got)
	}
}

func TestResolveList_JSONLiteral(t *testing.T) {
	got, err := ResolveList(`["a","b","c"]`, nil, expression.NewExprEvaluator())
	if err != nil {
		t.Fatalf("ResolveList: %v", err)
	}
	if !reflect.DeepEqual(got, []interface{}{"a", "b", "c"}) {
		t.Errorf("got %v, want [a b c]", got)
	}
}

func TestResolveList_PongoStringifiedSlice(t *testing.T) {
	got, err := ResolveList("[a b c]", nil, expression.NewExprEvaluator())
	if err != nil {
		t.Fatalf("ResolveList: %v", err)
	}
	want := []interface{}{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveList_YAMLFlowSequence(t *testing.T) {
	got, err := ResolveList("[a, b, c]", nil, expression.NewExprEvaluator())
	if err != nil {
		t.Fatalf("ResolveList: %v", err)
	}
	want := []interface{}{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveList_WhitespaceScalar(t *testing.T) {
	got, err := ResolveList("a b c", nil, expression.NewExprEvaluator())
	if err != nil {
		t.Fatalf("ResolveList: %v", err)
	}
	want := []interface{}{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveStringList(t *testing.T) {
	vars := map[string]interface{}{
		"pkgs": []string{"vim", "git"},
	}
	got, err := ResolveStringList("pkgs", vars, expression.NewExprEvaluator())
	if err != nil {
		t.Fatalf("ResolveStringList: %v", err)
	}
	want := []string{"vim", "git"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
