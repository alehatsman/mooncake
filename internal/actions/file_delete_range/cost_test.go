//nolint:revive // package name follows action convention (file_delete_range)
package file_delete_range

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_Basic(t *testing.T) {
	h := &Handler{}
	step := &config.Step{TextDeleteRange: &config.FileDeleteRange{Path: "/tmp/x", StartAnchor: "a", EndAnchor: "b"}}
	c, _ := h.Cost(nil, step)
	if c.Resources != 1 || !c.Reversible {
		t.Errorf("unexpected cost: %+v", c)
	}
	if c.Risk != 5 {
		t.Errorf("Risk = %d, want 5 (range deletion)", c.Risk)
	}
}

func TestHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
