//nolint:revive // package name follows action convention
package os_ssh_key

// F035 regression tests:
//
// 1. user.Lookup error must abort the run BEFORE writing the file —
//    pre-fix the file was written with uid=-1/gid=-1, leaving sshd
//    rejecting "bad ownership" while the step reported success.
//
// 2. Chown EPERM on the file must surface to the caller — pre-fix
//    the chown error was discarded ("best-effort, ignore EPERM so
//    unit tests on user-owned dirs don't fail"), shipping a file
//    owned by the operator instead of the target user.
//
// 3. Pre-existing parent dir at a too-permissive mode (0755) must be
//    tightened to 0700 — pre-fix the chmod-after-MkdirAll branch only
//    ran on first-create, leaving inherited modes alone.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

func TestRun_UserLookupFailureRefusesWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")

	// Use a username guaranteed not to resolve. The leading underscore
	// is rejected by every POSIX user database I know of, and the
	// suffix is random enough that there's no real collision risk.
	missing := "_mooncake_f035_missing_user_xz9k"

	_, err := (&Handler{}).Run(newCtx(t, false), &config.Step{OsSSHKey: &config.OsSSHKey{
		User: missing,
		Key:  key1,
		Path: path,
	}})
	if err == nil {
		t.Fatal("expected Run to reject unknown user; got nil error")
	}
	if !strings.Contains(err.Error(), "cannot determine uid/gid for user "+missing) {
		t.Errorf("error should name the lookup-failure remediation; got %v", err)
	}
	// The file must NOT have been written.
	if _, statErr := os.Stat(path); statErr == nil {
		t.Errorf("file written despite lookup failure: %s", path)
	}
}

// TestRun_ChownEPERMSurfacesAsError — F035 regression guard,
// retired by spec-72 Layer C + the named-user filesystem ownership
// follow-up + the chownFn-retry-path cleanup. The
// ErrBecomeNoSudoPass branch in writeAuthorizedKeys (which the
// original test exercised) was unreachable under the new model:
//
//   - When as_user is set on the step, the Performer routes
//     WriteFile through sudo. Missing SudoPass surfaces as
//     ErrBecomeNoSudoPass from WriteFile before chownFn is ever
//     called, so the retry path that returned the same sentinel
//     never fires.
//   - When as_user is unset, the runner's bound AsUser is "",
//     so its chown retry exec's directly (no sudo wrap). It can
//     return EPERM or success — never ErrBecomeNoSudoPass.
//
// The hint code was deleted in the cleanup commit. The retry-via-
// runner path remains and is exercised whenever chownFn EPERMs;
// when the runner also can't fix the ownership (typical for
// non-current sshUser without as_user:root), the wrapped error
// includes the new "add as_user: root to the step" remediation.
// The structural regression intent ("chown failures aren't silently
// swallowed") is preserved.

func TestRun_TightensExistingSshDirMode(t *testing.T) {
	// Pre-create the parent directory at 0o755 (a too-permissive mode
	// that sshd will refuse). The handler must chmod it to 0o700 on
	// every run, not only on first-create.
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sshDir, "authorized_keys")

	_ = mustRun(t, false, &config.Step{OsSSHKey: &config.OsSSHKey{
		User: currentUsername(t),
		Key:  key1,
		Path: path,
	}})

	info, err := os.Stat(sshDir)
	if err != nil {
		t.Fatalf("stat %s: %v", sshDir, err)
	}
	if got := info.Mode().Perm(); got != sshDirMode {
		t.Errorf("parent dir mode = %o, want %o (pre-fix: pre-existing 0755 stayed at 0755)", got, sshDirMode)
	}
}
