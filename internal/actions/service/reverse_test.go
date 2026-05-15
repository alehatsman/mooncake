package service

import (
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func TestReverse_RestoresPriorActiveAndEnabled(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("os.service Reverse v6 covers Linux/systemd only")
	}
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsServiceReverseInfo{
		Name:             "nginx",
		PriorActive:      true,
		PriorEnabled:     true,
		HadStateIntent:   true,
		HadEnabledIntent: true,
	}
	enabled := false
	step := &config.Step{OsService: &config.ServiceAction{
		Name:    "nginx",
		State:   ServiceStateStopped,
		Enabled: &enabled,
	}}

	rev, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.OsService == nil {
		t.Fatal("Reverse must return an os.service step")
	}
	if rev.OsService.State != ServiceStateStarted {
		t.Errorf("State = %s, want started (prior was active)", rev.OsService.State)
	}
	if rev.OsService.Enabled == nil || *rev.OsService.Enabled != true {
		t.Errorf("Enabled = %v, want *true (prior was enabled)", rev.OsService.Enabled)
	}
}

func TestReverse_PriorInactiveBecomesStopped(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("os.service Reverse v6 covers Linux/systemd only")
	}
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsServiceReverseInfo{
		Name:           "nginx",
		PriorActive:    false,
		HadStateIntent: true,
	}
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx", State: ServiceStateStarted}}

	rev, _ := h.Reverse(nil, step, r)
	if rev == nil || rev.OsService.State != ServiceStateStopped {
		t.Errorf("State = %v, want stopped (prior was inactive)", rev)
	}
}

func TestReverse_NoIntentReturnsNoop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("os.service Reverse v6 covers Linux/systemd only")
	}
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsServiceReverseInfo{
		Name: "nginx",
		// No HadStateIntent or HadEnabledIntent — apply didn't
		// manage lifecycle.
	}
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx"}}

	rev, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse must not error on lifecycle-noop apply; got %v", err)
	}
	if rev != nil {
		t.Errorf("Reverse must return nil step when neither State nor Enabled is set on inverse; got %+v", rev)
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("os.service Reverse v6 covers Linux/systemd only")
	}
	h := &Handler{}
	r := executor.NewResult()
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx", State: ServiceStateStarted}}

	rev, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse on no-capture must not error; got: %v", err)
	}
	if rev != nil {
		t.Errorf("Reverse on no-capture must return nil step; got %+v", rev)
	}
}

func TestReverse_NonLinuxRefuses(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("test asserts non-Linux refusal; skip on Linux")
	}
	h := &Handler{}
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx"}}
	_, err := h.Reverse(nil, step, executor.NewResult())
	if err == nil || !strings.Contains(err.Error(), "only Linux") {
		t.Errorf("Reverse on non-Linux must refuse with 'only Linux' message; got: %v", err)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := &Handler{}
	_, err := h.Reverse(nil, nil, nil)
	if err == nil {
		t.Fatal("Reverse must error on nil step")
	}
}

func TestReverse_WrongReverseDataType(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("os.service Reverse v6 covers Linux/systemd only")
	}
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = "wrong"
	_, err := h.Reverse(nil, &config.Step{OsService: &config.ServiceAction{Name: "nginx"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
	if !actions.IsReverser(&Handler{}) {
		t.Error("actions.IsReverser((*Handler)) = false; want true")
	}
}
