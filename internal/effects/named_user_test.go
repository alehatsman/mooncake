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
		t.Fatalf("user.Current: %v", err)
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

	wantSuffix := "&& chown " + wantSpec + " " + shellQuote("/etc/foo")
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

// TestWithChown_RootAsUser_NoChown — the "root" / "0" AsUser doesn't
// need an explicit chown (sudo's already operating as root → file
// lands owned by root).
func TestWithChown_RootAsUser_NoChown(t *testing.T) {
	for _, target := range []string{"root", "0"} {
		t.Run(target, func(t *testing.T) {
			p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, target).(*defaultPerformer)
			base := "mv /tmp/x /etc/foo && chmod 0644 /etc/foo"
			if got := p.withChown(base, "/etc/foo"); got != base {
				t.Errorf("withChown added chown for AsUser=%q (should be no-op):\n  got: %s", target, got)
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
