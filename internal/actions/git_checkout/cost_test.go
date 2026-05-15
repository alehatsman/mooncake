package git_checkout

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_RiskIs3(t *testing.T) {
	h := Handler{}
	c, err := h.Cost(nil, &config.Step{GitCheckout: &config.GitCheckout{Dest: "/tmp/x", Ref: "v1"}})
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}
	if c.Risk != 3 {
		t.Errorf("Risk = %d, want 3", c.Risk)
	}
}

func TestCost_ReversibleByInterface(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{GitCheckout: &config.GitCheckout{Dest: "/tmp/x", Ref: "v1"}})
	if !c.Reversible {
		t.Errorf("Reversible = false; git.checkout opts into Reverser interface (refusal pending refactor)")
	}
}

func TestCost_OneResource(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{GitCheckout: &config.GitCheckout{Dest: "/tmp/x", Ref: "v1"}})
	if c.Resources != 1 {
		t.Errorf("Resources = %d, want 1", c.Resources)
	}
}

func TestGitCheckoutHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
