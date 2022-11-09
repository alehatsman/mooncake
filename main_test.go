package main

import (
	"testing"
)

func TestRenderTemplate(t *testing.T) {
	result, err := renderTemplate("{{ foo }}", Context{"foo": "bar"})
	if err != nil {
		t.Error(err)
	}

	if result != "bar" {
		t.Errorf("Expected 'bar', got '%s'", result)
	}
}

func TestEvaluateExpression(t *testing.T) {
	result, err := evaluateExpression("foo == 'bar'", Context{"foo": "bar"})
	if err != nil {
		t.Error(err)
	}

	if result != true {
		t.Errorf("Expected 'true', got '%v'", result)
	}

	result, err = evaluateExpression("foo == 'bar'", Context{"foo": "baz"})
	if err != nil {
		t.Error(err)
	}

	if result != false {
		t.Errorf("Expected 'false', got '%v'", result)
	}
}
