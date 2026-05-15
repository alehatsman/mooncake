package git_config

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestDiff_GlobalScopeUpdate(t *testing.T) {
	h := Handler{}
	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "global",
		Set:   map[string]string{"user.email": "dev@example.com"},
	}}

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
	if d.Resource.Identifier != "global" {
		t.Errorf("Resource.Identifier = %s, want global", d.Resource.Identifier)
	}
}

func TestDiff_LocalScopeIdentifierCarriesRepo(t *testing.T) {
	h := Handler{}
	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  "/tmp/myrepo",
		Set:   map[string]string{"core.autocrlf": "false"},
	}}

	d, _ := h.Diff(nil, step)
	if d.Resource.Identifier != "local:/tmp/myrepo" {
		t.Errorf("Resource.Identifier = %s, want local:/tmp/myrepo", d.Resource.Identifier)
	}
}

func TestDiff_AfterCarriesSortedEntries(t *testing.T) {
	h := Handler{}
	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "global",
		Set:   map[string]string{"z.last": "1", "a.first": "2", "m.middle": "3"},
		Unset: []string{"old.key"},
	}}

	d, _ := h.Diff(nil, step)
	after, ok := d.After.(*GitConfigSnapshot)
	if !ok {
		t.Fatalf("After is not *GitConfigSnapshot; got %T", d.After)
	}
	if len(after.Entries) != 4 {
		t.Fatalf("Entries len = %d, want 4", len(after.Entries))
	}
	wantKeys := []string{"a.first", "m.middle", "z.last", "old.key"}
	for i, k := range wantKeys {
		if after.Entries[i].Key != k {
			t.Errorf("Entries[%d].Key = %s, want %s", i, after.Entries[i].Key, k)
		}
	}
	if after.Entries[3].Op != "unset" {
		t.Errorf("Entries[3].Op = %s, want unset", after.Entries[3].Op)
	}
}

func TestDiff_EmptyEntriesIsNoop(t *testing.T) {
	h := Handler{}
	step := &config.Step{GitConfig: &config.GitConfig{Scope: "global"}}

	d, _ := h.Diff(nil, step)
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %s, want %s for vacuous step", d.Operation, actions.OpNoop)
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	_, err := h.Diff(nil, nil)
	if err == nil {
		t.Fatal("Diff must error on nil step")
	}
}

func TestGitConfigHandler_ImplementsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}
