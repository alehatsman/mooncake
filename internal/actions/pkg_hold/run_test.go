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
	held       map[string]bool
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

func TestRun_OnlyAptSupported(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = newStub(t, map[string]bool{})
	step := &config.Step{PkgHold: &config.PkgHold{Name: "x", Manager: "dnf"}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for non-apt manager")
	}
}
