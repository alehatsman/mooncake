package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderTemplateSuccess(t *testing.T) {
	result, err := renderTemplate("{{ foo }}", Context{"foo": "bar"})
	assert.NoError(t, err)
	assert.Equal(t, "bar", result)
}

func TestRenderTemplateError(t *testing.T) {
	_, err := renderTemplate("{{ foo }", Context{"foo": "bar"})
	assert.Error(t, err)
}

func TestRenderTemplateExecutionError(t *testing.T) {
	_, err := renderTemplate("{{ foo }}", Context{})
	assert.Error(t, err)
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
