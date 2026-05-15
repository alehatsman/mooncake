package git_config

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_RiskIs2(t *testing.T) {
	h := Handler{}
	c, err := h.Cost(nil, &config.Step{GitConfig: &config.GitConfig{Scope: "global", Set: map[string]string{"user.email": "x"}}})
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}
	if c.Risk != 2 {
		t.Errorf("Risk = %d, want 2", c.Risk)
	}
}

func TestCost_ResourcesCountsKeys(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{GitConfig: &config.GitConfig{
		Scope: "global",
		Set:   map[string]string{"a.b": "1", "c.d": "2"},
		Unset: []string{"e.f"},
	}})
	if c.Resources != 3 {
		t.Errorf("Resources = %d, want 3 (2 set + 1 unset)", c.Resources)
	}
}

func TestCost_ReversibleByInterface(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{GitConfig: &config.GitConfig{Scope: "global", Set: map[string]string{"a": "b"}}})
	if !c.Reversible {
		t.Errorf("Reversible = false; git.config opts into Reverser interface")
	}
}

func TestCost_NilStepDefaults(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, nil)
	if c.Risk != 2 {
		t.Errorf("Risk = %d, want 2 for nil step", c.Risk)
	}
}

func TestGitConfigHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
