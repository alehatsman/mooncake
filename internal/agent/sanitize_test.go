package agent

import (
	"strings"
	"testing"
)

func TestSanitizePlan(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "yaml fence",
			input:    "```yaml\n- shell:\n    cmd: echo hello\n```",
			expected: "- shell:\n    cmd: echo hello",
			wantErr:  false,
		},
		{
			name:     "yml fence",
			input:    "```yml\n- shell:\n    cmd: echo hello\n```",
			expected: "- shell:\n    cmd: echo hello",
			wantErr:  false,
		},
		{
			name:     "generic fence",
			input:    "```\n- shell:\n    cmd: echo hello\n```",
			expected: "- shell:\n    cmd: echo hello",
			wantErr:  false,
		},
		{
			name:     "no fence",
			input:    "- shell:\n    cmd: echo hello",
			expected: "- shell:\n    cmd: echo hello",
			wantErr:  false,
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "whitespace only",
			input:    "   \n\n  ",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "fence with whitespace",
			input:    "  ```yaml\n- shell:\n    cmd: echo hello\n```  ",
			expected: "- shell:\n    cmd: echo hello",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizePlan(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if strings.TrimSpace(string(result)) != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, string(result))
			}
		})
	}
}
