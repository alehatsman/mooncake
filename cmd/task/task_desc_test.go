package task

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/modules"
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
			name: "alias use-ref with no resolver shows a ref hint",
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
			if got := taskListDescription(tc.task, dir, nil); got != tc.want {
				t.Errorf("taskListDescription = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTaskListDescription_CachedAlias verifies #56: a shorthand task whose
// `use:` names a module alias shows the cached component's description: —
// resolved from the local cache only, never cloned — and falls back to the
// "→ ref" hint when the module isn't cached.
func TestTaskListDescription_CachedAlias(t *testing.T) {
	cacheRoot := t.TempDir()
	const source = "127.0.0.1:8080/owner/ts-quality@v0.1.0"

	// Lay down a fake cached module: <root>/<host>/<owner>/<repo>@<ver>/.
	modDir := filepath.Join(cacheRoot, "127.0.0.1:8080", "owner", "ts-quality@v0.1.0")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir module cache: %v", err)
	}
	writeFile(t, filepath.Join(modDir, "index.yml"), "name: ts-quality\nexports:\n  lint: lint.yml\n")
	writeFile(t, filepath.Join(modDir, "lint.yml"), "name: ts-quality-lint\ndescription: Run eslint over the web project\nsteps:\n  - shell: { cmd: \"echo lint\" }\n")

	resolver := &modules.Resolver{
		Fetcher: &modules.Fetcher{Root: cacheRoot},
		Modules: map[string]string{"tq": source},
	}
	task := config.Task{Steps: []config.Step{{Use: "tq/lint"}}}

	if got := taskListDescription(task, "", resolver); got != "Run eslint over the web project" {
		t.Errorf("cached alias desc = %q, want the component description", got)
	}

	// Uncached alias (unknown export's module dir absent) falls back to a hint
	// without cloning.
	missing := config.Task{Steps: []config.Step{{Use: "tq/typecheck"}}}
	resolver2 := &modules.Resolver{
		Fetcher: &modules.Fetcher{Root: t.TempDir()}, // empty cache root
		Modules: map[string]string{"tq": source},
	}
	if got := taskListDescription(missing, "", resolver2); got != "→ tq/typecheck" {
		t.Errorf("uncached alias = %q, want the → hint", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
