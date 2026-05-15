package copy

import (
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

// TestMT51_FollowSymlinks_DefaultsToTrue covers manual-test #51
// (2026-05-15): file.copy used to silently dereference a symlink src,
// producing a regular dest with the link's *target* content. That's
// wrong for dotfile / /usr/local/bin/foo workflows where the link IS
// what the user wants to install.
func TestMT51_FollowSymlinks_DefaultsToTrue(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "dest")

	ec := newCopyTestEC(t)
	res, err := (&Handler{}).Run(ec, &config.Step{FileCopy: &config.Copy{Src: link, Dest: dest}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}

	// default behavior: dest is a regular file with link's target content.
	info, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("dest: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("dest should be a regular file by default, got symlink")
	}
	content, _ := os.ReadFile(dest)
	if string(content) != "payload" {
		t.Errorf("dest content = %q, want %q", content, "payload")
	}
}

func TestMT51_FollowSymlinks_FalsePreservesLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "dest")

	follow := false
	ec := newCopyTestEC(t)
	res, err := (&Handler{}).Run(ec, &config.Step{FileCopy: &config.Copy{Src: link, Dest: dest, FollowSymlinks: &follow}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	r, _ := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true for new symlink")
	}

	info, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("dest: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("dest should be a symlink when follow_symlinks: false")
	}
	got, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if got != target {
		t.Errorf("symlink target = %q, want %q", got, target)
	}
}

func TestMT51_FollowSymlinks_False_Idempotent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "dest")

	follow := false
	step := &config.Step{FileCopy: &config.Copy{Src: link, Dest: dest, FollowSymlinks: &follow}}

	// Run 1: creates the symlink.
	ec1 := newCopyTestEC(t)
	if _, err := (&Handler{}).Run(ec1, step); err != nil {
		t.Fatalf("run1: %v", err)
	}
	// Run 2: dest already a symlink with same target → no change.
	ec2 := newCopyTestEC(t)
	res, err := (&Handler{}).Run(ec2, step)
	if err != nil {
		t.Fatalf("run2: %v", err)
	}
	r, _ := res.(*executor.Result)
	if r.Changed {
		t.Errorf("expected Changed=false on second run (same target)")
	}
}

func newCopyTestEC(t *testing.T) *executor.ExecutionContext {
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
