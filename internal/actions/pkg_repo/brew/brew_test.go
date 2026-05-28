package brew

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(plan),
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}

// stubBrew replaces the package-level ListTaps + Exec hooks for the
// duration of a test. Records every Exec invocation so assertions
// can confirm "did we call brew tap/untap, and which args did we
// pass?". t.Cleanup restores the production hooks.
type stubBrew struct {
	taps    []string
	calls   [][]string
	execErr error
	listErr error
}

func installStubBrew(t *testing.T, s *stubBrew) {
	t.Helper()
	origList := ListTaps
	origExec := Exec
	ListTaps = func(_ context.Context) ([]string, error) {
		if s.listErr != nil {
			return nil, s.listErr
		}
		return append([]string{}, s.taps...), nil
	}
	Exec = func(_ context.Context, args ...string) error {
		s.calls = append(s.calls, append([]string{}, args...))
		if s.execErr != nil {
			return s.execErr
		}
		switch args[0] {
		case "tap":
			s.taps = append(s.taps, args[1])
		case "untap":
			next := s.taps[:0]
			for _, t := range s.taps {
				if t != args[1] {
					next = append(next, t)
				}
			}
			s.taps = next
		}
		return nil
	}
	t.Cleanup(func() {
		ListTaps = origList
		Exec = origExec
	})
}

func brewStep(tap, state string) *config.Step {
	return &config.Step{PkgRepo: &config.PkgRepo{
		Name:  strings.ReplaceAll(tap, "/", "-"),
		State: state,
		Brew:  &config.PkgRepoBrew{Tap: tap},
	}}
}

func mustRun(t *testing.T, plan bool, step *config.Step) *executor.Result {
	t.Helper()
	result := executor.NewResult()
	result.Checkable = true
	res, err := Run(newCtx(t, plan), step.PkgRepo, result)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res.(*executor.Result)
}

// TestBrew_PresentAlreadyTapped — proposal-08 hot path: re-running
// `state: present` on an already-tapped repo must be a noop.
func TestBrew_PresentAlreadyTapped(t *testing.T) {
	s := &stubBrew{taps: []string{"homebrew/cask-fonts"}}
	installStubBrew(t, s)

	r := mustRun(t, false, brewStep("homebrew/cask-fonts", "present"))
	if r.Changed {
		t.Error("changed=true on already-tapped; want false (idempotent noop)")
	}
	if len(s.calls) != 0 {
		t.Errorf("Exec calls = %v; want none on noop", s.calls)
	}
	// brew-specific phase moved from Data["operation"] to Data["phase"]
	// when the envelope claimed `operation` for the OpX taxonomy
	// (create/update/delete/noop). The fine-grained "already-tapped"
	// verb stays here for the brew-aware consumer.
	if r.Data["phase"] != "already-tapped" {
		t.Errorf("phase = %v, want already-tapped", r.Data["phase"])
	}
	if r.Operation != executor.OpNoop {
		t.Errorf("envelope Operation = %v, want noop", r.Operation)
	}
}

func TestBrew_PresentNotTapped(t *testing.T) {
	s := &stubBrew{taps: []string{"homebrew/services"}}
	installStubBrew(t, s)

	r := mustRun(t, false, brewStep("homebrew/cask-fonts", "present"))
	if !r.Changed {
		t.Error("changed=false on first tap; want true")
	}
	if len(s.calls) != 1 || s.calls[0][0] != "tap" || s.calls[0][1] != "homebrew/cask-fonts" {
		t.Errorf("Exec calls = %v; want one [tap homebrew/cask-fonts] invocation", s.calls)
	}
}

func TestBrew_AbsentTapped(t *testing.T) {
	s := &stubBrew{taps: []string{"homebrew/cask-fonts", "homebrew/services"}}
	installStubBrew(t, s)

	r := mustRun(t, false, brewStep("homebrew/cask-fonts", "absent"))
	if !r.Changed {
		t.Error("changed=false when untapping; want true")
	}
	if len(s.calls) != 1 || s.calls[0][0] != "untap" {
		t.Errorf("Exec calls = %v; want one untap invocation", s.calls)
	}
}

func TestBrew_AbsentNotTapped(t *testing.T) {
	s := &stubBrew{taps: []string{"homebrew/services"}}
	installStubBrew(t, s)

	r := mustRun(t, false, brewStep("homebrew/cask-fonts", "absent"))
	if r.Changed {
		t.Error("changed=true on already-untapped; want false (noop)")
	}
	if len(s.calls) != 0 {
		t.Errorf("Exec calls = %v; want none on noop", s.calls)
	}
}

func TestBrew_PlanModeNoExec(t *testing.T) {
	s := &stubBrew{}
	installStubBrew(t, s)

	r := mustRun(t, true, brewStep("homebrew/cask-fonts", "present"))
	if !r.WouldChange {
		t.Error("plan mode WouldChange=false; want true")
	}
	if len(s.calls) != 0 {
		t.Errorf("plan mode invoked Exec: %v", s.calls)
	}
}

func TestBrew_TapTemplateRendered(t *testing.T) {
	s := &stubBrew{}
	installStubBrew(t, s)

	ec := newCtx(t, false)
	ec.Scope.User["org"] = "homebrew"
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name:  "cask-fonts",
		State: "present",
		Brew:  &config.PkgRepoBrew{Tap: "{{ org }}/cask-fonts"},
	}}
	result := executor.NewResult()
	result.Checkable = true
	if _, err := Run(ec, step.PkgRepo, result); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(s.calls) != 1 || s.calls[0][1] != "homebrew/cask-fonts" {
		t.Errorf("Exec calls = %v; want template-rendered homebrew/cask-fonts", s.calls)
	}
}

func TestBrew_ListErrorPropagates(t *testing.T) {
	s := &stubBrew{listErr: errors.New("brew: not installed")}
	installStubBrew(t, s)

	result := executor.NewResult()
	result.Checkable = true
	_, err := Run(newCtx(t, false), brewStep("homebrew/cask-fonts", "present").PkgRepo, result)
	if err == nil {
		t.Fatal("expected list error to surface; got nil")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %q; want it to include the brew message", err)
	}
}

// TestBrew_ReverseInfoCapturedOnTap — pkg.repo is reversible
// (proposal-05). The brew driver populates ReverseData with the
// pre-apply tap state so a future Reverser can build the inverse
// step without re-listing.
func TestBrew_ReverseInfoCapturedOnTap(t *testing.T) {
	s := &stubBrew{taps: []string{}}
	installStubBrew(t, s)

	r := mustRun(t, false, brewStep("homebrew/cask-fonts", "present"))
	info, ok := r.ReverseData.(*PkgRepoBrewReverseInfo)
	if !ok {
		t.Fatalf("ReverseData type = %T, want *PkgRepoBrewReverseInfo", r.ReverseData)
	}
	if info.Tap != "homebrew/cask-fonts" {
		t.Errorf("captured tap = %q, want homebrew/cask-fonts", info.Tap)
	}
	if info.WasTapped {
		t.Error("captured WasTapped = true; want false (pre-apply state)")
	}
}
