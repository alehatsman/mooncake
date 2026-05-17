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

func TestRun_UnsupportedManagerErrors(t *testing.T) {
	_ = newStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"x"}, Manager: "dnf"}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for unsupported manager")
	}
}

// brewStub captures the calls made through the brew hooks. Parallel
// to stub for the apt path.
type brewStub struct {
	upgradeCalls    [][]string
	autoremoveCalls int
}

func newBrewStub(t *testing.T) *brewStub {
	t.Helper()
	s := &brewStub{}
	origUp := brewUpgrade
	origAr := brewAutoremove
	origLookPath := lookPath
	brewUpgrade = func(names []string) error {
		cp := append([]string(nil), names...)
		s.upgradeCalls = append(s.upgradeCalls, cp)
		return nil
	}
	brewAutoremove = func() error {
		s.autoremoveCalls++
		return nil
	}
	// Brew-only on PATH: apt-get missing forces auto-detect to pick
	// brew when manager: is unset.
	lookPath = func(name string) (string, error) {
		if name == "brew" {
			return "/opt/homebrew/bin/brew", nil
		}
		return "", errNotFound{}
	}
	t.Cleanup(func() {
		brewUpgrade = origUp
		brewAutoremove = origAr
		lookPath = origLookPath
	})
	return s
}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

func TestApply_Brew_NamedUpgrade(t *testing.T) {
	s := newBrewStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"git", "jq"}}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected Changed=true; reason=%q", r.Reason)
	}
	if len(s.upgradeCalls) != 1 {
		t.Fatalf("expected one brew upgrade call; got %v", s.upgradeCalls)
	}
	got := s.upgradeCalls[0]
	if len(got) != 2 || got[0] != "git" || got[1] != "jq" {
		t.Errorf("expected [git jq]; got %v", got)
	}
	if s.autoremoveCalls != 0 {
		t.Errorf("expected no autoremove; got %d", s.autoremoveCalls)
	}
	if r.Data["manager"] != "brew" {
		t.Errorf("manager fact = %v, want brew", r.Data["manager"])
	}
}

func TestApply_Brew_FullUpgrade_NoNames(t *testing.T) {
	s := newBrewStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected Changed=true; reason=%q", r.Reason)
	}
	if len(s.upgradeCalls) != 1 {
		t.Fatalf("expected one upgrade call; got %v", s.upgradeCalls)
	}
	if len(s.upgradeCalls[0]) != 0 {
		t.Errorf("expected empty names slice (full upgrade); got %v", s.upgradeCalls[0])
	}
}

func TestApply_Brew_Autoremove(t *testing.T) {
	s := newBrewStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Autoremove: true}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected Changed=true; reason=%q", r.Reason)
	}
	if s.autoremoveCalls != 1 {
		t.Errorf("expected one autoremove call; got %d", s.autoremoveCalls)
	}
}

// TestAutoDetect_PrefersAptOverBrew — multi-manager hosts: apt wins.
func TestAutoDetect_PrefersAptOverBrew(t *testing.T) {
	s := newStub(t)
	// Sentinel: brew should not be invoked.
	origBrew := brewUpgrade
	brewUpgrade = func(names []string) error {
		t.Errorf("brew should not be invoked when apt-get is on PATH; got %v", names)
		return nil
	}
	t.Cleanup(func() { brewUpgrade = origBrew })

	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"git"}}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "apt" {
		t.Errorf("manager = %v, want apt", r.Data["manager"])
	}
	if len(s.upgradeCalls) != 1 {
		t.Errorf("expected apt upgrade call; got %v", s.upgradeCalls)
	}
}

// TestPermissions_BinaryByGOOS pins the host-shaped RequiredBinaries.
func TestPermissions_BinaryByGOOS(t *testing.T) {
	ps := (Handler{}).Permissions(nil)
	wantBin := "apt-get"
	if runtime.GOOS == "darwin" {
		wantBin = "brew"
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != wantBin {
		t.Errorf("RequiredBinaries = %v, want [%s]", ps.RequiredBinaries, wantBin)
	}
	if !ps.Sudo || !ps.Network {
		t.Errorf("Sudo + Network must remain true; got %+v", ps)
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
