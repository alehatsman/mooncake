package effects

import (
	"os/user"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

// currentUserName returns the name of the user mooncake's tests run
// as. Used to drive the named-user chown path without relying on
// /etc/passwd containing a specific test fixture user.
func currentUserName(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		// Minimal CI containers run as a uid with no /etc/passwd row and
		// no $USER; the current user is genuinely unresolvable there. These
		// named-user chown tests require a resolvable operator, so skip
		// rather than fail (they still run on dev hosts).
		t.Skipf("current user unresolvable (hermetic CI): %v", err)
	}
	return u.Username
}

// TestWithChown_NamedUser_AppendsChownClause — spec-72 Layer C:
// when the Performer is bound to a named non-root AsUser, every
// sudo-create path must chain a chown so the resulting file lands
// owned by the operator's declared target — not by root.
func TestWithChown_NamedUser_AppendsChownClause(t *testing.T) {
	username := currentUserName(t)
	u, err := user.Lookup(username)
	if err != nil {
		t.Fatalf("user.Lookup(%q): %v", username, err)
	}
	wantSpec := u.Uid + ":" + u.Gid

	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, username).(*defaultPerformer)
	got := p.withChown("mv /tmp/x /etc/foo && chmod 0644 /etc/foo", "/etc/foo")

	wantSuffix := "&& chown " + wantSpec + " " + ShellQuote("/etc/foo")
	if !strings.Contains(got, wantSuffix) {
		t.Errorf("withChown output missing chown clause for %q:\n  got:  %s\n  want: contains %q", username, got, wantSuffix)
	}
}

// TestWithChown_EmptyAsUser_PassesThrough — no AsUser bound, no
// chown should be added.
func TestWithChown_EmptyAsUser_PassesThrough(t *testing.T) {
	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, "").(*defaultPerformer)
	base := "mv /tmp/x /etc/foo && chmod 0644 /etc/foo"
	if got := p.withChown(base, "/etc/foo"); got != base {
		t.Errorf("withChown changed the command despite empty AsUser:\n  got:  %s\n  want: %s", got, base)
	}
}

// TestWithChown_RootAsUser_Chown — issue #168: the "root" / "0"
// AsUser still needs an explicit chown 0:0. Mkdir/Touch create the
// path directly under sudo so this is a no-op for them, but
// WriteFile/CopyFile stage content in a tempfile owned by the
// unprivileged process and `mv` it into place under sudo — a
// same-filesystem `mv` is a rename(2), which preserves that
// pre-sudo owner instead of resetting it to root. Without an
// explicit chown here, those two primitives silently land the file
// owned by the invoking user, not root.
func TestWithChown_RootAsUser_Chown(t *testing.T) {
	for _, target := range []string{"root", "0"} {
		t.Run(target, func(t *testing.T) {
			p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, target).(*defaultPerformer)
			base := "mv /tmp/x /etc/foo && chmod 0644 /etc/foo"
			want := base + " && chown 0:0 " + ShellQuote("/etc/foo")
			if got := p.withChown(base, "/etc/foo"); got != want {
				t.Errorf("withChown for AsUser=%q:\n  got:  %s\n  want: %s", target, got, want)
			}
		})
	}
}

// TestChownSpec_ResolutionFailure_EmptySpec — if user.Lookup fails
// for the bound AsUser, chownSpec returns "" so the sudo command
// still completes; the file lands owned by root and the operator
// will see the mis-ownership and can fix the user resolution. We
// prefer this to failing the apply.
func TestChownSpec_ResolutionFailure_EmptySpec(t *testing.T) {
	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, "nonexistent-user-xyz-12345").(*defaultPerformer)
	if got := p.chownSpec(); got != "" {
		t.Errorf("chownSpec for nonexistent user = %q, want empty (resolution-failure fallback)", got)
	}
}

// TestChownSpec_KnownUser_ResolvesToUidGid — sanity check that the
// current user (always resolvable) produces a uid:gid spec.
func TestChownSpec_KnownUser_ResolvesToUidGid(t *testing.T) {
	username := currentUserName(t)
	u, _ := user.Lookup(username)
	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, username).(*defaultPerformer)
	want := u.Uid + ":" + u.Gid
	if got := p.chownSpec(); got != want {
		t.Errorf("chownSpec(%q) = %q, want %q", username, got, want)
	}
}

// TestNeedSudoForOwnership_NoAsUser — empty AsUser means "no
// ownership constraint." Direct path is always fine.
func TestNeedSudoForOwnership_NoAsUser(t *testing.T) {
	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, "").(*defaultPerformer)
	if p.needSudoForOwnership() {
		t.Error("needSudoForOwnership = true for empty AsUser; want false")
	}
}

// TestNeedSudoForOwnership_CurrentUser — AsUser matching the
// current process user means direct write produces correct
// ownership; no sudo needed.
func TestNeedSudoForOwnership_CurrentUser(t *testing.T) {
	username := currentUserName(t)
	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, username).(*defaultPerformer)
	if p.needSudoForOwnership() {
		t.Errorf("needSudoForOwnership = true for AsUser=%q (current user); want false", username)
	}
}

// TestNeedSudoForOwnership_NamedNonCurrent — AsUser matching a
// different named user means direct write would land owned by the
// current uid (wrong); needSudoForOwnership must report true so
// the caller forces the sudo+chown path. This is the regression
// guard for the spec-72 "direct-write doesn't post-chown" caveat
// fix.
func TestNeedSudoForOwnership_NamedNonCurrent(t *testing.T) {
	// "nobody" is a POSIX-conventional account present on every
	// supported test host and very unlikely to match the test
	// runner's current user.
	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, "nobody").(*defaultPerformer)
	if currentUserName(t) == "nobody" {
		t.Skip("test runs as 'nobody'; can't drive the cross-user branch")
	}
	if !p.needSudoForOwnership() {
		t.Error("needSudoForOwnership = false for AsUser=nobody (cross-user); want true")
	}
}

// TestNeedSudoForOwnership_RootWhenNonRoot — AsUser=root from a
// non-root process: direct write would produce a file owned by
// the current (non-root) uid, not by root. Must force sudo.
func TestNeedSudoForOwnership_RootWhenNonRoot(t *testing.T) {
	if currentUserName(t) == "root" {
		t.Skip("test runs as root; can't drive the non-root branch")
	}
	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, "root").(*defaultPerformer)
	if !p.needSudoForOwnership() {
		t.Error("needSudoForOwnership = false for AsUser=root from non-root process; want true")
	}
}
