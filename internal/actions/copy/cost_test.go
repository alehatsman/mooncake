package copy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_SrcSizeReportedWhenStatable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(src, make([]byte, 1234), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	h := &Handler{}
	step := &config.Step{FileCopy: &config.Copy{Src: src, Dest: filepath.Join(dir, "dst")}}
	c, _ := h.Cost(nil, step)
	if c.Bytes != 1234 {
		t.Errorf("Bytes = %d, want 1234 (src size)", c.Bytes)
	}
	if c.Risk != 4 {
		t.Errorf("Risk = %d, want 4", c.Risk)
	}
}

func TestCost_ForceRaisesRisk(t *testing.T) {
	h := &Handler{}
	step := &config.Step{FileCopy: &config.Copy{Src: "/nope", Dest: "/x", Force: true}}
	c, _ := h.Cost(nil, step)
	if c.Risk != 5 {
		t.Errorf("Force=true Risk = %d, want 5", c.Risk)
	}
}

func TestHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
