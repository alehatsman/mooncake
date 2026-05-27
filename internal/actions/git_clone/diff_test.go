package git_clone

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestDiff_MissingDestIsCreate(t *testing.T) {
	h := Handler{}
	dest := filepath.Join(t.TempDir(), "does-not-exist")
	step := &config.Step{GitClone: &config.GitClone{Repo: "https://x/y", Dest: dest, Ref: "v1"}}

	d, err := h.Diff(nil, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpCreate)
	}
	if d.Resource.Kind != actions.ResourceGit {
		t.Errorf("Resource.Kind = %s, want %s", d.Resource.Kind, actions.ResourceGit)
	}
	if d.Resource.Identifier != dest {
		t.Errorf("Resource.Identifier = %s, want %s", d.Resource.Identifier, dest)
	}
	if d.Before != nil {
		t.Errorf("Before must be nil for OpCreate; got %+v", d.Before)
	}
	after, ok := d.After.(*actions.GitCloneDiff)
	if !ok {
		t.Fatalf("After is not *actions.GitCloneDiff; got %T", d.After)
	}
	if after.Repo != "https://x/y" || after.Ref != "v1" {
		t.Errorf("After = %+v, want repo=https://x/y ref=v1", after)
	}
}

func TestDiff_NonGitDirIsUpdate(t *testing.T) {
	h := Handler{}
	dir := t.TempDir() // exists, no .git
	step := &config.Step{GitClone: &config.GitClone{Repo: "x", Dest: dir, Update: true}}

	d, _ := h.Diff(nil, step)
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %s, want %s for non-git directory", d.Operation, actions.OpUpdate)
	}
}

func TestDiff_ExistingRepoUpdateFalseIsNoop(t *testing.T) {
	h := Handler{}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{GitClone: &config.GitClone{Repo: "x", Dest: dir, Update: false}}

	d, _ := h.Diff(nil, step)
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %s, want %s (existing repo, update=false)", d.Operation, actions.OpNoop)
	}
}

func TestDiff_ExistingRepoUpdateTrueIsUpdate(t *testing.T) {
	h := Handler{}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{GitClone: &config.GitClone{Repo: "x", Dest: dir, Update: true}}

	d, _ := h.Diff(nil, step)
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpUpdate)
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	_, err := h.Diff(nil, nil)
	if err == nil {
		t.Fatal("Diff must error on nil step")
	}
}

func TestGitCloneHandler_ImplementsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}
