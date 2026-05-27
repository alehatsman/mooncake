package git_checkout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestDiff_AlwaysUpdate(t *testing.T) {
	h := Handler{}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: dir, Ref: "v1.2.3"}}

	d, err := h.Diff(nil, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpUpdate)
	}
	if d.Resource.Kind != actions.ResourceGit {
		t.Errorf("Resource.Kind = %s, want %s", d.Resource.Kind, actions.ResourceGit)
	}
	if d.Resource.Identifier != dir {
		t.Errorf("Resource.Identifier = %s, want %s", d.Resource.Identifier, dir)
	}
}

func TestDiff_AfterCarriesRef(t *testing.T) {
	h := Handler{}
	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: "/tmp/missing", Ref: "v1.2.3"}}

	d, _ := h.Diff(nil, step)
	after, ok := d.After.(*actions.GitCheckoutDiff)
	if !ok {
		t.Fatalf("After is not *actions.GitCheckoutDiff; got %T", d.After)
	}
	if after.Ref != "v1.2.3" {
		t.Errorf("After.Ref = %s, want v1.2.3", after.Ref)
	}
}

func TestDiff_NoBeforeForMissingDest(t *testing.T) {
	h := Handler{}
	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: "/tmp/definitely-missing-xyz", Ref: "v1"}}

	d, _ := h.Diff(nil, step)
	if d.Before != nil {
		t.Errorf("Before must be nil when dest is missing; got %+v", d.Before)
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	_, err := h.Diff(nil, nil)
	if err == nil {
		t.Fatal("Diff must error on nil step")
	}
}

func TestGitCheckoutHandler_ImplementsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}
