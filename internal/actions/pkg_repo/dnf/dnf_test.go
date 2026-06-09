package dnf

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

// stubFS overrides the dnf paths + key fetcher + cache cleaner for
// the duration of a test. Mirrors the apt stubFS shape.
type stubFS struct {
	reposDir    string
	keyringDir  string
	keyBody     []byte
	cacheCalled int
}

func newStubFS(t *testing.T) *stubFS {
	t.Helper()
	dir := t.TempDir()
	s := &stubFS{
		reposDir:   filepath.Join(dir, "yum.repos.d"),
		keyringDir: filepath.Join(dir, "rpm-gpg"),
		keyBody:    []byte("-----BEGIN PGP PUBLIC KEY-----\nfake\n-----END PGP PUBLIC KEY-----\n"),
	}
	originalPaths := paths
	originalFetch := shared.HTTPFetchKey
	originalClean := cleanCache
	originalVerify := shared.VerifyKeyFingerprint
	paths = Paths{ReposDir: s.reposDir, KeyringDir: s.keyringDir}
	shared.HTTPFetchKey = func(context.Context, string) ([]byte, error) { return s.keyBody, nil }
	shared.VerifyKeyFingerprint = func([]byte, string) error { return nil }
	cleanCache = func(_ context.Context, _ *security.Privileged) error {
		s.cacheCalled++
		return nil
	}
	t.Cleanup(func() {
		paths = originalPaths
		shared.HTTPFetchKey = originalFetch
		cleanCache = originalClean
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

func TestDnf_Apply_CreatesRepoAndKeyringAndCleansCache(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "docker-ce",
		Dnf: &config.PkgRepoDnf{
			BaseURL:           "https://download.docker.com/linux/rhel/9/x86_64/stable",
			Description:       "Docker CE Stable - x86_64",
			GPGKeyURL:         "https://download.docker.com/linux/rhel/gpg",
			GPGKeyFingerprint: "9DC858229FC7DD38854AE2D88D81803C0EBFCD88",
		},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatal("expected change on create")
	}
	repoPath := filepath.Join(s.reposDir, "docker-ce.repo")
	keyringPath := filepath.Join(s.keyringDir, "RPM-GPG-KEY-docker-ce")
	got, err := os.ReadFile(repoPath)
	if err != nil {
		t.Fatalf("read repo file: %v", err)
	}
	want := strings.Join([]string{
		"# Managed by mooncake pkg.repo. Do not edit by hand.",
		"[docker-ce]",
		"name=Docker CE Stable - x86_64",
		"baseurl=https://download.docker.com/linux/rhel/9/x86_64/stable",
		"enabled=1",
		"gpgcheck=1",
		"gpgkey=file://" + keyringPath,
		"",
	}, "\n")
	if string(got) != want {
		t.Errorf("repo content mismatch\n got %q\nwant %q", got, want)
	}
	if _, err := os.Stat(keyringPath); err != nil {
		t.Errorf("keyring not written: %v", err)
	}
	if s.cacheCalled != 1 {
		t.Errorf("expected dnf clean expire-cache once, got %d", s.cacheCalled)
	}
}

func TestDnf_Apply_DescriptionDefaultsToName(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	boolFalse := false
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "epel",
		Dnf: &config.PkgRepoDnf{
			BaseURL:  "https://download.example.com/epel/9/x86_64/",
			GPGCheck: &boolFalse,
		},
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(filepath.Join(s.reposDir, "epel.repo"))
	if !strings.Contains(string(got), "name=epel\n") {
		t.Errorf("description should default to repo name; got %q", got)
	}
}

func TestDnf_Apply_NoKeyringWhenGPGCheckOff(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	boolFalse := false
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "local",
		Dnf: &config.PkgRepoDnf{
			BaseURL:  "http://localhost:8080/rpms/",
			GPGCheck: &boolFalse,
		},
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(filepath.Join(s.reposDir, "local.repo"))
	if strings.Contains(string(got), "gpgkey=") {
		t.Errorf("should not emit gpgkey= without GPGKeyURL; got %q", got)
	}
	if !strings.Contains(string(got), "gpgcheck=0") {
		t.Errorf("expected gpgcheck=0; got %q", got)
	}
	entries, _ := os.ReadDir(s.keyringDir)
	if len(entries) != 0 {
		t.Errorf("keyring dir should be untouched; got %d entries", len(entries))
	}
}

func TestDnf_Apply_MetalinkAndEnabledFalse(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	boolFalse := false
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "fedora-debuginfo",
		Dnf: &config.PkgRepoDnf{
			Metalink: "https://mirrors.fedoraproject.org/metalink?repo=fedora-debug-$releasever&arch=$basearch",
			Enabled:  &boolFalse,
			GPGCheck: &boolFalse,
		},
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(filepath.Join(s.reposDir, "fedora-debuginfo.repo"))
	if !strings.Contains(string(got), "metalink=https://mirrors.fedoraproject.org/") {
		t.Errorf("metalink not rendered; got %q", got)
	}
	if !strings.Contains(string(got), "enabled=0\n") {
		t.Errorf("enabled=0 not rendered; got %q", got)
	}
	if strings.Contains(string(got), "baseurl=") {
		t.Errorf("should not emit baseurl= when only metalink is set; got %q", got)
	}
}

func TestDnf_Apply_Idempotent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "docker-ce",
		Dnf: &config.PkgRepoDnf{
			BaseURL:           "https://download.docker.com/linux/rhel/9/x86_64/stable",
			GPGKeyURL:         "https://download.docker.com/linux/rhel/gpg",
			GPGKeyFingerprint: "ABCD",
		},
	}}
	_ = mustRun(t, false, step)
	first := s.cacheCalled
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("second run should be no-op; reason=%q", r.Reason)
	}
	if s.cacheCalled != first {
		t.Errorf("dnf clean must not be called when nothing changed (was %d, now %d)", first, s.cacheCalled)
	}
}

func TestDnf_Apply_DetectsDrift(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "docker-ce",
		Dnf: &config.PkgRepoDnf{
			BaseURL:           "https://download.docker.com/linux/rhel/8/x86_64/stable",
			GPGKeyURL:         "https://k",
			GPGKeyFingerprint: "ABCD",
		},
	}}
	_ = mustRun(t, false, step)

	step.PkgRepo.Dnf.BaseURL = "https://download.docker.com/linux/rhel/9/x86_64/stable"
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatal("expected drift to trigger change")
	}
	got, _ := os.ReadFile(filepath.Join(s.reposDir, "docker-ce.repo"))
	if !strings.Contains(string(got), "/rhel/9/") {
		t.Errorf("baseurl not updated to RHEL 9; got %q", got)
	}
}

func TestDnf_Absent_RemovesRepo(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "old",
		Dnf: &config.PkgRepoDnf{
			BaseURL:           "https://example.com/rpms/",
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
	if _, err := os.Stat(filepath.Join(s.reposDir, "old.repo")); !os.IsNotExist(err) {
		t.Errorf("repo file should be removed; stat err=%v", err)
	}
}

func TestDnf_Absent_NoopWhenMissing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name:  "never",
		State: "absent",
		Dnf:   &config.PkgRepoDnf{},
	}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("absent on missing repo should be no-op; reason=%q", r.Reason)
	}
}

func TestDnf_Plan_DoesNotWrite(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "docker-ce",
		Dnf: &config.PkgRepoDnf{
			BaseURL:           "https://download.docker.com/linux/rhel/9/x86_64/stable",
			GPGKeyURL:         "https://k",
			GPGKeyFingerprint: "ABCD",
		},
	}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange; reason=%q", r.Reason)
	}
	if _, err := os.Stat(filepath.Join(s.reposDir, "docker-ce.repo")); err == nil {
		t.Error("plan must not write repo file")
	}
	if s.cacheCalled != 0 {
		t.Errorf("plan must not run dnf clean; called %d times", s.cacheCalled)
	}
}

// TestDnf_FingerprintMismatchRefuses — mirrors F034 on the apt side.
func TestDnf_FingerprintMismatchRefuses(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	shared.VerifyKeyFingerprint = func(body []byte, want string) error {
		return errors.New("fake mismatch")
	}
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "docker-ce",
		Dnf: &config.PkgRepoDnf{
			BaseURL:           "https://download.docker.com/linux/rhel/9/x86_64/stable",
			GPGKeyURL:         "https://attacker.example/key.gpg",
			GPGKeyFingerprint: "9DC858229FC7DD38854AE2D88D81803C0EBFCD88",
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
	keyringPath := filepath.Join(s.keyringDir, "RPM-GPG-KEY-docker-ce")
	if _, statErr := os.Stat(keyringPath); statErr == nil {
		t.Errorf("keyring file was written despite fingerprint mismatch: %s", keyringPath)
	}
}

func TestDnf_Reverse_CapturedOnUpdate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	if err := os.MkdirAll(s.reposDir, 0o755); err != nil {
		t.Fatal(err)
	}
	priorRepo := filepath.Join(s.reposDir, "docker-ce.repo")
	if err := os.WriteFile(priorRepo, []byte("# prior content\n[docker-ce]\nname=old\nbaseurl=https://old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "docker-ce",
		Dnf: &config.PkgRepoDnf{
			BaseURL:           "https://download.docker.com/linux/rhel/9/x86_64/stable",
			GPGKeyURL:         "https://k",
			GPGKeyFingerprint: "ABCD",
		},
	}}
	res := mustRun(t, false, step)
	if res.ReverseData == nil {
		t.Fatal("expected ReverseData captured on update")
	}
	info, ok := res.ReverseData.(*shared.PkgRepoReverseInfo)
	if !ok {
		t.Fatalf("ReverseData type %T, want *shared.PkgRepoReverseInfo", res.ReverseData)
	}
	if !info.PriorExisted {
		t.Error("PriorExisted should be true when the repo file was on disk pre-apply")
	}
	if !strings.Contains(info.PriorContent, "baseurl=https://old") {
		t.Errorf("PriorContent does not match pre-apply state; got %q", info.PriorContent)
	}
}
