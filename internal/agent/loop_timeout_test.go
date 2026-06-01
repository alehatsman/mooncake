package agent

import (
	"testing"
	"time"
)

// TestRunOptionsPlanGenTimeout pins the per-iteration LLM-budget resolution
// (#80): an explicit positive LLMTimeout wins; zero/negative falls back to the
// built-in default so existing callers are unchanged.
func TestRunOptionsPlanGenTimeout(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"zero falls back to default", 0, defaultPlanGenTimeout},
		{"negative falls back to default", -1 * time.Second, defaultPlanGenTimeout},
		{"override honored", 20 * time.Minute, 20 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RunOptions{LLMTimeout: tt.in}.planGenTimeout()
			if got != tt.want {
				t.Errorf("planGenTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}
