package pilot

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestPromptExamples_DecodeAuto pins the contract from proposal-08:
// the JSON examples embedded in the system prompts must themselves
// decode cleanly through config.DecodeAuto into the kernel's Step
// shape. If someone edits the example strings, this test fails
// before the change reaches the model.
func TestPromptExamples_DecodeAuto(t *testing.T) {
	examples := []struct {
		name string
		body string
	}{
		{
			name: "plan-style preamble example",
			body: `[{"name":"create greeting","cmd":{"argv":["sh","-c","echo hello > /tmp/greeting.txt"]}}]`,
		},
		{
			name: "step-style fragment example",
			body: `[{"name":"check git status","cmd":{"argv":["git","status","--short"]}}]`,
		},
	}
	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			var steps []config.Step
			if err := config.DecodeAuto([]byte(ex.body), &steps); err != nil {
				t.Fatalf("DecodeAuto(%q): %v", ex.body, err)
			}
			if len(steps) != 1 {
				t.Fatalf("expected 1 step, got %d", len(steps))
			}
			if steps[0].Name == "" {
				t.Errorf("step.Name not decoded; full step: %+v", steps[0])
			}
			if steps[0].Cmd == nil {
				t.Errorf("step.Cmd not decoded; full step: %+v", steps[0])
			}
		})
	}
}
