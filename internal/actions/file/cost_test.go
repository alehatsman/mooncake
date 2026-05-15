package file

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_StateFileWithContent(t *testing.T) {
	h := &Handler{}
	step := &config.Step{FileWrite: &config.File{Path: "/tmp/x", Content: "hello"}}
	c, err := h.Cost(nil, step)
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}
	if c.Resources != 1 {
		t.Errorf("Resources = %d, want 1", c.Resources)
	}
	if c.Bytes != 5 {
		t.Errorf("Bytes = %d, want 5 (len of \"hello\")", c.Bytes)
	}
	if !c.Reversible {
		t.Error("Reversible should be true")
	}
	if c.Risk != 4 {
		t.Errorf("Risk = %d, want 4 (routine config write)", c.Risk)
	}
}

func TestCost_StateAbsentIsHighRisk(t *testing.T) {
	h := &Handler{}
	step := &config.Step{FileWrite: &config.File{Path: "/tmp/x", State: "absent"}}
	c, _ := h.Cost(nil, step)
	if c.Risk != 8 {
		t.Errorf("state=absent Risk = %d, want 8 (delete)", c.Risk)
	}
	if c.Bytes != -1 {
		t.Errorf("state=absent Bytes = %d, want -1", c.Bytes)
	}
}

func TestCost_NilStep(t *testing.T) {
	h := &Handler{}
	c, err := h.Cost(nil, nil)
	if err != nil {
		t.Fatalf("nil step should not error: %v", err)
	}
	if c.Risk != 4 || c.Resources != 1 {
		t.Errorf("nil step defaults wrong: %+v", c)
	}
}

func TestHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
	if !actions.IsCoster(&Handler{}) {
		t.Error("actions.IsCoster((*Handler)) = false; want true")
	}
}
