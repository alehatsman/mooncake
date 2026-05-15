package repo_tree

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

// TestMT23_IncludeFilesDefaultsToTrue is a regression test for
// manual-test #23 (2026-05-15): the old IncludeFiles bool field had
// no way to distinguish "unset" from "explicit false" — both went
// through the zero-value path which silently set includeFiles=false.
// Result: every no-args `repo.tree: { path: ... }` reported zero
// files. After the *bool migration, the default is true and explicit
// false still opts out.
func TestMT23_IncludeFilesDefaultsToTrue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		include    *bool
		wantFiles  int
		wantOutput string
	}{
		{"unset → default true (counts files)", nil, 2, ""},
		{"explicit true → counts files", boolPtr(true), 2, ""},
		{"explicit false → directories only", boolPtr(false), 0, ""},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			step := &config.Step{
				RepoTree: &config.RepoTree{
					Path:         dir,
					IncludeFiles: c.include,
				},
			}
			ec := newTestExecutionContext(t)
			h := &Handler{}
			if err := h.Validate(step); err != nil {
				t.Fatalf("validate: %v", err)
			}
			res, err := h.Execute(ec, step)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if res == nil {
				t.Fatal("nil result")
			}
			r, ok := res.(*executor.Result)
			if !ok {
				t.Fatalf("result is not *executor.Result: %T", res)
			}
			totalFiles, _ := r.Data["total_files"].(int)
			if totalFiles != c.wantFiles {
				t.Errorf("total_files = %d, want %d (data=%v)", totalFiles, c.wantFiles, r.Data)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

func newTestExecutionContext(t *testing.T) *executor.ExecutionContext {
	t.Helper()
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Logger:         logger.NewConsoleLogger(logger.ErrorLevel),
			Template:       renderer,
			Evaluator:      expression.NewGovaluateEvaluator(),
			PathUtil:       pathutil.NewPathExpander(renderer),
			FileTree:       filetree.NewWalker(pathutil.NewPathExpander(renderer)),
			EventPublisher: events.NewSyncPublisher(),
			Mode:           actions.ModeApply,
			Stats:          executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: ".",
	}
}

// Avoid "imported and not used" if test imports drift in the future.
var _ = context.Background
