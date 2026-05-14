//nolint:revive // package name follows action convention
package pkg_upgrade

import (
	"runtime"
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

type stub struct {
	upgradeCalls    [][]string
	autoremoveCalls int
}

func newStub(t *testing.T) *stub {
	t.Helper()
	s := &stub{}
	origUp := aptUpgrade
	origAr := aptAutoremove
	origLookPath := lookPath
	aptUpgrade = func(names []string) error {
		cp := append([]string(nil), names...)
		s.upgradeCalls = append(s.upgradeCalls, cp)
		return nil
	}
	aptAutoremove = func() error {
		s.autoremoveCalls++
		return nil
	}
	lookPath = func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	}
	t.Cleanup(func() {
		aptUpgrade = origUp
		aptAutoremove = origAr
		lookPath = origLookPath
	})
	return s
}

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
		{"empty in names", &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"nginx", ""}}}, true},
		{"ok empty (upgrade all)", &config.Step{PkgUpgrade: &config.PkgUpgrade{}}, false},
		{"ok autoremove only", &config.Step{PkgUpgrade: &config.PkgUpgrade{Autoremove: true}}, false},
		{"ok with names", &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"nginx", "curl"}}}, false},
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

func TestApply_NamedUpgrade_NoAutoremove(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"nginx", "curl"}}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected Changed=true; reason=%q", r.Reason)
	}
	if len(s.upgradeCalls) != 1 || len(s.upgradeCalls[0]) != 2 {
		t.Fatalf("expected one upgrade call with [nginx curl]; got %v", s.upgradeCalls)
	}
	if s.upgradeCalls[0][0] != "nginx" || s.upgradeCalls[0][1] != "curl" {
		t.Errorf("unexpected packages: %v", s.upgradeCalls[0])
	}
	if s.autoremoveCalls != 0 {
		t.Errorf("autoremove not requested but ran %d times", s.autoremoveCalls)
	}
}

func TestApply_UpgradeAll_WithAutoremove(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Autoremove: true}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected Changed=true; reason=%q", r.Reason)
	}
	if len(s.upgradeCalls) != 1 || len(s.upgradeCalls[0]) != 0 {
		t.Errorf("expected one upgrade-all call; got %v", s.upgradeCalls)
	}
	if s.autoremoveCalls != 1 {
		t.Errorf("expected autoremove once; got %d", s.autoremoveCalls)
	}
}

func TestPlan_DoesNotInvoke(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"nginx"}, Autoremove: true}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Fatalf("plan should report WouldChange; reason=%q", r.Reason)
	}
	if r.Changed {
		t.Errorf("plan must not flip Changed")
	}
	if len(s.upgradeCalls) != 0 {
		t.Errorf("plan must not call apt upgrade; got %v", s.upgradeCalls)
	}
	if s.autoremoveCalls != 0 {
		t.Errorf("plan must not call autoremove; got %d", s.autoremoveCalls)
	}
}

func TestRun_OnlyAptSupported(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = newStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"x"}, Manager: "dnf"}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for non-apt manager")
	}
}
