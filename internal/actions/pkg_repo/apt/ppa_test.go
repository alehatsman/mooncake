package apt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/pkg_repo/shared"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func TestPPARE_Shape(t *testing.T) {
	good := []string{
		"neovim-ppa/unstable",
		"longsleep/ubuntu-golang-backports",
		"a/b",
		"a.1/b_2",
		"deadsnakes/ppa",
	}
	bad := []string{
		"",
		"/",
		"a",
		"foo/",
		"/bar",
		"Foo/bar", // uppercase
		"foo/Bar",
		"foo/bar/baz", // too many parts
		"-foo/bar",    // must start with [a-z0-9]
		"foo/-bar",    // ditto
		"foo bar/baz", // space
		"foo/../etc",  // dot-slash traversal
		"foo/bar:tag", // colon
	}
	for _, s := range good {
		if !PPARE.MatchString(s) {
			t.Errorf("good ppa rejected: %q", s)
		}
	}
	for _, s := range bad {
		if PPARE.MatchString(s) {
			t.Errorf("bad ppa accepted: %q", s)
		}
	}
}

func TestPPAExpansion_FillsDefaults(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	called := 0
	origDiscover := discoverPPAKey
	discoverPPAKey = func(_ context.Context, owner, ppa string) (string, string, error) {
		called++
		if owner != "neovim-ppa" || ppa != "unstable" {
			t.Errorf("discoverPPAKey args = (%q, %q), want (neovim-ppa, unstable)", owner, ppa)
		}
		return "DEADBEEFDEADBEEF", "https://fake-keyserver/lookup?op=get&search=0xDEADBEEFDEADBEEF", nil
	}
	t.Cleanup(func() { discoverPPAKey = origDiscover })

	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "neovim-ppa",
		Apt:  &config.PkgRepoApt{PPA: "neovim-ppa/unstable"},
	}}
	r := mustRunWithCodename(t, false, step, "jammy")
	if !r.Changed {
		t.Fatal("expected change on create")
	}
	if called != 1 {
		t.Errorf("discoverPPAKey called %d times; want 1", called)
	}

	sourcesPath := filepath.Join(s.sourcesDir, "neovim-ppa.sources")
	got, err := os.ReadFile(sourcesPath)
	if err != nil {
		t.Fatalf("read sources: %v", err)
	}
	wantLines := []string{
		"URIs: http://ppa.launchpad.net/neovim-ppa/unstable/ubuntu",
		"Suites: jammy",
		"Components: main",
		"Signed-By: " + filepath.Join(s.keyringsDir, "neovim-ppa.gpg"),
	}
	for _, line := range wantLines {
		if !strings.Contains(string(got), line) {
			t.Errorf("DEB822 missing %q\n got:\n%s", line, got)
		}
	}
	if _, err := os.Stat(filepath.Join(s.keyringsDir, "neovim-ppa.gpg")); err != nil {
		t.Errorf("keyring not written: %v", err)
	}
}

func TestPPAExpansion_RespectsOperatorOverrides(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = newStubFS(t)
	called := 0
	origDiscover := discoverPPAKey
	discoverPPAKey = func(_ context.Context, _, _ string) (string, string, error) {
		called++
		return "AAAA", "https://x", nil
	}
	t.Cleanup(func() { discoverPPAKey = origDiscover })

	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "neovim-ppa",
		Apt: &config.PkgRepoApt{
			PPA:               "neovim-ppa/unstable",
			Suites:            []string{"focal"}, // override codename
			Components:        []string{"main", "universe"},
			GPGKeyFingerprint: "CAFECAFECAFECAFE", // pre-pinned
		},
	}}
	r := mustRunWithCodename(t, false, step, "jammy")
	if !r.Changed {
		t.Fatal("expected change on create")
	}
	if called != 0 {
		t.Errorf("operator pre-pinned fingerprint should skip launchpad lookup; called=%d", called)
	}
	got, _ := os.ReadFile(filepath.Join("/tmp", "ignored"))
	_ = got // (path is determined by stubFS; checked below via sourcesDir)

	sourcesPath := filepath.Join(newStubFSPaths(t).sourcesDir, "neovim-ppa.sources")
	// We replaced paths above via newStubFS; re-derive via current paths.
	sourcesPath = filepath.Join(paths.SourcesDir, "neovim-ppa.sources")
	body, err := os.ReadFile(sourcesPath)
	if err != nil {
		t.Fatalf("read sources: %v", err)
	}
	for _, line := range []string{
		"URIs: http://ppa.launchpad.net/neovim-ppa/unstable/ubuntu",
		"Suites: focal",
		"Components: main universe",
	} {
		if !strings.Contains(string(body), line) {
			t.Errorf("missing %q in DEB822 body:\n%s", line, body)
		}
	}
}

func TestPPAExpansion_MissingCodenameFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = newStubFS(t)
	origDiscover := discoverPPAKey
	discoverPPAKey = func(_ context.Context, _, _ string) (string, string, error) {
		return "AAAA", "https://x", nil
	}
	t.Cleanup(func() { discoverPPAKey = origDiscover })

	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "neovim-ppa",
		Apt:  &config.PkgRepoApt{PPA: "neovim-ppa/unstable"},
	}}
	result := executor.NewResult()
	result.Checkable = true
	_, err := Run(newCtxWithCodename(t, false, ""), step.PkgRepo, result)
	if err == nil {
		t.Fatal("expected error when codename is unset")
	}
	if !strings.Contains(err.Error(), "distribution_codename") {
		t.Errorf("error should mention distribution_codename; got %q", err.Error())
	}
}

func TestPPAExpansion_AbsentSkipsLaunchpad(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = newStubFS(t)
	origDiscover := discoverPPAKey
	called := 0
	discoverPPAKey = func(_ context.Context, _, _ string) (string, string, error) {
		called++
		return "AAAA", "https://x", nil
	}
	t.Cleanup(func() { discoverPPAKey = origDiscover })

	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name:  "neovim-ppa",
		State: "absent",
		Apt:   &config.PkgRepoApt{PPA: "neovim-ppa/unstable"},
	}}
	_ = mustRunWithCodename(t, false, step, "jammy")
	if called != 0 {
		t.Errorf("absent must not contact launchpad; called=%d", called)
	}
}

func TestPPAExpansion_LaunchpadErrorPropagates(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = newStubFS(t)
	origDiscover := discoverPPAKey
	discoverPPAKey = func(_ context.Context, _, _ string) (string, string, error) {
		return "", "", errors.New("launchpad down")
	}
	t.Cleanup(func() { discoverPPAKey = origDiscover })

	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "neovim-ppa",
		Apt:  &config.PkgRepoApt{PPA: "neovim-ppa/unstable"},
	}}
	result := executor.NewResult()
	result.Checkable = true
	_, err := Run(newCtxWithCodename(t, false, "jammy"), step.PkgRepo, result)
	if err == nil {
		t.Fatal("expected launchpad error to surface")
	}
	if !strings.Contains(err.Error(), "launchpad down") {
		t.Errorf("error should wrap launchpad failure; got %q", err.Error())
	}
}

func TestKeyserverURL_Format(t *testing.T) {
	orig := KeyserverBase
	KeyserverBase = "https://keyserver.example.com/pks/lookup"
	t.Cleanup(func() { KeyserverBase = orig })
	got := KeyserverURL("DEADBEEF")
	want := "https://keyserver.example.com/pks/lookup?op=get&search=0xDEADBEEF"
	if got != want {
		t.Errorf("KeyserverURL = %q, want %q", got, want)
	}
}

func TestPPAExpansion_NameTemplating(t *testing.T) {
	// The PPA value itself isn't currently template-rendered (it's a
	// literal launchpad slug); confirm the validator regex still
	// matches a plain literal.
	if !PPARE.MatchString("deadsnakes/ppa") {
		t.Fatal("regex regression")
	}
}

// --- helpers ----------------------------------------------------------------

func newCtxWithCodename(t *testing.T, plan bool, codename string) *executor.ExecutionContext {
	t.Helper()
	ctx := newCtx(t, plan)
	if codename != "" {
		ctx.Scope.Facts = &facts.Facts{DistributionCodename: codename}
	}
	return ctx
}

func mustRunWithCodename(t *testing.T, plan bool, step *config.Step, codename string) *executor.Result {
	t.Helper()
	result := executor.NewResult()
	result.Checkable = true
	res, err := Run(newCtxWithCodename(t, plan, codename), step.PkgRepo, result)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res.(*executor.Result)
}

// newStubFSPaths is a typed shim to access paths without re-stubbing.
func newStubFSPaths(t *testing.T) struct{ sourcesDir, keyringsDir string } {
	return struct{ sourcesDir, keyringsDir string }{paths.SourcesDir, paths.KeyringsDir}
}

// silence unused-import vet noise across go versions that drop unused imports.
var _ = actions.ModeApply
var _ = logger.ErrorLevel
var _ = template.NewPongo2Renderer
var _ = pathutil.NewPathExpander
var _ = shared.StatePresent
