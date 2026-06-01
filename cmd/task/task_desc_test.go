package task

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestTaskListDescription covers the #53 listing-description fallback chain.
func TestTaskListDescription(t *testing.T) {
	dir := t.TempDir()

	// A local component with its own description, referenced by a shorthand task.
	comp := filepath.Join(dir, "lint.yml")
	if err := os.WriteFile(comp, []byte(`name: lint
description: Run the linters
steps:
  - shell: { cmd: "echo lint" }
`), 0o644); err != nil {
		t.Fatalf("write component: %v", err)
	}

	tests := []struct {
		name string
		task config.Task
		want string
	}{
		{
			name: "explicit desc wins",
			task: config.Task{Desc: "explicit", Steps: []config.Step{{Use: "./lint.yml"}}},
			want: "explicit",
		},
		{
			name: "local use-ref falls back to component description",
			task: config.Task{Steps: []config.Step{{Use: "./lint.yml"}}},
			want: "Run the linters",
		},
		{
			name: "alias use-ref shows a ref hint (no network)",
			task: config.Task{Steps: []config.Step{{Use: "tq/lint"}}},
			want: "→ tq/lint",
		},
		{
			name: "multi-step task with no desc",
			task: config.Task{Steps: []config.Step{{Use: "a"}, {Use: "b"}}},
			want: "(no description)",
		},
		{
			name: "non-use task with no desc",
			task: config.Task{Steps: []config.Step{{Shell: &config.ShellAction{Cmd: "echo hi"}}}},
			want: "(no description)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := taskListDescription(tc.task, dir); got != tc.want {
				t.Errorf("taskListDescription = %q, want %q", got, tc.want)
			}
		})
	}
}
