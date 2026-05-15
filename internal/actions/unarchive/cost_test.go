package unarchive

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_Basic(t *testing.T) {
	h := &Handler{}
	step := &config.Step{FileUnarchive: &config.Unarchive{Src: "x.tar", Dest: "/tmp/out"}}
	c, _ := h.Cost(nil, step)
	if c.Resources != -1 || c.Bytes != -1 {
		t.Errorf("expected -1 for Resources+Bytes (unknown until extraction): %+v", c)
	}
	if c.Risk != 6 {
		t.Errorf("Risk = %d, want 6", c.Risk)
	}
}

func TestHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
