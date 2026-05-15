package template

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_Basic(t *testing.T) {
	h := &Handler{}
	step := &config.Step{FileTemplate: &config.Template{Src: "x.tmpl", Dest: "/tmp/x"}}
	c, _ := h.Cost(nil, step)
	if c.Resources != 1 || c.Risk != 4 || !c.Reversible || c.Bytes != -1 {
		t.Errorf("unexpected cost: %+v", c)
	}
}

func TestHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
