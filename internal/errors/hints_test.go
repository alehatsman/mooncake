package errors

import (
	"testing"
)

func TestInferHint(t *testing.T) {
	tests := []struct {
		name     string
		stderr   string
		wantText string
		wantStep string
	}{
		{
			name:     "no match returns empty hint",
			stderr:   "some random error",
			wantText: "",
		},
		{
			name:     "curl command not found",
			stderr:   "curl: command not found",
			wantText: "curl is not installed",
			wantStep: "pkg:\n  name: curl\n  state: present",
		},
		{
			name:     "generic command not found",
			stderr:   "command not found: jq",
			wantText: "jq is not installed",
			wantStep: "pkg:\n  name: jq\n  state: present",
		},
		{
			// Regression for manual-test #13 (2026-05-15): the bash/sh prefix
			// form of the "command not found" error was extracting the next
			// line's first token (typically "bash:") instead of the actual
			// missing command, producing a suggested_step with a trailing
			// colon and the wrong package name.
			name:     "bash prefix form with multiple lines",
			stderr:   "bash: line 1: lt: command not found\nbash: line 2: foo: command not found",
			wantText: "lt is not installed",
			wantStep: "pkg:\n  name: lt\n  state: present",
		},
		{
			name:     "permission denied",
			stderr:   "/etc/hosts: permission denied",
			wantText: "insufficient permissions; try running with sudo",
			wantStep: "",
		},
		{
			name:     "EACCES",
			stderr:   "open /var/log: EACCES",
			wantText: "insufficient permissions; try running with sudo",
		},
		{
			name:     "no such file or directory",
			stderr:   "No such file or directory: '/tmp/missing'",
			wantText: "path does not exist: /tmp/missing",
		},
		{
			name:     "address already in use",
			stderr:   "listen tcp :8080: bind: address already in use",
			wantText: "port already bound; check running processes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := InferHint(tc.stderr)
			if h.Text != tc.wantText {
				t.Errorf("Text: got %q, want %q", h.Text, tc.wantText)
			}
			if tc.wantStep != "" && h.SuggestedStep != tc.wantStep {
				t.Errorf("SuggestedStep: got %q, want %q", h.SuggestedStep, tc.wantStep)
			}
		})
	}
}
