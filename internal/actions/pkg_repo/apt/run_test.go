package apt

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/pkg_repo/shared"
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

// stubFS overrides the package-level apt paths + hooks for the
// duration of a test. The fake key fetcher returns a fixed byte
// sequence; the cache-updater records whether it was called.
// shared.VerifyKeyFingerprint defaults to a no-op stub so existing
// happy-path tests (which use a fake key body that can't be parsed
// by the real openpgp verifier) keep passing.
type stubFS struct {
	sourcesDir   string
	keyringsDir  string
	keyBody      []byte
	updateCalled int
}

func newStubFS(t *testing.T) *stubFS {
	t.Helper()
	dir := t.TempDir()
	s := &stubFS{
		sourcesDir:  filepath.Join(dir, "sources.list.d"),
		keyringsDir: filepath.Join(dir, "keyrings"),
		keyBody:     []byte("-----BEGIN PGP PUBLIC KEY-----\nfake\n-----END PGP PUBLIC KEY-----\n"),
	}
	originalPaths := paths
	originalFetch := shared.HTTPFetchKey
	originalUpdate := updateCache
	originalVerify := shared.VerifyKeyFingerprint
	paths = Paths{SourcesDir: s.sourcesDir, KeyringsDir: s.keyringsDir}
	shared.HTTPFetchKey = func(string) ([]byte, error) { return s.keyBody, nil }
	shared.VerifyKeyFingerprint = func([]byte, string) error { return nil } // no-op stub
	updateCache = func(_ actions.PrivilegedRunner) error {
		s.updateCalled++
		return nil
	}
	t.Cleanup(func() {
		paths = originalPaths
		shared.HTTPFetchKey = originalFetch
		updateCache = originalUpdate
		shared.VerifyKeyFingerprint = originalVerify
	})
	return s
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

func TestApply_CreatesSourcesAndKeyringAndCallsUpdate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "nodesource",
		Apt: &config.PkgRepoApt{
			URI:               "https://deb.nodesource.com/node_20.x",
			Suites:            []string{"nodistro"},
			Components:        []string{"main"},
			GPGKeyURL:         "https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key",
			GPGKeyFingerprint: "9FD3B784BC1C6FC31A8A0A1C1655A0AB68576280",
		},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatal("expected change on create")
	}
	sourcesPath := filepath.Join(s.sourcesDir, "nodesource.sources")
	keyringPath := filepath.Join(s.keyringsDir, "nodesource.gpg")
	got, err := os.ReadFile(sourcesPath)
	if err != nil {
		t.Fatalf("read sources: %v", err)
	}
	want := strings.Join([]string{
		"# Managed by mooncake pkg.repo. Do not edit by hand.",
		"Types: deb",
		"URIs: https://deb.nodesource.com/node_20.x",
		"Suites: nodistro",
		"Components: main",
		"Signed-By: " + keyringPath,
		"",
	}, "\n")
	if string(got) != want {
		t.Errorf("DEB822 mismatch\n got %q\nwant %q", got, want)
	}
	if _, err := os.Stat(keyringPath); err != nil {
		t.Errorf("keyring not written: %v", err)
	}
	if s.updateCalled != 1 {
		t.Errorf("expected apt-get update once, got %d", s.updateCalled)
	}
}

func TestApply_NoKeyringWhenNotProvided(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	boolFalse := false
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "local",
		Apt: &config.PkgRepoApt{
			URI:      "http://localhost:8080/deb",
			Suites:   []string{"stable"},
			GPGCheck: &boolFalse, // explicitly disable to skip key requirement
		},
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(filepath.Join(s.sourcesDir, "local.sources"))
	if strings.Contains(string(got), "Signed-By") {
		t.Errorf("should not emit Signed-By when no keyring; got %q", got)
	}
	entries, _ := os.ReadDir(s.keyringsDir)
	if len(entries) != 0 {
		t.Errorf("keyrings dir should be untouched; got %d entries", len(entries))
	}
}

func TestApply_Idempotent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "nodesource",
		Apt: &config.PkgRepoApt{
			URI:               "https://deb.nodesource.com/node_20.x",
			Suites:            []string{"nodistro"},
			GPGKeyURL:         "https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key",
			GPGKeyFingerprint: "9FD3...",
		},
	}}
	_ = mustRun(t, false, step)
	first := s.updateCalled
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("second run should be no-op; reason=%q", r.Reason)
	}
	if s.updateCalled != first {
		t.Errorf("apt-get update should not be called when nothing changed (was %d, now %d)", first, s.updateCalled)
	}
}

func TestApply_UpdateDetectsDrift(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "nodesource",
		Apt: &config.PkgRepoApt{
			URI:               "https://deb.nodesource.com/node_18.x",
			Suites:            []string{"nodistro"},
			GPGKeyURL:         "https://k",
			GPGKeyFingerprint: "ABCD",
		},
	}}
	_ = mustRun(t, false, step)

	step.PkgRepo.Apt.URI = "https://deb.nodesource.com/node_20.x"
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatal("expected drift to trigger change")
	}
	got, _ := os.ReadFile(filepath.Join(s.sourcesDir, "nodesource.sources"))
	if !strings.Contains(string(got), "node_20.x") {
		t.Errorf("URI not updated: %q", got)
	}
}

func TestAbsent_RemovesSources(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "old",
		Apt: &config.PkgRepoApt{
			URI:               "https://example.com",
			Suites:            []string{"stable"},
			GPGKeyURL:         "https://k",
			GPGKeyFingerprint: "ABCD",
		},
	}}
	_ = mustRun(t, false, step)
	step.PkgRepo.State = "absent"
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatal("expected delete")
	}
	if _, err := os.Stat(filepath.Join(s.sourcesDir, "old.sources")); !os.IsNotExist(err) {
		t.Errorf("sources file should be removed; stat err=%v", err)
	}
}

func TestAbsent_NoopWhenMissing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name:  "never",
		State: "absent",
		Apt:   &config.PkgRepoApt{},
	}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("absent on missing source should be no-op; reason=%q", r.Reason)
	}
}

func TestPlan_DoesNotWrite(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "nodesource",
		Apt: &config.PkgRepoApt{
			URI:               "https://deb.nodesource.com/node_20.x",
			Suites:            []string{"nodistro"},
			GPGKeyURL:         "https://k",
			GPGKeyFingerprint: "ABCD",
		},
	}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange; reason=%q", r.Reason)
	}
	if _, err := os.Stat(filepath.Join(s.sourcesDir, "nodesource.sources")); err == nil {
		t.Error("plan must not write sources file")
	}
	if s.updateCalled != 0 {
		t.Errorf("plan must not run apt-get update; called %d times", s.updateCalled)
	}
}

// TestApply_FingerprintMismatchRefuses — F034. When the operator
// supplies a `gpg_key_fingerprint` and the fetched key doesn't carry
// it, Run must refuse and NOT write the keyring.
func TestApply_FingerprintMismatchRefuses(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	// Override the no-op stub with a strict verifier that always
	// reports a mismatch — simulates the real verifier catching a
	// swapped key.
	shared.VerifyKeyFingerprint = func(body []byte, want string) error {
		return errFakeMismatch
	}
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "nodesource",
		Apt: &config.PkgRepoApt{
			URI:               "https://deb.nodesource.com/node_20.x",
			Suites:            []string{"nodistro"},
			Components:        []string{"main"},
			GPGKeyURL:         "https://attacker.example/key.gpg",
			GPGKeyFingerprint: "9FD3B784BC1C6FC31A8A0A1C1655A0AB68576280",
		},
	}}
	result := executor.NewResult()
	result.Checkable = true
	_, err := Run(newCtx(t, false), step.PkgRepo, result)
	if err == nil {
		t.Fatal("expected fingerprint-mismatch error; got nil")
	}
	if !strings.Contains(err.Error(), "fake mismatch") {
		t.Errorf("error should propagate verifier message; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "key url:") {
		t.Errorf("error should name the key url; got %q", err.Error())
	}
	// Critically: the keyring must NOT exist on disk.
	keyringPath := filepath.Join(s.keyringsDir, "nodesource.gpg")
	if _, statErr := os.Stat(keyringPath); statErr == nil {
		t.Errorf("keyring file was written despite fingerprint mismatch: %s", keyringPath)
	}
}

var errFakeMismatch = fakeErr("fake mismatch")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }
