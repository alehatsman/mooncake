package executor

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/template"
)

// TestExecutionContext_GetEvaluator tests GetEvaluator method
func TestExecutionContext_GetEvaluator(t *testing.T) {
	ctx := &ExecutionContext{
		Evaluator: expression.NewGovaluateEvaluator(),
	}
	if ctx.GetEvaluator() == nil {
		t.Error("GetEvaluator() should return non-nil evaluator")
	}
}

func TestExecutionContext_GetEvaluator_Nil(t *testing.T) {
	ctx := &ExecutionContext{}
	if ctx.GetEvaluator() != nil {
		t.Error("GetEvaluator() should return nil when evaluator is not set")
	}
}

func TestExecutionContext_GetTemplate(t *testing.T) {
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	ctx := &ExecutionContext{Template: tmpl}
	if ctx.GetTemplate() == nil {
		t.Error("GetTemplate() should return non-nil template")
	}
}

// TestExecutionContext_EmitEvent_NilPublisher tests EmitEvent with nil
// publisher (no-op, should not panic).
func TestExecutionContext_EmitEvent_NilPublisher(t *testing.T) {
	ctx := &ExecutionContext{}
	ctx.EmitEvent("test_event", map[string]interface{}{"key": "value"})
}

// TestExecutionContext_Mode verifies CurrentMode round-trips through
// Mode(). Spec 16 stores the mode as a single field.
func TestExecutionContext_Mode(t *testing.T) {
	for _, tt := range []struct {
		name string
		mode actions.Mode
	}{
		{"execute", actions.ModeApply},
		{"plan", actions.ModePlan},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &ExecutionContext{CurrentMode: tt.mode}
			if got := ctx.Mode(); got != tt.mode {
				t.Errorf("Mode() = %v, want %v", got, tt.mode)
			}
		})
	}
}

func TestMode_String(t *testing.T) {
	for _, tt := range []struct {
		mode actions.Mode
		want string
	}{
		{actions.ModeApply, "apply"},
		{actions.ModePlan, "plan"},
		{actions.Mode(99), "unknown"},
	} {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

// TestNewExecutionContext tests basic context construction.
func TestNewExecutionContext(t *testing.T) {
	testLogger := logger.NewTestLogger()
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	eval := expression.NewGovaluateEvaluator()

	ctx := &ExecutionContext{
		Logger:      testLogger,
		Template:    tmpl,
		Evaluator:   eval,
		CurrentMode: actions.ModeApply,
	}

	if ctx.Logger == nil {
		t.Error("Logger should not be nil")
	}
	if ctx.Template == nil {
		t.Error("Template should not be nil")
	}
	if ctx.Evaluator == nil {
		t.Error("Evaluator should not be nil")
	}
	if ctx.Mode() != actions.ModeApply {
		t.Errorf("Mode = %v, want ModeApply", ctx.Mode())
	}
}
