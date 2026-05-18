package git_config //nolint:revive // package name follows action convention

import (
	"sort"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func TestReverse_RestoresPriorValues(t *testing.T) {
	h := Handler{}
	result := executor.NewResult()
	result.ReverseData = &GitConfigReverseInfo{
		Scope: "global",
		Entries: []GitConfigReverseEntry{
			{Key: "user.email", PriorValue: "old@example.com", HadValue: true},
			{Key: "core.autocrlf", PriorValue: "true", HadValue: true},
		},
	}
	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "global",
		Set:   map[string]string{"user.email": "new@example.com", "core.autocrlf": "false"},
	}}

	rev, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.GitConfig == nil {
		t.Fatal("Reverse must return a git.config step")
	}
	if rev.GitConfig.Scope != "global" {
		t.Errorf("Scope = %s, want global", rev.GitConfig.Scope)
	}
	if rev.GitConfig.Set["user.email"] != "old@example.com" {
		t.Errorf("Set[user.email] = %s, want old@example.com", rev.GitConfig.Set["user.email"])
	}
	if rev.GitConfig.Set["core.autocrlf"] != "true" {
		t.Errorf("Set[core.autocrlf] = %s, want true", rev.GitConfig.Set["core.autocrlf"])
	}
}

func TestReverse_UnsetsKeysThatDidNotExistBefore(t *testing.T) {
	h := Handler{}
	result := executor.NewResult()
	result.ReverseData = &GitConfigReverseInfo{
		Scope: "global",
		Entries: []GitConfigReverseEntry{
			{Key: "user.email", HadValue: false},                    // didn't exist before — reverse should unset
			{Key: "user.name", PriorValue: "Alice", HadValue: true}, // existed — reverse restores
		},
	}
	rev, _ := h.Reverse(nil, &config.Step{GitConfig: &config.GitConfig{Scope: "global"}}, result)
	if rev == nil {
		t.Fatal("Reverse must return a step")
	}
	if rev.GitConfig.Set["user.name"] != "Alice" {
		t.Errorf("Set[user.name] = %s, want Alice", rev.GitConfig.Set["user.name"])
	}
	if _, hasEmail := rev.GitConfig.Set["user.email"]; hasEmail {
		t.Error("user.email must NOT be in Set (it didn't exist before)")
	}
	if len(rev.GitConfig.Unset) != 1 || rev.GitConfig.Unset[0] != "user.email" {
		t.Errorf("Unset = %v, want [user.email]", rev.GitConfig.Unset)
	}
}

func TestReverse_LocalScopeCarriesRepo(t *testing.T) {
	h := Handler{}
	result := executor.NewResult()
	result.ReverseData = &GitConfigReverseInfo{
		Scope: "local",
		Repo:  "/srv/app",
		Entries: []GitConfigReverseEntry{
			{Key: "core.autocrlf", PriorValue: "true", HadValue: true},
		},
	}
	rev, _ := h.Reverse(nil, &config.Step{GitConfig: &config.GitConfig{Scope: "local", Repo: "/srv/app"}}, result)
	if rev == nil {
		t.Fatal("Reverse must return a step")
	}
	if rev.GitConfig.Repo != "/srv/app" {
		t.Errorf("Repo = %s, want /srv/app", rev.GitConfig.Repo)
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := Handler{}
	result := executor.NewResult()
	// No ReverseData → apply was a noop. Reverse returns (nil, nil).
	step, err := h.Reverse(nil, &config.Step{GitConfig: &config.GitConfig{Scope: "global"}}, result)
	if err != nil {
		t.Fatalf("Reverse on no-capture must not error; got: %v", err)
	}
	if step != nil {
		t.Errorf("Reverse on no-capture must return nil step; got %+v", step)
	}
}

func TestReverse_EmptyEntriesIsNoop(t *testing.T) {
	h := Handler{}
	result := executor.NewResult()
	result.ReverseData = &GitConfigReverseInfo{Scope: "global"}
	step, err := h.Reverse(nil, &config.Step{GitConfig: &config.GitConfig{Scope: "global"}}, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if step != nil {
		t.Errorf("Reverse on empty entries must return nil step; got %+v", step)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	_, err := h.Reverse(nil, nil, nil)
	if err == nil {
		t.Fatal("Reverse must error on nil step")
	}
}

func TestReverse_WrongReverseDataType(t *testing.T) {
	h := Handler{}
	result := executor.NewResult()
	result.ReverseData = "not the right type"
	_, err := h.Reverse(nil, &config.Step{GitConfig: &config.GitConfig{Scope: "global"}}, result)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestBuildReverseInfo_PreservesScopeAndEntries(t *testing.T) {
	drift := []driftEntry{
		{key: "user.email", op: "set", current: "old@x", desired: "new@x", hadValue: true},
		{key: "user.name", op: "set", current: "", desired: "Alice", hadValue: false},
	}
	info := buildReverseInfo("global", "", drift)
	if info.Scope != "global" {
		t.Errorf("Scope = %s, want global", info.Scope)
	}
	if len(info.Entries) != 2 {
		t.Fatalf("Entries len = %d, want 2", len(info.Entries))
	}
	// Sort for stable comparison.
	keys := []string{info.Entries[0].Key, info.Entries[1].Key}
	sort.Strings(keys)
	if keys[0] != "user.email" || keys[1] != "user.name" {
		t.Errorf("keys = %v, want [user.email, user.name]", keys)
	}
}

func TestGitConfigHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
