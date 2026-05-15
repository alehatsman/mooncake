package service

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestReverse_Refuses(t *testing.T) {
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx", State: "started"}}
	h := &Handler{}
	reverseStep, err := h.Reverse(nil, step, nil)
	if err == nil {
		t.Fatal("Reverse should refuse for os.service; got nil error")
	}
	if reverseStep != nil {
		t.Errorf("Reverse returned a step despite refusal: %+v", reverseStep)
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("error %q should mention not-yet-implemented", err.Error())
	}
}

func TestHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
	if !actions.IsReverser(&Handler{}) {
		t.Error("actions.IsReverser((*Handler)) = false; want true (refusal Reverser still satisfies interface)")
	}
}
