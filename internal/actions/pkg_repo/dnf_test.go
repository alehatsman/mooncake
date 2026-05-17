//nolint:revive // package name follows action convention
package pkg_repo

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// stubDnfFS overrides the dnf paths + key fetcher + cache cleaner for
// the duration of a test. Mirrors stubFS for the apt driver; sharing
// state lets one stub serve a mixed-driver test if a future scenario
// needs it. The fake key body still can't be parsed by the real
// openpgp verifier, so the default verifyKeyFingerprint stub is a
// no-op (same convention as the apt happy-path tests).
type stubDnfFS struct {
	reposDir    string
	keyringDir  string
	keyBody     []byte
	cacheCalled int
	resetVerify func()
	originalDnf dnfPaths
	originalFK  func(string) ([]byte, error)
	originalCC  func() error
	originalVKF func([]byte, string) error
}

func newStubDnfFS(t *testing.T) *stubDnfFS {
	t.Helper()
	dir := t.TempDir()
	s := &stubDnfFS{
		reposDir:    filepath.Join(dir, "yum.repos.d"),
		keyringDir:  filepath.Join(dir, "rpm-gpg"),
		keyBody:     []byte("-----BEGIN PGP PUBLIC KEY-----\nfake\n-----END PGP PUBLIC KEY-----\n"),
		originalDnf: dnf,
		originalFK:  fetchKey,
		originalCC:  dnfCleanCache,
		originalVKF: verifyKeyFingerprint,
	}
	dnf = dnfPaths{reposDir: s.reposDir, keyringDir: s.keyringDir}
	fetchKey = func(string) ([]byte, error) { return s.keyBody, nil }
	verifyKeyFingerprint = func([]byte, string) error { return nil }
	dnfCleanCache = func() error {
		s.cacheCalled++
		return nil
	}
	t.Cleanup(func() {
		dnf = s.originalDnf
		fetchKey = s.originalFK
		dnfCleanCache = s.originalCC
		verifyKeyFingerprint = s.originalVKF
	})
	return s
}

func TestDnf_Validate(t *testing.T) {
	boolp := func(b bool) *bool { return &b }
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			"dnf no baseurl/metalink/mirrorlist",
			&config.Step{PkgRepo: &config.PkgRepo{Name: "x", Dnf: &config.PkgRepoDnf{}}},
			true,
		},
		{
			"dnf baseurl + metalink mutually exclusive",
			&config.Step{PkgRepo: &config.PkgRepo{Name: "x", Dnf: &config.PkgRepoDnf{
				BaseURL:  "https://example.com",
				Metalink: "https://example.com/metalink",
			}}},
			true,
		},
		{
			"dnf gpg check default needs fingerprint",
			&config.Step{PkgRepo: &config.PkgRepo{Name: "x", Dnf: &config.PkgRepoDnf{
				BaseURL:   "https://example.com",
				GPGKeyURL: "https://example.com/key",
			}}},
			true,
		},
		{
			"dnf gpg check off ok without fingerprint",
			&config.Step{PkgRepo: &config.PkgRepo{Name: "x", Dnf: &config.PkgRepoDnf{
				BaseURL:   "https://example.com",
				GPGKeyURL: "https://example.com/key",
				GPGCheck:  boolp(false),
			}}},
			false,
		},
		{
			"dnf ok baseurl only",
			&config.Step{PkgRepo: &config.PkgRepo{Name: "epel", Dnf: &config.PkgRepoDnf{
				BaseURL: "https://download.example.com/epel/9/Everything/x86_64/",
			}}},
			false,
		},
		{
			"dnf ok metalink only",
			&config.Step{PkgRepo: &config.PkgRepo{Name: "fedora", Dnf: &config.PkgRepoDnf{
				Metalink: "https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever&arch=$basearch",
			}}},
			false,
		},
		{
			"dnf ok absent skips source-required check",
			&config.Step{PkgRepo: &config.PkgRepo{Name: "old", State: "absent", Dnf: &config.PkgRepoDnf{}}},
			false,
		},
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

func TestDnf_Apply_CreatesRepoAndKeyringAndCleansCache(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubDnfFS(t)
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
	s := newStubDnfFS(t)
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
	s := newStubDnfFS(t)
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
	s := newStubDnfFS(t)
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
	s := newStubDnfFS(t)
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
	s := newStubDnfFS(t)
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "docker-ce",
		Dnf: &config.PkgRepoDnf{
			BaseURL:           "https://download.docker.com/linux/rhel/8/x86_64/stable",
			GPGKeyURL:         "https://k",
			GPGKeyFingerprint: "ABCD",
		},
	}}
	_ = mustRun(t, false, step)

	// Bump major version 8 → 9.
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
	s := newStubDnfFS(t)
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
	_ = newStubDnfFS(t)
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
	s := newStubDnfFS(t)
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
// Operator pins a fingerprint, the fetched key carries a different
// one, and the handler must refuse before the keyring lands on disk.
func TestDnf_FingerprintMismatchRefuses(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubDnfFS(t)
	verifyKeyFingerprint = func(body []byte, want string) error {
		return errFakeMismatch
	}
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "docker-ce",
		Dnf: &config.PkgRepoDnf{
			BaseURL:           "https://download.docker.com/linux/rhel/9/x86_64/stable",
			GPGKeyURL:         "https://attacker.example/key.gpg",
			GPGKeyFingerprint: "9DC858229FC7DD38854AE2D88D81803C0EBFCD88",
		},
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
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

func TestDnf_Permissions(t *testing.T) {
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "docker-ce",
		Dnf: &config.PkgRepoDnf{
			BaseURL:           "https://download.docker.com/linux/rhel/9/x86_64/stable",
			GPGKeyURL:         "https://download.docker.com/linux/rhel/gpg",
			GPGKeyFingerprint: "ABCD",
		},
	}}
	ps := Handler{}.Permissions(step)
	if !ps.Sudo {
		t.Error("dnf driver should require sudo")
	}
	if !ps.Network {
		t.Error("dnf driver should advertise network when gpg_key_url is set")
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "dnf" {
		t.Errorf("expected RequiredBinaries=[dnf]; got %v", ps.RequiredBinaries)
	}
	wantRepo := dnf.reposDir + "/docker-ce.repo"
	wantKey := dnf.keyringDir + "/RPM-GPG-KEY-docker-ce"
	gotPaths := strings.Join(ps.FilesystemWrite, ",")
	if !strings.Contains(gotPaths, wantRepo) || !strings.Contains(gotPaths, wantKey) {
		t.Errorf("FilesystemWrite missing dnf paths; got %v", ps.FilesystemWrite)
	}
}

func TestDnf_Diff_DriverDnf(t *testing.T) {
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "epel",
		Dnf:  &config.PkgRepoDnf{BaseURL: "https://example.com/epel"},
	}}
	d, err := Handler{}.Diff(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	snap, ok := d.After.(*PkgRepoSnapshot)
	if !ok {
		t.Fatalf("After is %T, want *PkgRepoSnapshot", d.After)
	}
	if snap.Driver != "dnf" {
		t.Errorf("Driver=%q, want dnf", snap.Driver)
	}
	if snap.Name != "epel" {
		t.Errorf("Name=%q, want epel", snap.Name)
	}
}

func TestDnf_Reverse_RestoresPriorContent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubDnfFS(t)
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
	info, ok := res.ReverseData.(*PkgRepoReverseInfo)
	if !ok {
		t.Fatalf("ReverseData type %T, want *PkgRepoReverseInfo", res.ReverseData)
	}
	if !info.PriorExisted {
		t.Error("PriorExisted should be true when the repo file was on disk pre-apply")
	}
	if !strings.Contains(info.PriorContent, "baseurl=https://old") {
		t.Errorf("PriorContent does not match pre-apply state; got %q", info.PriorContent)
	}

	reverseStep, err := Handler{}.Reverse(newCtx(t, false), step, res)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep == nil || reverseStep.FileWrite == nil {
		t.Fatalf("Reverse should produce a file.write step; got %+v", reverseStep)
	}
	if reverseStep.FileWrite.Path != priorRepo {
		t.Errorf("Reverse path %q, want %q", reverseStep.FileWrite.Path, priorRepo)
	}
	if reverseStep.FileWrite.State != "file" {
		t.Errorf("Reverse state %q, want file", reverseStep.FileWrite.State)
	}
	if !strings.Contains(reverseStep.FileWrite.Content, "baseurl=https://old") {
		t.Errorf("Reverse content does not restore prior bytes; got %q", reverseStep.FileWrite.Content)
	}
}
