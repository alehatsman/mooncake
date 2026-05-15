//nolint:revive // package name follows action convention
package text_patch_ini

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_Basic(t *testing.T) {
	h := &Handler{}
	step := &config.Step{TextPatchINI: &config.TextPatchINI{Path: "/tmp/x"}}
	c, _ := h.Cost(nil, step)
	if c.Resources != 1 || c.Risk != 4 || !c.Reversible {
		t.Errorf("unexpected cost: %+v", c)
	}
}

func TestHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
