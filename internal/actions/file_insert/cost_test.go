//nolint:revive // package name follows action convention (file_insert)
package file_insert

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_Basic(t *testing.T) {
	h := &Handler{}
	step := &config.Step{TextInsert: &config.FileInsert{Path: "/tmp/x", Anchor: "a", Position: "after", Content: "x"}}
	c, _ := h.Cost(nil, step)
	if c.Resources != 1 || c.Risk != 4 || !c.Reversible {
		t.Errorf("unexpected cost: %+v", c)
	}
}

func TestHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
