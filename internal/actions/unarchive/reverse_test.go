package unarchive

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestReverse_Refuses(t *testing.T) {
	step := &config.Step{FileUnarchive: &config.Unarchive{Src: "x.tar", Dest: "/tmp/extract"}}
	h := &Handler{}
	reverseStep, err := h.Reverse(nil, step, nil)
	if err == nil {
		t.Fatal("Reverse should refuse for file.unarchive; got nil error")
	}
	if reverseStep != nil {
		t.Errorf("Reverse returned a step despite refusal: %+v", reverseStep)
	}
	if !strings.Contains(err.Error(), "multi-step") {
		t.Errorf("error %q should mention multi-step blocker", err.Error())
	}
}

func TestHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
	if !actions.IsReverser(&Handler{}) {
		t.Error("actions.IsReverser((*Handler)) = false; want true (refusal Reverser still satisfies interface)")
	}
}
