package preset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/modules"
)

// ecWithRootDir builds the minimum ExecutionContext runLock reads.
func ecWithRootDir(rootDir string) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{RootDir: rootDir},
		// The module-cache dir a component step would run in. runLock must
		// ignore it: verification is anchored to the consumer's playbook.
		CurrentDir: filepath.Join(os.TempDir(), "module-cache", "somewhere"),
	}
}

func TestRunLock_NoLockfileVerifiesNothing(t *testing.T) {
	if got := runLock(ecWithRootDir(t.TempDir())); got != nil {
		t.Errorf("runLock with no lockfile returned %v, want nil", got)
	}
}

func TestRunLock_EmptyRootDirVerifiesNothing(t *testing.T) {
	if got := runLock(ecWithRootDir("")); got != nil {
		t.Errorf("runLock with an empty RootDir returned %v, want nil", got)
	}
	if got := runLock(nil); got != nil {
		t.Errorf("runLock(nil) returned %v, want nil", got)
	}
}

func TestRunLock_LoadsLockfileFromRootDir(t *testing.T) {
	root := t.TempDir()
	lock := &modules.Lock{}
	lock.Set(modules.LockEntry{Ref: "github.com/o/r@v1.0.0", Hash: "h1:pinned"})
	if err := lock.SaveLock(filepath.Join(root, modules.LockFilename)); err != nil {
		t.Fatal(err)
	}

	got := runLock(ecWithRootDir(root))
	if got == nil {
		t.Fatal("runLock did not find the lockfile next to the playbook")
	}
	e, ok := got.LookupLock("github.com/o/r@v1.0.0")
	if !ok || e.Hash != "h1:pinned" {
		t.Errorf("loaded lock is missing the pin: %+v", e)
	}
}

// A lockfile in an ancestor directory still applies — same upward search as
// the tool lockfile.
func TestRunLock_FindsLockfileInAncestorDir(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "envs", "prod")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	lock := &modules.Lock{}
	lock.Set(modules.LockEntry{Ref: "github.com/o/r@v1.0.0", Hash: "h1:pinned"})
	if err := lock.SaveLock(filepath.Join(root, modules.LockFilename)); err != nil {
		t.Fatal(err)
	}

	if got := runLock(ecWithRootDir(nested)); got == nil {
		t.Error("runLock did not walk up to the ancestor lockfile")
	}
}

// "Lockfile present but unreadable" must not collapse into "no lockfile",
// which would run unverified — the exact thing the lockfile exists to prevent.
func TestRunLock_CorruptLockfileFailsVerificationRatherThanSkipping(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, modules.LockFilename)
	if err := os.WriteFile(path, []byte("modules: [oh: no: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runLock(ecWithRootDir(root))
	if got == nil {
		t.Fatal("a corrupt lockfile was treated as absent")
	}
	err := got.VerifyDir("github.com/o/r@v1.0.0", root)
	if err == nil {
		t.Fatal("a corrupt lockfile verified successfully")
	}
	if !strings.Contains(err.Error(), modules.LockFilename) {
		t.Errorf("error should name the lockfile, got: %v", err)
	}
}

// resolverFor is what the `use:` handler actually calls; the lock must reach
// the resolver it builds, not just runLock.
func TestResolverFor_CarriesTheLock(t *testing.T) {
	root := t.TempDir()
	lock := &modules.Lock{}
	lock.Set(modules.LockEntry{Ref: "github.com/o/r@v1.0.0", Hash: "h1:pinned"})
	if err := lock.SaveLock(filepath.Join(root, modules.LockFilename)); err != nil {
		t.Fatal(err)
	}

	r := resolverFor(ecWithRootDir(root))
	if r.Lock == nil {
		t.Fatal("resolverFor built a resolver with no lock")
	}
	if _, ok := r.Lock.LookupLock("github.com/o/r@v1.0.0"); !ok {
		t.Error("the resolver's lock is missing the pin")
	}
}
