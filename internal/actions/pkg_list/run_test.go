//nolint:revive // package name follows action convention
package pkg_list

import (
	"os/exec"
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

func stubDpkgQuery(t *testing.T, stdout string) {
	t.Helper()
	orig := dpkgQuery
	origLook := lookPath
	dpkgQuery = func() (string, error) { return stdout, nil }
	lookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() {
		dpkgQuery = orig
		lookPath = origLook
	})
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
		{"empty ok", &config.Step{PkgList: &config.PkgList{}}, false},
		{"explicit manager ok", &config.Step{PkgList: &config.PkgList{Manager: "apt"}}, false},
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

const sampleDpkg = "zlib1g\t1:1.2.13-1\n" +
	"curl\t7.88.1-10\n" +
	"nginx\t1.24.0-2\n" +
	"\n" + // blank line ignored
	"malformed-no-tab\n" + // skipped
	"libc6\t2.36-9\n"

func TestApply_ReturnsSortedPackages(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubDpkgQuery(t, sampleDpkg)
	step := &config.Step{PkgList: &config.PkgList{}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("pkg.list must never report Changed")
	}
	pkgs, ok := r.Data["packages"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected packages []map[string]interface{}; got %T", r.Data["packages"])
	}
	wantNames := []string{"curl", "libc6", "nginx", "zlib1g"}
	if len(pkgs) != len(wantNames) {
		t.Fatalf("expected %d packages; got %d (%v)", len(wantNames), len(pkgs), pkgs)
	}
	for i, want := range wantNames {
		got := pkgs[i]["name"].(string)
		if got != want {
			t.Errorf("packages[%d].name = %q; want %q", i, got, want)
		}
		if v, ok := pkgs[i]["version"].(string); !ok || v == "" {
			t.Errorf("packages[%d].version missing", i)
		}
		if pkgs[i]["manager"] != "apt" {
			t.Errorf("packages[%d].manager = %v; want apt", i, pkgs[i]["manager"])
		}
	}
	if c, ok := r.Data["count"].(int); !ok || c != len(wantNames) {
		t.Errorf("count mismatch: %v", r.Data["count"])
	}
}

func TestPlanAndApply_AreIdentical(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubDpkgQuery(t, sampleDpkg)
	step := &config.Step{PkgList: &config.PkgList{}}
	applied := mustRun(t, false, step)
	planned := mustRun(t, true, step)
	if applied.Changed || planned.Changed {
		t.Errorf("Changed must be false in both modes")
	}
	if planned.WouldChange {
		t.Errorf("pkg.list must not flip WouldChange (query is read-only)")
	}
	ap := applied.Data["packages"].([]map[string]interface{})
	pp := planned.Data["packages"].([]map[string]interface{})
	if len(ap) != len(pp) {
		t.Fatalf("plan/apply differ in length: %d vs %d", len(ap), len(pp))
	}
	for i := range ap {
		if ap[i]["name"] != pp[i]["name"] || ap[i]["version"] != pp[i]["version"] {
			t.Errorf("plan/apply differ at %d: %v vs %v", i, ap[i], pp[i])
		}
	}
}

func TestApply_ExplicitManagerApt(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubDpkgQuery(t, "foo\t1.0\n")
	step := &config.Step{PkgList: &config.PkgList{Manager: "apt"}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("must not report Changed")
	}
	pkgs := r.Data["packages"].([]map[string]interface{})
	if len(pkgs) != 1 || pkgs[0]["name"] != "foo" {
		t.Errorf("expected [foo]; got %v", pkgs)
	}
}

// TestRun_UnsupportedManagerRejected — non-{apt,dnf,brew} routing
// produces a clear error. The original "only apt is supported" test
// became stale once the dnf driver shipped; now we just pin the
// negative path against a manager we don't ship a driver for.
func TestRun_UnsupportedManagerRejected(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubDpkgQuery(t, "")
	step := &config.Step{PkgList: &config.PkgList{Manager: "pacman"}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for unsupported manager")
	}
}

// stubBrewList replaces the brew binary call + makes lookPath
// resolve `brew` (but NOT `dpkg-query`, so auto-detection picks
// brew instead of falling through to apt).
func stubBrewList(t *testing.T, stdout string) {
	t.Helper()
	origBrew := brewList
	origLook := lookPath
	brewList = func() (string, error) { return stdout, nil }
	lookPath = func(name string) (string, error) {
		if name == "brew" {
			return "/opt/homebrew/bin/brew", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() {
		brewList = origBrew
		lookPath = origLook
	})
}

const sampleBrew = "git 2.43.0\n" +
	"zsh 5.9\n" +
	"node@20 20.10.0 20.10.1\n" + // multi-version: last wins
	"\n" + // blank line ignored
	"orphaned-no-version\n" + // skipped (single field)
	"jq 1.7\n"

// TestApply_Brew_ReturnsSortedPackages — brew detection + parse +
// sort. Mirrors TestApply_ReturnsSortedPackages but for the darwin
// path. Stubs make it host-agnostic.
func TestApply_Brew_ReturnsSortedPackages(t *testing.T) {
	stubBrewList(t, sampleBrew)
	step := &config.Step{PkgList: &config.PkgList{}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("pkg.list must never report Changed")
	}
	pkgs := r.Data["packages"].([]map[string]interface{})
	wantNames := []string{"git", "jq", "node@20", "zsh"}
	if len(pkgs) != len(wantNames) {
		t.Fatalf("got %d packages, want %d: %v", len(pkgs), len(wantNames), pkgs)
	}
	for i, want := range wantNames {
		if pkgs[i]["name"] != want {
			t.Errorf("packages[%d].name = %v, want %s", i, pkgs[i]["name"], want)
		}
		if pkgs[i]["manager"] != "brew" {
			t.Errorf("packages[%d].manager = %v, want brew", i, pkgs[i]["manager"])
		}
	}
	if r.Data["manager"] != "brew" {
		t.Errorf("manager fact = %v, want brew", r.Data["manager"])
	}
}

// TestApply_Brew_LastVersionWins — `brew list --versions` can emit
// multiple versions per line when several slots are installed
// (`node@18` + `node@20`). We report the last token as the version,
// matching the "most recently linked" convention.
func TestApply_Brew_LastVersionWins(t *testing.T) {
	stubBrewList(t, "python@3.11 3.11.5 3.11.7\n")
	step := &config.Step{PkgList: &config.PkgList{}}
	r := mustRun(t, false, step)
	pkgs := r.Data["packages"].([]map[string]interface{})
	if len(pkgs) != 1 {
		t.Fatalf("got %d packages, want 1: %v", len(pkgs), pkgs)
	}
	if pkgs[0]["version"] != "3.11.7" {
		t.Errorf("version = %v, want 3.11.7", pkgs[0]["version"])
	}
}

// TestApply_ExplicitManagerBrew — explicit `manager: brew` routes
// through the brew branch even when dpkg-query is on PATH. Pins the
// operator-override semantic.
func TestApply_ExplicitManagerBrew(t *testing.T) {
	stubBrewList(t, "git 2.43.0\n")
	// Force lookPath to find BOTH binaries — explicit manager must
	// win regardless of detection order.
	origLook := lookPath
	lookPath = func(_ string) (string, error) { return "/usr/local/bin/x", nil }
	t.Cleanup(func() { lookPath = origLook })

	step := &config.Step{PkgList: &config.PkgList{Manager: "brew"}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "brew" {
		t.Errorf("manager = %v, want brew", r.Data["manager"])
	}
}

// TestAutoDetect_PrefersAptWhenBoth — on multi-manager hosts
// (e.g. Linuxbrew on a Debian box), the system-level package list
// is more authoritative than per-user brew. Detection picks apt
// first, brew second. Operator can override via explicit manager:.
func TestAutoDetect_PrefersAptWhenBoth(t *testing.T) {
	stubDpkgQuery(t, "apt-pkg\t1.0\n")
	// stubDpkgQuery already sets lookPath to succeed for all names,
	// so brew would also be "detected" — apt should still win.
	stubBrewList := func() (string, error) {
		t.Errorf("brew should not be queried when dpkg-query is available")
		return "", nil
	}
	origBrew := brewList
	brewList = stubBrewList
	t.Cleanup(func() { brewList = origBrew })

	step := &config.Step{PkgList: &config.PkgList{}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "apt" {
		t.Errorf("manager = %v, want apt (auto-detect must prefer apt)", r.Data["manager"])
	}
}

// TestPermissions_BinaryByGOOS — darwin advertises `brew`, linux
// (default) advertises `dpkg-query`. Spec-44 doctor consults this
// to flag missing binaries before runtime.
func TestPermissions_BinaryByGOOS(t *testing.T) {
	ps := (Handler{}).Permissions(nil)
	wantBin := "dpkg-query"
	if runtime.GOOS == "darwin" {
		wantBin = "brew"
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != wantBin {
		t.Errorf("RequiredBinaries = %v, want [%s]", ps.RequiredBinaries, wantBin)
	}
}

// TestMetadata_AdvertisesLinuxAndDarwin guards the SupportedPlatforms
// expansion. If darwin is later removed, this test fires to force a
// doc + UX update in lockstep.
func TestMetadata_AdvertisesLinuxAndDarwin(t *testing.T) {
	m := (&Handler{}).Metadata()
	want := map[string]bool{"linux": true, "darwin": true}
	got := map[string]bool{}
	for _, p := range m.SupportedPlatforms {
		got[p] = true
	}
	for p := range want {
		if !got[p] {
			t.Errorf("SupportedPlatforms missing %s: %v", p, m.SupportedPlatforms)
		}
	}
}

// TestParseBrewList_DirectShapes pins the parser without going
// through Run, so output-format regressions surface at the parsing
// level instead of as a mis-sorted Data["packages"].
func TestParseBrewList_DirectShapes(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantNames []string
	}{
		{"empty", "", nil},
		{"single", "foo 1.0\n", []string{"foo"}},
		{"trailing newline", "foo 1.0\nbar 2.0\n", []string{"foo", "bar"}},
		{"skip nameless", "  \nfoo 1.0\n", []string{"foo"}},
		{"skip orphan", "noversion\nfoo 1.0\n", []string{"foo"}},
		{"@-suffix kept", "node@20 20.10.0\n", []string{"node@20"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseBrewList(c.in, "brew")
			if len(got) != len(c.wantNames) {
				t.Fatalf("got %d, want %d: %v", len(got), len(c.wantNames), got)
			}
			for i, n := range c.wantNames {
				if got[i]["name"] != n {
					t.Errorf("[%d].name = %v, want %s", i, got[i]["name"], n)
				}
			}
		})
	}
}

// stubRpmQuery replaces the rpm binary call + makes lookPath resolve
// `rpm` (but NOT `dpkg-query`, so auto-detection picks dnf instead of
// falling through to apt). Mirrors stubBrewList.
func stubRpmQuery(t *testing.T, stdout string) {
	t.Helper()
	origRpm := rpmQuery
	origLook := lookPath
	rpmQuery = func() (string, error) { return stdout, nil }
	lookPath = func(name string) (string, error) {
		if name == "rpm" {
			return "/usr/bin/rpm", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() {
		rpmQuery = origRpm
		lookPath = origLook
	})
}

const sampleRpm = "bash\t5.1.8-9.el9\n" +
	"curl\t7.76.1-26.el9_3.2\n" +
	"\n" + // blank line ignored
	"malformed-no-tab\n" + // skipped
	"glibc\t2.34-83.el9_3.7\n" +
	"systemd\t252-18.el9_3.4\n"

// TestApply_Dnf_ReturnsSortedPackages — rpm detection + parse + sort.
// Mirrors TestApply_ReturnsSortedPackages for the RHEL family. Stubs
// make the test host-agnostic (runs on any Linux with go test).
func TestApply_Dnf_ReturnsSortedPackages(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubRpmQuery(t, sampleRpm)
	step := &config.Step{PkgList: &config.PkgList{}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("pkg.list must never report Changed")
	}
	pkgs, ok := r.Data["packages"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected packages []map[string]interface{}; got %T", r.Data["packages"])
	}
	wantNames := []string{"bash", "curl", "glibc", "systemd"}
	if len(pkgs) != len(wantNames) {
		t.Fatalf("expected %d packages; got %d (%v)", len(wantNames), len(pkgs), pkgs)
	}
	for i, want := range wantNames {
		if pkgs[i]["name"] != want {
			t.Errorf("packages[%d].name = %v; want %s", i, pkgs[i]["name"], want)
		}
		if v, ok := pkgs[i]["version"].(string); !ok || v == "" {
			t.Errorf("packages[%d].version missing", i)
		}
		if pkgs[i]["manager"] != "dnf" {
			t.Errorf("packages[%d].manager = %v; want dnf", i, pkgs[i]["manager"])
		}
	}
	if r.Data["manager"] != "dnf" {
		t.Errorf("manager fact = %v, want dnf", r.Data["manager"])
	}
}

// TestApply_Dnf_VersionReleaseJoined — rpm versions are reported as
// `version-release` (e.g. `5.1.8-9.el9`). The parser must keep them
// joined; splitting on the dash would corrupt names containing dashes
// in their version-release component.
func TestApply_Dnf_VersionReleaseJoined(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubRpmQuery(t, "openssl\t3.0.7-25.el9_3\n")
	r := mustRun(t, false, &config.Step{PkgList: &config.PkgList{}})
	pkgs := r.Data["packages"].([]map[string]interface{})
	if len(pkgs) != 1 || pkgs[0]["version"] != "3.0.7-25.el9_3" {
		t.Errorf("expected openssl 3.0.7-25.el9_3; got %v", pkgs)
	}
}

// TestApply_ExplicitManagerDnf — explicit `manager: dnf` routes
// through the rpm branch even when dpkg-query is on PATH.
func TestApply_ExplicitManagerDnf(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubRpmQuery(t, "vim\t9.0.1879-1\n")
	// Force lookPath to find BOTH binaries — explicit manager must
	// win regardless of detection order.
	origLook := lookPath
	lookPath = func(_ string) (string, error) { return "/usr/bin/x", nil }
	t.Cleanup(func() { lookPath = origLook })

	step := &config.Step{PkgList: &config.PkgList{Manager: "dnf"}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "dnf" {
		t.Errorf("manager = %v, want dnf", r.Data["manager"])
	}
}

// TestApply_YumAlias — `manager: yum` is a synonym for `manager: dnf`
// since rpm is the same database on both. The result canonicalizes to
// "dnf" so downstream consumers don't need to handle two spellings.
func TestApply_YumAlias(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubRpmQuery(t, "yum-utils\t4.3.0-13.el9\n")
	origLook := lookPath
	lookPath = func(_ string) (string, error) { return "/usr/bin/x", nil }
	t.Cleanup(func() { lookPath = origLook })

	step := &config.Step{PkgList: &config.PkgList{Manager: "yum"}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "dnf" {
		t.Errorf("manager = %v, want dnf (yum canonicalizes to dnf)", r.Data["manager"])
	}
	pkgs := r.Data["packages"].([]map[string]interface{})
	if len(pkgs) != 1 || pkgs[0]["manager"] != "dnf" {
		t.Errorf("per-package manager should be dnf; got %v", pkgs)
	}
}

// TestAutoDetect_DnfWhenNoApt — auto-detect picks dnf on a host with
// rpm but no dpkg-query. Pins the order apt > dnf > brew so a Linux
// box without apt routes to the rpm path instead of erroring.
func TestAutoDetect_DnfWhenNoApt(t *testing.T) {
	stubRpmQuery(t, "kernel\t5.14.0-362.18.1.el9_3\n")
	// stubRpmQuery's lookPath only resolves `rpm`; dpkg-query + brew
	// both return ErrNotFound. Auto-detect must land on dnf.
	step := &config.Step{PkgList: &config.PkgList{}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "dnf" {
		t.Errorf("manager = %v, want dnf (auto-detect must fall through apt→dnf when apt is absent)", r.Data["manager"])
	}
}

// TestAutoDetect_PrefersAptOverDnf — on the very rare host with BOTH
// dpkg-query and rpm (e.g. someone running apt-rpm hybrid setups, or
// a Debian-on-rpm experiment), apt wins. Symmetric to
// TestAutoDetect_PrefersAptWhenBoth (apt vs brew).
func TestAutoDetect_PrefersAptOverDnf(t *testing.T) {
	stubDpkgQuery(t, "apt-pkg\t1.0\n")
	rpmStub := func() (string, error) {
		t.Errorf("rpm should not be queried when dpkg-query is available")
		return "", nil
	}
	origRpm := rpmQuery
	rpmQuery = rpmStub
	t.Cleanup(func() { rpmQuery = origRpm })

	step := &config.Step{PkgList: &config.PkgList{}}
	r := mustRun(t, false, step)
	if r.Data["manager"] != "apt" {
		t.Errorf("manager = %v, want apt (auto-detect must prefer apt over dnf)", r.Data["manager"])
	}
}

// TestPermissions_BinaryByManager — explicit manager: dnf advertises
// `rpm`, manager: apt advertises `dpkg-query`. Closes the doctor-
// preflight gap for dnf hosts that pre-fix would always report
// dpkg-query missing.
func TestPermissions_BinaryByManager(t *testing.T) {
	cases := []struct {
		manager string
		wantBin string
	}{
		{"apt", "dpkg-query"},
		{"dnf", "rpm"},
		{"yum", "rpm"},
		{"brew", "brew"},
	}
	for _, c := range cases {
		t.Run(c.manager, func(t *testing.T) {
			step := &config.Step{PkgList: &config.PkgList{Manager: c.manager}}
			ps := Handler{}.Permissions(step)
			if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != c.wantBin {
				t.Errorf("manager=%s: RequiredBinaries = %v, want [%s]", c.manager, ps.RequiredBinaries, c.wantBin)
			}
		})
	}
}

// TestParseRpmQuery_DirectShapes pins the parser without going
// through Run, so output-format regressions surface at the parsing
// level instead of as a mis-sorted Data["packages"].
func TestParseRpmQuery_DirectShapes(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantNames []string
	}{
		{"empty", "", nil},
		{"single", "foo\t1.0-1\n", []string{"foo"}},
		{"trailing newline", "foo\t1.0-1\nbar\t2.0-1\n", []string{"foo", "bar"}},
		{"skip blank", "  \nfoo\t1.0-1\n", []string{"foo"}},
		{"skip orphan", "noversion\nfoo\t1.0-1\n", []string{"foo"}},
		{"dot-suffixed release", "openssl\t3.0.7-25.el9_3\n", []string{"openssl"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseRpmQuery(c.in, "dnf")
			if len(got) != len(c.wantNames) {
				t.Fatalf("got %d, want %d: %v", len(got), len(c.wantNames), got)
			}
			for i, n := range c.wantNames {
				if got[i]["name"] != n {
					t.Errorf("[%d].name = %v, want %s", i, got[i]["name"], n)
				}
			}
		})
	}
}
