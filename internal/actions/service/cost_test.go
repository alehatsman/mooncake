package service

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestCost_StartedIsRoutine(t *testing.T) {
	h := &Handler{}
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx", State: ServiceStateStarted}}
	c, _ := h.Cost(nil, step)
	if c.Risk != 5 {
		t.Errorf("Risk = %d, want 5", c.Risk)
	}
}

func TestCost_StoppedIsRisk7(t *testing.T) {
	h := &Handler{}
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx", State: ServiceStateStopped}}
	c, _ := h.Cost(nil, step)
	if c.Risk != 7 {
		t.Errorf("Risk = %d, want 7 (downtime)", c.Risk)
	}
}

func TestCost_RestartedIsRisk7(t *testing.T) {
	h := &Handler{}
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx", State: ServiceStateRestarted}}
	c, _ := h.Cost(nil, step)
	if c.Risk != 7 {
		t.Errorf("Risk = %d, want 7", c.Risk)
	}
}

func TestServiceHandler_ImplementsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}
