package git_clone

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_RiskIs4(t *testing.T) {
	h := Handler{}
	c, err := h.Cost(nil, &config.Step{GitClone: &config.GitClone{Repo: "x", Dest: "/tmp/x"}})
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}
	if c.Risk != 4 {
		t.Errorf("Risk = %d, want 4", c.Risk)
	}
}

func TestCost_OneResourceBytesUnknown(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{GitClone: &config.GitClone{Repo: "x", Dest: "/tmp/x"}})
	if c.Resources != 1 {
		t.Errorf("Resources = %d, want 1", c.Resources)
	}
	if c.Bytes != -1 {
		t.Errorf("Bytes = %d, want -1 (unknown)", c.Bytes)
	}
}

func TestCost_NotReversibleByDesign(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{GitClone: &config.GitClone{Repo: "x", Dest: "/tmp/x"}})
	if c.Reversible {
		t.Errorf("Reversible = true; git.clone is irreversible by design (spec-26)")
	}
}

func TestCost_NilStepReturnsDefaults(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, nil)
	if c.Risk != 4 {
		t.Errorf("Risk = %d, want 4 even for nil step", c.Risk)
	}
}

func TestGitCloneHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
