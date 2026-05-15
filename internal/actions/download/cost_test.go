package download

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_Basic(t *testing.T) {
	h := &Handler{}
	step := &config.Step{FileDownload: &config.Download{URL: "https://example/x", Dest: "/tmp/x"}}
	c, _ := h.Cost(nil, step)
	if c.Resources != 1 || !c.Reversible {
		t.Errorf("unexpected cost: %+v", c)
	}
	if c.Risk != 5 {
		t.Errorf("Risk = %d, want 5 (network involvement)", c.Risk)
	}
}

func TestHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
