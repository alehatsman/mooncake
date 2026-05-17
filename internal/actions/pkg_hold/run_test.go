//nolint:revive // package name follows action convention
package pkg_hold

import (
	"runtime"
	"sort"
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

// stub captures the calls made through the apt-mark hooks.
type stub struct {
	held        map[string]bool
	holdCalls   [][]string
	unholdCalls [][]string
	managerOK   bool
}

func newStub(t *testing.T, held map[string]bool) *stub {
	t.Helper()
	s := &stub{held: held, managerOK: true}
	origShow := aptMarkShowHold
	origHold := aptMarkHold
	origUnhold := aptMarkUnhold
	origLookPath := lookPath
	aptMarkShowHold = func() (map[string]bool, error) {
		out := make(map[string]bool, len(s.held))
		for k, v := range s.held {
			out[k] = v
		}
		return out, nil
	}
	aptMarkHold = func(pkgs []string) error {
		cp := append([]string(nil), pkgs...)
		s.holdCalls = append(s.holdCalls, cp)
		for _, p := range cp {
			s.held[p] = true
		}
		return nil
	}
	aptMarkUnhold = func(pkgs []string) error {
		cp := append([]string(nil), pkgs...)
		s.unholdCalls = append(s.unholdCalls, cp)
		for _, p := range cp {
			delete(s.held, p)
		}
		return nil
	}
	lookPath = func(name string) (string, error) {
		if !s.managerOK {
			return "", errNotFound{}
		}
		return "/usr/bin/" + name, nil
	}
	t.Cleanup(func() {
		aptMarkShowHold = origShow
		aptMarkHold = origHold
		aptMarkUnhold = origUnhold
		lookPath = origLookPath
	})
	return s
}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

func mustRun(t *testing.T, plan bool, step *config.Step) *executor.Result {
	t.Helper()
	res, err := (&Handler{}).Run(newCtx(t, plan), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res.(*executor.Result)
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil", &config.Step{}, true},
		{"no name nor names", &config.Step{PkgHold: &config.PkgHold{}}, true},
		{"both name and names", &config.Step{PkgHold: &config.PkgHold{Name: "a", Names: []string{"b"}}}, true},
		{"bad state", &config.Step{PkgHold: &config.PkgHold{Name: "a", State: "frozen"}}, true},
		{"empty name in names", &config.Step{PkgHold: &config.PkgHold{Names: []string{"a", ""}}}, true},
		{"ok single", &config.Step{PkgHold: &config.PkgHold{Name: "postgresql-15"}}, false},
		{"ok multi", &config.Step{PkgHold: &config.PkgHold{Names: []string{"nginx", "curl"}}}, false},
		{"ok unheld", &config.Step{PkgHold: &config.PkgHold{Name: "a", State: "unheld"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestApply_HoldAlreadyHeld_Noop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t, map[string]bool{"postgresql-15": true})
	step := &config.Step{PkgHold: &config.PkgHold{Name: "postgresql-15"}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("expected no-op when already held; reason=%q", r.Reason)
	}
	if len(s.holdCalls) != 0 || len(s.unholdCalls) != 0 {
		t.Errorf("expected no apt-mark calls; hold=%v unhold=%v", s.holdCalls, s.unholdCalls)
	}
}

func TestApply_HoldWhenUnheld_Holds(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t, map[string]bool{})
	step := &config.Step{PkgHold: &config.PkgHold{Name: "postgresql-15"}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	if len(s.holdCalls) != 1 || len(s.holdCalls[0]) != 1 || s.holdCalls[0][0] != "postgresql-15" {
		t.Errorf("expected single hold call for postgresql-15; got %v", s.holdCalls)
	}
	if len(s.unholdCalls) != 0 {
		t.Errorf("did not expect unhold calls; got %v", s.unholdCalls)
	}
}

func TestApply_UnholdWhenHeld_Unholds(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t, map[string]bool{"nginx": true})
	step := &config.Step{PkgHold: &config.PkgHold{Name: "nginx", State: "unheld"}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	if len(s.unholdCalls) != 1 || s.unholdCalls[0][0] != "nginx" {
		t.Errorf("expected single unhold call for nginx; got %v", s.unholdCalls)
	}
}

func TestApply_MultiPackage_OnlyDriftActedOn(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	// curl already held; nginx is not. Only nginx should be added.
	s := newStub(t, map[string]bool{"curl": true})
	step := &config.Step{PkgHold: &config.PkgHold{Names: []string{"nginx", "curl"}}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	if len(s.holdCalls) != 1 {
		t.Fatalf("expected one hold call batch; got %d (%v)", len(s.holdCalls), s.holdCalls)
	}
	got := append([]string(nil), s.holdCalls[0]...)
	sort.Strings(got)
	if len(got) != 1 || got[0] != "nginx" {
		t.Errorf("expected hold [nginx]; got %v", got)
	}
}

func TestPlan_ReportsTargetsButDoesNotCall(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t, map[string]bool{})
	step := &config.Step{PkgHold: &config.PkgHold{Names: []string{"nginx", "curl"}}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Fatalf("plan should report WouldChange; reason=%q", r.Reason)
	}
	if r.Changed {
		t.Errorf("plan must not flip Changed")
	}
	if len(s.holdCalls) != 0 || len(s.unholdCalls) != 0 {
		t.Errorf("plan must not invoke apt-mark; hold=%v unhold=%v", s.holdCalls, s.unholdCalls)
	}
}

func TestRun_UnsupportedManagerErrors(t *testing.T) {
	_ = newStub(t, map[string]bool{})
	step := &config.Step{PkgHold: &config.PkgHold{Name: "x", Manager: "dnf"}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for unsupported manager")
	}
}

// brewStub captures the calls made through the brew hooks. Parallel
// to stub for the apt path; tests use whichever stub matches the
// manager under test.
type brewStub struct {
	pinned     map[string]bool
	pinCalls   [][]string
	unpinCalls [][]string
}

func newBrewStub(t *testing.T, pinned map[string]bool) *brewStub {
	t.Helper()
	s := &brewStub{pinned: pinned}
	origListPinned := brewListPinned
	origPin := brewPin
	origUnpin := brewUnpin
	origLookPath := lookPath
	brewListPinned = func() (map[string]bool, error) {
		out := make(map[string]bool, len(s.pinned))
		for k, v := range s.pinned {
			out[k] = v
		}
		return out, nil
	}
	brewPin = func(pkgs []string) error {
		cp := append([]string(nil), pkgs...)
		s.pinCalls = append(s.pinCalls, cp)
		for _, p := range cp {
			s.pinned[p] = true
		}
		return nil
	}
	brewUnpin = func(pkgs []string) error {
		cp := append([]string(nil), pkgs...)
		s.unpinCalls = append(s.unpinCalls, cp)
		for _, p := range cp {
			delete(s.pinned, p)
		}
		return nil
	}
	// Brew-only on PATH: apt-mark missing forces auto-detect to
	// pick brew when manager: is unset.
	lookPath = func(name string) (string, error) {
		if name == "brew" {
			return "/opt/homebrew/bin/brew", nil
		}
		return "", errNotFound{}
	}
	t.Cleanup(func() {
		brewListPinned = origListPinned
		brewPin = origPin
		brewUnpin = origUnpin
		lookPath = origLookPath
	})
	return s
}

func TestApply_Brew_PinAlreadyPinned_Noop(t *testing.T) {
	s := newBrewStub(t, map[string]bool{"git": true})
	step := &config.Step{PkgHold: &config.PkgHold{Name: "git"}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("expected no-op when already pinned; reason=%q", r.Reason)
	}
	if len(s.pinCalls) != 0 || len(s.unpinCalls) != 0 {
		t.Errorf("expected no brew calls; pin=%v unpin=%v", s.pinCalls, s.unpinCalls)
	}
	if r.Data["manager"] != "brew" {
		t.Errorf("manager fact = %v, want brew", r.Data["manager"])
	}
}

func TestApply_Brew_PinWhenUnpinned_Pins(t *testing.T) {
	s := newBrewStub(t, map[string]bool{})
	step := &config.Step{PkgHold: &config.PkgHold{Name: "git"}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	if len(s.pinCalls) != 1 || len(s.pinCalls[0]) != 1 || s.pinCalls[0][0] != "git" {
		t.Errorf("expected single pin call for git; got %v", s.pinCalls)
	}
}

func TestApply_Brew_UnpinWhenPinned_Unpins(t *testing.T) {
	s := newBrewStub(t, map[string]bool{"node": true})
	step := &config.Step{PkgHold: &config.PkgHold{Name: "node", State: "unheld"}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	if len(s.unpinCalls) != 1 || s.unpinCalls[0][0] != "node" {
		t.Errorf("expected single unpin call for node; got %v", s.unpinCalls)
	}
}

func TestApply_Brew_MultiPackage_OnlyDriftActedOn(t *testing.T) {
	s := newBrewStub(t, map[string]bool{"git": true})
	step := &config.Step{PkgHold: &config.PkgHold{Names: []string{"git", "jq"}}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	if len(s.pinCalls) != 1 || len(s.pinCalls[0]) != 1 || s.pinCalls[0][0] != "jq" {
		t.Errorf("expected single pin call for jq; got %v", s.pinCalls)
	}
}

// TestApply_ExplicitManagerBrew_OnLinuxBox — explicit manager: brew
// routes through the brew driver even when apt-mark is also on PATH.
// Pins the operator-override semantic across both managers.
func TestApply_ExplicitManagerBrew_OnLinuxBox(t *testing.T) {
	// Both binaries on PATH; brew set as explicit manager.
	bs := newBrewStub(t, map[string]bool{})
	// Override lookPath to ALSO find apt-mark so we know we picked
	// brew on intent, not on detection.
	origLook := lookPath
	lookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() { lookPath = origLook })

	step := &config.Step{PkgHold: &config.PkgHold{Name: "git", Manager: "brew"}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "brew" {
		t.Errorf("manager = %v, want brew", r.Data["manager"])
	}
	if len(bs.pinCalls) != 1 {
		t.Errorf("expected pin call via brew; got %v", bs.pinCalls)
	}
}

// TestAutoDetect_PrefersAptOverBrew — same multi-manager precedent
// as pkg.list: apt-mark wins auto-detection when both are on PATH.
func TestAutoDetect_PrefersAptOverBrew(t *testing.T) {
	s := newStub(t, map[string]bool{}) // sets lookPath to find everything
	// Sentinel: any brew call fails the test.
	origBrewPin := brewPin
	brewPin = func(pkgs []string) error {
		t.Errorf("brew should not be invoked when apt-mark is on PATH; got pin %v", pkgs)
		return nil
	}
	t.Cleanup(func() { brewPin = origBrewPin })

	step := &config.Step{PkgHold: &config.PkgHold{Name: "git"}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "apt" {
		t.Errorf("manager = %v, want apt (auto-detect must prefer apt)", r.Data["manager"])
	}
	if len(s.holdCalls) != 1 {
		t.Errorf("expected apt-mark hold call; got %v", s.holdCalls)
	}
}

// TestPermissions_BinaryByGOOS pins the host-shaped RequiredBinaries.
func TestPermissions_BinaryByGOOS(t *testing.T) {
	ps := (Handler{}).Permissions(nil)
	wantBin := "apt-mark"
	if runtime.GOOS == "darwin" {
		wantBin = "brew"
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != wantBin {
		t.Errorf("RequiredBinaries = %v, want [%s]", ps.RequiredBinaries, wantBin)
	}
}

// TestMetadata_AdvertisesLinuxAndDarwin guards SupportedPlatforms.
func TestMetadata_AdvertisesLinuxAndDarwin(t *testing.T) {
	m := (&Handler{}).Metadata()
	got := map[string]bool{}
	for _, p := range m.SupportedPlatforms {
		got[p] = true
	}
	for _, want := range []string{"linux", "darwin"} {
		if !got[want] {
			t.Errorf("SupportedPlatforms missing %s: %v", want, m.SupportedPlatforms)
		}
	}
}
