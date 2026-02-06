package vars

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "vars" {
		t.Errorf("Name = %v, want 'vars'", meta.Name)
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if meta.Category != actions.CategoryData {
		t.Errorf("Category = %v, want %v", meta.Category, actions.CategoryData)
	}
	if !meta.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
	}
	if meta.SupportsBecome {
		t.Error("SupportsBecome should be false")
	}
	if len(meta.EmitsEvents) != 1 {
		t.Errorf("EmitsEvents length = %d, want 1", len(meta.EmitsEvents))
	}
	if len(meta.EmitsEvents) > 0 && meta.EmitsEvents[0] != string(events.EventVarsSet) {
		t.Errorf("EmitsEvents[0] = %v, want %v", meta.EmitsEvents[0], string(events.EventVarsSet))
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Version = %v, want '1.0.0'", meta.Version)
	}
}

func TestHandler_Validate(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "valid vars action",
			step: &config.Step{
				Vars: &map[string]interface{}{
					"foo": "bar",
				},
			},
			wantErr: false,
		},
		{
			name: "nil vars action",
			step: &config.Step{
				Vars: nil,
			},
			wantErr: true,
		},
		{
			name: "empty vars map",
			step: &config.Step{
				Vars: &map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "multiple variables",
			step: &config.Step{
				Vars: &map[string]interface{}{
					"var1": "value1",
					"var2": 123,
					"var3": true,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.Validate(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name         string
		step         *config.Step
		existingVars map[string]interface{}
		wantVars     map[string]interface{}
		wantErr      bool
	}{
		{
			name: "set single variable",
			step: &config.Step{
				Vars: &map[string]interface{}{
					"foo": "bar",
				},
			},
			existingVars: map[string]interface{}{},
			wantVars: map[string]interface{}{
				"foo": "bar",
			},
			wantErr: false,
		},
		{
			name: "set multiple variables",
			step: &config.Step{
				Vars: &map[string]interface{}{
					"var1": "value1",
					"var2": 123,
					"var3": true,
				},
			},
			existingVars: map[string]interface{}{},
			wantVars: map[string]interface{}{
				"var1": "value1",
				"var2": 123,
				"var3": true,
			},
			wantErr: false,
		},
		{
			name: "merge with existing variables",
			step: &config.Step{
				Vars: &map[string]interface{}{
					"new_var": "new_value",
				},
			},
			existingVars: map[string]interface{}{
				"existing_var": "existing_value",
			},
			wantVars: map[string]interface{}{
				"existing_var": "existing_value",
				"new_var":      "new_value",
			},
			wantErr: false,
		},
		{
			name: "override existing variable",
			step: &config.Step{
				Vars: &map[string]interface{}{
					"foo": "new_value",
				},
			},
			existingVars: map[string]interface{}{
				"foo": "old_value",
			},
			wantVars: map[string]interface{}{
				"foo": "new_value",
			},
			wantErr: false,
		},
		{
			name: "set complex types",
			step: &config.Step{
				Vars: &map[string]interface{}{
					"array": []interface{}{"a", "b", "c"},
					"map": map[string]interface{}{
						"nested": "value",
					},
				},
			},
			existingVars: map[string]interface{}{},
			wantVars: map[string]interface{}{
				"array": []interface{}{"a", "b", "c"},
				"map": map[string]interface{}{
					"nested": "value",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutil.NewMockContext()
			ctx.Variables = tt.existingVars

			result, err := h.Execute(ctx, tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Check result properties
			execResult, ok := result.(*executor.Result)
			if !ok {
				t.Fatalf("Execute() result is not *executor.Result")
			}

			if execResult.Changed {
				t.Error("Result.Changed should be false for vars action")
			}

			// Check variables were set correctly
			for key, want := range tt.wantVars {
				got, exists := ctx.Variables[key]
				if !exists {
					t.Errorf("Variable %q not set", key)
					continue
				}

				// Compare values (simple comparison, doesn't handle deep nested structures)
				if !compareValues(got, want) {
					t.Errorf("Variable %q = %v, want %v", key, got, want)
				}
			}

			// Check event was published
			pub := ctx.Publisher
			if len(pub.Events) != 1 {
				t.Errorf("Expected 1 event to be published, got %d", len(pub.Events))
				return
			}

			event := pub.Events[0]
			if event.Type != events.EventVarsSet {
				t.Errorf("Event.Type = %v, want %v", event.Type, events.EventVarsSet)
			}

			varsData, ok := event.Data.(events.VarsSetData)
			if !ok {
				t.Fatalf("Event.Data is not events.VarsSetData")
			}

			if varsData.Count != len(*tt.step.Vars) {
				t.Errorf("VarsSetData.Count = %v, want %v", varsData.Count, len(*tt.step.Vars))
			}

			if len(varsData.Keys) != len(*tt.step.Vars) {
				t.Errorf("VarsSetData.Keys length = %v, want %v", len(varsData.Keys), len(*tt.step.Vars))
			}
		})
	}
}

func TestHandler_Execute_NilVars(t *testing.T) {
	h := &Handler{}
	ctx := testutil.NewMockContext()

	step := &config.Step{
		Vars: nil,
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when vars is nil")
	}
}

func TestHandler_Execute_NoPublisher(t *testing.T) {
	h := &Handler{}
	ctx := testutil.NewMockContext()
	ctx.Publisher = nil

	step := &config.Step{
		Vars: &map[string]interface{}{
			"foo": "bar",
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Errorf("Execute() should not error when publisher is nil, got: %v", err)
	}

	execResult, ok := result.(*executor.Result)
	if !ok {
		t.Fatalf("Execute() result is not *executor.Result")
	}

	if execResult.Changed {
		t.Error("Result.Changed should be false")
	}

	// Check variable was still set
	if got, exists := ctx.Variables["foo"]; !exists || got != "bar" {
		t.Errorf("Variable 'foo' = %v, want 'bar'", got)
	}
}

func TestHandler_DryRun(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name         string
		step         *config.Step
		existingVars map[string]interface{}
		wantVars     map[string]interface{}
		wantErr      bool
	}{
		{
			name: "dry-run sets variables",
			step: &config.Step{
				Vars: &map[string]interface{}{
					"foo": "bar",
				},
			},
			existingVars: map[string]interface{}{},
			wantVars: map[string]interface{}{
				"foo": "bar",
			},
			wantErr: false,
		},
		{
			name: "dry-run with multiple variables",
			step: &config.Step{
				Vars: &map[string]interface{}{
					"var1": "value1",
					"var2": 123,
				},
			},
			existingVars: map[string]interface{}{},
			wantVars: map[string]interface{}{
				"var1": "value1",
				"var2": 123,
			},
			wantErr: false,
		},
		{
			name: "dry-run nil vars",
			step: &config.Step{
				Vars: nil,
			},
			existingVars: map[string]interface{}{},
			wantVars:     map[string]interface{}{},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutil.NewMockContext()
			ctx.Variables = tt.existingVars
			ctx.DryRun = true

			err := h.DryRun(ctx, tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("DryRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// In dry-run mode, variables should still be set
			for key, want := range tt.wantVars {
				got, exists := ctx.Variables[key]
				if !exists {
					t.Errorf("Variable %q not set in dry-run", key)
					continue
				}

				if !compareValues(got, want) {
					t.Errorf("Variable %q = %v, want %v", key, got, want)
				}
			}

			// Check that something was logged
			log := ctx.Log.(*testutil.MockLogger)
			if len(log.Logs) == 0 {
				t.Error("DryRun() should log something")
			}
		})
	}
}

// compareValues compares two values for equality (simplified version)
func compareValues(a, b interface{}) bool {
	// This is a simplified comparison that works for basic types
	// For production code, you'd want a more robust comparison
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case int:
		bv, ok := b.(int)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !compareValues(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for k := range av {
			if !compareValues(av[k], bv[k]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}
