//nolint:revive // package name follows action convention
package pkg_repo

import (
	"errors"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// stubBrew replaces the package-level brewListTaps + brewExec hooks
// for the duration of a test. Records every brewExec invocation so
// assertions can confirm "did we call brew tap/untap, and which
// args did we pass?". t.Cleanup restores the production hooks.
type stubBrew struct {
	taps    []string
	calls   [][]string
	execErr error
	listErr error
}

func installStubBrew(t *testing.T, s *stubBrew) {
	t.Helper()
	origList := brewListTaps
	origExec := brewExec
	brewListTaps = func() ([]string, error) {
		if s.listErr != nil {
			return nil, s.listErr
		}
		return append([]string{}, s.taps...), nil
	}
	brewExec = func(args ...string) error {
		s.calls = append(s.calls, append([]string{}, args...))
		if s.execErr != nil {
			return s.execErr
		}
		// Mutate the simulated tap set so a subsequent list reflects
		// the change — keeps the stub honest if a test runs two
		// applies back to back.
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
		brewListTaps = origList
		brewExec = origExec
	})
}

func brewStep(tap, state string) *config.Step {
	return &config.Step{PkgRepo: &config.PkgRepo{
		Name:  strings.ReplaceAll(tap, "/", "-"),
		State: state,
		Brew:  &config.PkgRepoBrew{Tap: tap},
	}}
}

// TestBrew_PresentAlreadyTapped — the proposal-08 hot path: re-running
// `state: present` on an already-tapped repo must be a noop (no brew
// exec, changed=false). This is the property that makes the shell
// `brew tap … || true` workaround unnecessary.
func TestBrew_PresentAlreadyTapped(t *testing.T) {
	s := &stubBrew{taps: []string{"homebrew/cask-fonts"}}
	installStubBrew(t, s)

	res, err := (&Handler{}).Run(newCtx(t, false), brewStep("homebrew/cask-fonts", "present"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Error("changed=true on already-tapped; want false (idempotent noop)")
	}
	if len(s.calls) != 0 {
		t.Errorf("brewExec calls = %v; want none on noop", s.calls)
	}
	if r.Data["operation"] != "already-tapped" {
		t.Errorf("operation = %v, want already-tapped", r.Data["operation"])
	}
}

// TestBrew_PresentNotTapped — first apply of `state: present` runs
// `brew tap <name>` and reports changed=true.
func TestBrew_PresentNotTapped(t *testing.T) {
	s := &stubBrew{taps: []string{"homebrew/services"}}
	installStubBrew(t, s)

	res, err := (&Handler{}).Run(newCtx(t, false), brewStep("homebrew/cask-fonts", "present"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Error("changed=false on first tap; want true")
	}
	if len(s.calls) != 1 || s.calls[0][0] != "tap" || s.calls[0][1] != "homebrew/cask-fonts" {
		t.Errorf("brewExec calls = %v; want one [tap homebrew/cask-fonts] invocation", s.calls)
	}
}

// TestBrew_AbsentTapped — `state: absent` on a currently-tapped repo
// runs `brew untap` and reports changed=true.
func TestBrew_AbsentTapped(t *testing.T) {
	s := &stubBrew{taps: []string{"homebrew/cask-fonts", "homebrew/services"}}
	installStubBrew(t, s)

	res, err := (&Handler{}).Run(newCtx(t, false), brewStep("homebrew/cask-fonts", "absent"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Error("changed=false when untapping; want true")
	}
	if len(s.calls) != 1 || s.calls[0][0] != "untap" {
		t.Errorf("brewExec calls = %v; want one untap invocation", s.calls)
	}
}

// TestBrew_AbsentNotTapped — `state: absent` on an already-untapped
// repo is a noop. Same idempotency property as the present case.
func TestBrew_AbsentNotTapped(t *testing.T) {
	s := &stubBrew{taps: []string{"homebrew/services"}}
	installStubBrew(t, s)

	res, err := (&Handler{}).Run(newCtx(t, false), brewStep("homebrew/cask-fonts", "absent"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Error("changed=true on already-untapped; want false (noop)")
	}
	if len(s.calls) != 0 {
		t.Errorf("brewExec calls = %v; want none on noop", s.calls)
	}
}

// TestBrew_PlanModeNoExec — plan mode reports WouldChange and the
// op but never invokes brewExec.
func TestBrew_PlanModeNoExec(t *testing.T) {
	s := &stubBrew{}
	installStubBrew(t, s)

	res, err := (&Handler{}).Run(newCtx(t, true), brewStep("homebrew/cask-fonts", "present"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("plan mode WouldChange=false; want true")
	}
	if len(s.calls) != 0 {
		t.Errorf("plan mode invoked brewExec: %v", s.calls)
	}
}

// TestBrew_TapTemplateRendered — Brew.Tap goes through the template
// renderer just like apt.uri / r.name, so {{ var }} interpolation
// matches the rest of the pkg.repo surface.
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
	if _, err := (&Handler{}).Run(ec, step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(s.calls) != 1 || s.calls[0][1] != "homebrew/cask-fonts" {
		t.Errorf("brewExec calls = %v; want template-rendered homebrew/cask-fonts", s.calls)
	}
}

// TestBrew_ListErrorPropagates — a wedged `brew tap` listing must
// surface as an error rather than silently treating "no taps known"
// as "everything is absent" (which would untap-everything on a
// large state=absent run).
func TestBrew_ListErrorPropagates(t *testing.T) {
	s := &stubBrew{listErr: errors.New("brew: not installed")}
	installStubBrew(t, s)

	_, err := (&Handler{}).Run(newCtx(t, false), brewStep("homebrew/cask-fonts", "present"))
	if err == nil {
		t.Fatal("expected list error to surface; got nil")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %q; want it to include the brew message", err)
	}
}

// TestBrew_ReverseInfoCapturedOnTap — proposal-05 capability flags
// promise pkg.repo is reversible. The brew driver populates
// ReverseData with the pre-apply tap state so a future Reverser
// can build the inverse step without re-listing.
func TestBrew_ReverseInfoCapturedOnTap(t *testing.T) {
	s := &stubBrew{taps: []string{}}
	installStubBrew(t, s)

	res, err := (&Handler{}).Run(newCtx(t, false), brewStep("homebrew/cask-fonts", "present"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
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

// TestBrew_ValidateRequiresTapOnPresent — proposal-08 design note:
// a missing tap must fail at validate time so a misconfigured step
// can't silently produce a noop after rendering.
func TestBrew_ValidateRequiresTapOnPresent(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name:    "present without tap",
			step:    &config.Step{PkgRepo: &config.PkgRepo{Name: "x", State: "present", Brew: &config.PkgRepoBrew{}}},
			wantErr: true,
		},
		{
			name:    "present with tap",
			step:    &config.Step{PkgRepo: &config.PkgRepo{Name: "x", State: "present", Brew: &config.PkgRepoBrew{Tap: "foo/bar"}}},
			wantErr: false,
		},
		{
			name:    "absent without tap",
			step:    &config.Step{PkgRepo: &config.PkgRepo{Name: "x", State: "absent", Brew: &config.PkgRepoBrew{}}},
			wantErr: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("Validate err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}
