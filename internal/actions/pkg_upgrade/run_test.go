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
	"github.com/alehatsman/mooncake/internal/security"
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
	aptUpgrade = func(_ *security.Privileged, names []string) error {
		cp := append([]string(nil), names...)
		s.upgradeCalls = append(s.upgradeCalls, cp)
		return nil
	}
	aptAutoremove = func(_ *security.Privileged) error {
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

// TestRun_UnsupportedManagerErrors — zypper / apk aren't shipped yet;
// the error message names the supported set. The pre-fix repro used
// `manager: dnf` (now valid since the dnf driver landed) and then
// `manager: pacman` (also now valid). `zypper` is the canonical "still
// missing" stand-in.
func TestRun_UnsupportedManagerErrors(t *testing.T) {
	_ = newStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"x"}, Manager: "zypper"}}
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
	brewUpgrade = func(_ *security.Privileged, names []string) error {
		cp := append([]string(nil), names...)
		s.upgradeCalls = append(s.upgradeCalls, cp)
		return nil
	}
	brewAutoremove = func(_ *security.Privileged) error {
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
	brewUpgrade = func(_ *security.Privileged, names []string) error {
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

// dnfStub captures the calls made through the dnf hooks. Parallel to
// stub (apt) and brewStub.
type dnfStub struct {
	upgradeCalls    [][]string
	autoremoveCalls int
}

// newDnfStub wires the dnf hooks + makes lookPath resolve `dnf` (and
// nothing else). Apt-get + brew both report "not found" so
// auto-detect picks dnf.
func newDnfStub(t *testing.T) *dnfStub {
	t.Helper()
	s := &dnfStub{}
	origUp := dnfUpgrade
	origAr := dnfAutoremove
	origLookPath := lookPath
	dnfUpgrade = func(_ *security.Privileged, names []string) error {
		cp := append([]string(nil), names...)
		s.upgradeCalls = append(s.upgradeCalls, cp)
		return nil
	}
	dnfAutoremove = func(_ *security.Privileged) error {
		s.autoremoveCalls++
		return nil
	}
	lookPath = func(name string) (string, error) {
		if name == "dnf" {
			return "/usr/bin/dnf", nil
		}
		return "", errNotFound{}
	}
	t.Cleanup(func() {
		dnfUpgrade = origUp
		dnfAutoremove = origAr
		lookPath = origLookPath
	})
	return s
}

// TestApply_Dnf_NamedUpgrade — explicit subset upgrade through the
// dnf driver. Mirrors TestApply_Brew_NamedUpgrade.
func TestApply_Dnf_NamedUpgrade(t *testing.T) {
	s := newDnfStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"bash", "openssl"}}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected Changed=true; reason=%q", r.Reason)
	}
	if len(s.upgradeCalls) != 1 {
		t.Fatalf("expected one dnf upgrade call; got %v", s.upgradeCalls)
	}
	got := s.upgradeCalls[0]
	if len(got) != 2 || got[0] != "bash" || got[1] != "openssl" {
		t.Errorf("expected [bash openssl]; got %v", got)
	}
	if s.autoremoveCalls != 0 {
		t.Errorf("expected no autoremove; got %d", s.autoremoveCalls)
	}
	if r.Data["manager"] != "dnf" {
		t.Errorf("manager fact = %v, want dnf", r.Data["manager"])
	}
}

// TestApply_Dnf_FullUpgrade_NoNames — Names empty → upgrade all.
func TestApply_Dnf_FullUpgrade_NoNames(t *testing.T) {
	s := newDnfStub(t)
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

// TestApply_Dnf_Autoremove — Autoremove: true triggers a follow-up
// dnf autoremove call after the upgrade. Same shape as apt/brew.
func TestApply_Dnf_Autoremove(t *testing.T) {
	s := newDnfStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Autoremove: true}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected Changed=true; reason=%q", r.Reason)
	}
	if s.autoremoveCalls != 1 {
		t.Errorf("expected one autoremove call; got %d", s.autoremoveCalls)
	}
}

// TestApply_YumAlias — `manager: yum` is canonicalized to "dnf" in
// the result. Same rpm-family driver; just the older CLI spelling.
func TestApply_YumAlias(t *testing.T) {
	s := newDnfStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Manager: "yum", Names: []string{"bash"}}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "dnf" {
		t.Errorf("manager = %v, want dnf (yum canonicalizes to dnf)", r.Data["manager"])
	}
	if len(s.upgradeCalls) != 1 {
		t.Fatalf("expected one upgrade call; got %v", s.upgradeCalls)
	}
}

// TestAutoDetect_PrefersAptOverDnf — multi-manager hosts (very rare
// in the wild but possible with apt-rpm experiments or chroots): apt
// still wins. Sentinel: dnf must not be touched.
func TestAutoDetect_PrefersAptOverDnf(t *testing.T) {
	s := newStub(t)
	origDnf := dnfUpgrade
	dnfUpgrade = func(_ *security.Privileged, names []string) error {
		t.Errorf("dnf should not be invoked when apt-get is on PATH; got %v", names)
		return nil
	}
	t.Cleanup(func() { dnfUpgrade = origDnf })

	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"bash"}}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "apt" {
		t.Errorf("manager = %v, want apt", r.Data["manager"])
	}
	if len(s.upgradeCalls) != 1 {
		t.Errorf("expected apt upgrade call; got %v", s.upgradeCalls)
	}
}

// TestAutoDetect_DnfWhenNoApt — auto-detect lands on dnf on a host
// with dnf but no apt. Pins the order apt > dnf > brew.
func TestAutoDetect_DnfWhenNoApt(t *testing.T) {
	s := newDnfStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"kernel"}}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "dnf" {
		t.Errorf("manager = %v, want dnf", r.Data["manager"])
	}
	if len(s.upgradeCalls) != 1 {
		t.Errorf("expected dnf upgrade call; got %v", s.upgradeCalls)
	}
}

// TestPermissions_BinaryByManager — explicit manager: dnf advertises
// `dnf`; yum alias also advertises `dnf` (the binary preflight; yum
// hosts auto-fall-through to yum at runtime). Apt + brew unchanged.
// Pacman + yay/paru rows pinned in TestPermissions_BinaryByManager_Pacman.
func TestPermissions_BinaryByManager(t *testing.T) {
	cases := []struct {
		manager string
		wantBin string
	}{
		{"apt", "apt-get"},
		{"dnf", "dnf"},
		{"yum", "dnf"},
		{"brew", "brew"},
	}
	for _, c := range cases {
		t.Run(c.manager, func(t *testing.T) {
			step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Manager: c.manager}}
			ps := Handler{}.Permissions(step)
			if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != c.wantBin {
				t.Errorf("manager=%s: RequiredBinaries = %v, want [%s]", c.manager, ps.RequiredBinaries, c.wantBin)
			}
			if !ps.Sudo || !ps.Network {
				t.Errorf("manager=%s: Sudo + Network must remain true; got %+v", c.manager, ps)
			}
		})
	}
}

// pacmanStub captures the calls made through the pacman hooks.
// Parallel to stub (apt), dnfStub, and brewStub.
type pacmanStub struct {
	upgradeCalls    [][]string
	autoremoveCalls int
}

// newPacmanStub wires the pacman hooks + makes lookPath resolve only
// `pacman`. Apt-get + dnf + brew all report "not found" so auto-detect
// picks pacman.
func newPacmanStub(t *testing.T) *pacmanStub {
	t.Helper()
	s := &pacmanStub{}
	origUp := pacmanUpgrade
	origAr := pacmanAutoremove
	origLookPath := lookPath
	pacmanUpgrade = func(_ *security.Privileged, names []string) error {
		cp := append([]string(nil), names...)
		s.upgradeCalls = append(s.upgradeCalls, cp)
		return nil
	}
	pacmanAutoremove = func(_ *security.Privileged) error {
		s.autoremoveCalls++
		return nil
	}
	lookPath = func(name string) (string, error) {
		if name == "pacman" {
			return "/usr/bin/pacman", nil
		}
		return "", errNotFound{}
	}
	t.Cleanup(func() {
		pacmanUpgrade = origUp
		pacmanAutoremove = origAr
		lookPath = origLookPath
	})
	return s
}

// TestApply_Pacman_NamedUpgrade — explicit subset upgrade through
// pacman. Mirrors TestApply_Dnf_NamedUpgrade.
func TestApply_Pacman_NamedUpgrade(t *testing.T) {
	s := newPacmanStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"bash", "linux"}}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected Changed=true; reason=%q", r.Reason)
	}
	if len(s.upgradeCalls) != 1 {
		t.Fatalf("expected one pacman upgrade call; got %v", s.upgradeCalls)
	}
	got := s.upgradeCalls[0]
	if len(got) != 2 || got[0] != "bash" || got[1] != "linux" {
		t.Errorf("expected [bash linux]; got %v", got)
	}
	if s.autoremoveCalls != 0 {
		t.Errorf("expected no autoremove; got %d", s.autoremoveCalls)
	}
	if r.Data["manager"] != "pacman" {
		t.Errorf("manager fact = %v, want pacman", r.Data["manager"])
	}
}

// TestApply_Pacman_FullUpgrade_NoNames — Names empty → upgrade all
// via pacman -Syu. The hook receives an empty slice; how it composes
// the actual command is a unit-of-realPacmanUpgrade detail.
func TestApply_Pacman_FullUpgrade_NoNames(t *testing.T) {
	s := newPacmanStub(t)
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

// TestApply_Pacman_Autoremove — Autoremove: true triggers a follow-up
// pacman -Rns orphans call after the upgrade.
func TestApply_Pacman_Autoremove(t *testing.T) {
	s := newPacmanStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Autoremove: true}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected Changed=true; reason=%q", r.Reason)
	}
	if s.autoremoveCalls != 1 {
		t.Errorf("expected one autoremove call; got %d", s.autoremoveCalls)
	}
}

// TestApply_Pacman_YayAlias / TestApply_Pacman_ParuAlias — `manager:
// yay` and `manager: paru` are canonicalized to "pacman" in the
// result. Same /var/lib/pacman db, just AUR-wrapper CLIs on top.
func TestApply_Pacman_YayAlias(t *testing.T) {
	s := newPacmanStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Manager: "yay", Names: []string{"google-chrome"}}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "pacman" {
		t.Errorf("manager = %v, want pacman (yay canonicalizes)", r.Data["manager"])
	}
	if len(s.upgradeCalls) != 1 {
		t.Fatalf("expected one upgrade call; got %v", s.upgradeCalls)
	}
}

func TestApply_Pacman_ParuAlias(t *testing.T) {
	s := newPacmanStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Manager: "paru", Names: []string{"firefox"}}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "pacman" {
		t.Errorf("manager = %v, want pacman (paru canonicalizes)", r.Data["manager"])
	}
	if len(s.upgradeCalls) != 1 {
		t.Fatalf("expected one upgrade call; got %v", s.upgradeCalls)
	}
}

// TestAutoDetect_PacmanWhenNoAptOrDnf — auto-detect lands on pacman
// when apt-get + dnf + yum are absent. Pins the order
// apt > dnf > pacman > brew.
func TestAutoDetect_PacmanWhenNoAptOrDnf(t *testing.T) {
	s := newPacmanStub(t)
	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"linux"}}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "pacman" {
		t.Errorf("manager = %v, want pacman", r.Data["manager"])
	}
	if len(s.upgradeCalls) != 1 {
		t.Errorf("expected pacman upgrade call; got %v", s.upgradeCalls)
	}
}

// TestAutoDetect_PrefersDnfOverPacman — symmetric to apt-over-dnf:
// dnf wins on the very rare host with BOTH dnf and pacman. Sentinel:
// pacman must not be touched.
func TestAutoDetect_PrefersDnfOverPacman(t *testing.T) {
	s := newDnfStub(t)
	origPacman := pacmanUpgrade
	pacmanUpgrade = func(_ *security.Privileged, names []string) error {
		t.Errorf("pacman should not be invoked when dnf is on PATH; got %v", names)
		return nil
	}
	t.Cleanup(func() { pacmanUpgrade = origPacman })

	step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"bash"}}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "dnf" {
		t.Errorf("manager = %v, want dnf", r.Data["manager"])
	}
	if len(s.upgradeCalls) != 1 {
		t.Errorf("expected dnf upgrade call; got %v", s.upgradeCalls)
	}
}

// TestPermissions_BinaryByManager_Pacman extends the per-manager
// binary preflight matrix to cover pacman + yay + paru.
func TestPermissions_BinaryByManager_Pacman(t *testing.T) {
	cases := []struct {
		manager string
		wantBin string
	}{
		{"pacman", "pacman"},
		{"yay", "pacman"},
		{"paru", "pacman"},
	}
	for _, c := range cases {
		t.Run(c.manager, func(t *testing.T) {
			step := &config.Step{PkgUpgrade: &config.PkgUpgrade{Manager: c.manager}}
			ps := Handler{}.Permissions(step)
			if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != c.wantBin {
				t.Errorf("manager=%s: RequiredBinaries = %v, want [%s]", c.manager, ps.RequiredBinaries, c.wantBin)
			}
			if !ps.Sudo || !ps.Network {
				t.Errorf("manager=%s: Sudo + Network must remain true; got %+v", c.manager, ps)
			}
		})
	}
}
