---
id: F035
title: os.ssh_key silently writes authorized_keys with wrong ownership when user.Lookup fails or chown lacks privilege — breaks sshd auth without error
severity: bug
package: internal/actions/os_ssh_key
file: internal/actions/os_ssh_key/handler.go
lines: 182-188, 220-228, 506-539
status: done
verified: 2026-05-16 — confirmed real on master @ 6edf2b0. handler.go:182-188 logged lookup error at debug level only (-1,-1 sentinel propagated so writeAuthorizedKeys skipped chown). handler.go:532-536 ignored chown errors entirely with _ = os.Chown(...). Both paths set Changed=true and reported success; sshd StrictModes refused wrong-owned authorized_keys → silent auth break.
resolved: 2026-05-16 — three paths plugged: (a) `lookupOwnership` runs BEFORE `writeAuthorizedKeys`; if it returns an error, Run returns immediately with `os.ssh_key: cannot determine uid/gid for user <name>: <err> (create the user first, or set `path:`...)`. (b) `chownFn` (package-level var, defaults to `os.Chown`) replaces the silent `_ = os.Chown(...)` on the file path; EPERM surfaces as a wrapped error with a "run with sudo" remediation hint, other errors surface unchanged. Parent-dir chown stays best-effort because the parent is typically pre-owned correctly and `MkdirAll` doesn't re-own. (c) `os.Chmod(parent, sshDirMode)` runs unconditionally after `MkdirAll` (with an EPERM escape for non-root callers managing an existing parent), replacing the `if createParentMode { _ = chmod }` branch that only fired on first-create — a pre-existing 0755 .ssh dir now gets tightened to 0700. Regression tests in `f035_test.go`: `TestRun_UserLookupFailureRefusesWrite` (no file written on lookup error), `TestRun_ChownEPERMSurfacesAsError` (stubs chownFn to EPERM, asserts error + sudo hint + no swallow), `TestRun_TightensExistingSshDirMode` (pre-creates parent at 0755, asserts final mode is 0700).
---

## What

`os.ssh_key` writes authorized_keys for a target user with mode
0600 and (best-effort) chown to the user's uid/gid. Three silent-
failure paths leave the file with **wrong ownership** while
reporting `Changed: true` and step success:

### Path 1 — user.Lookup fails

`lookupOwnership` (line 220-228):

```go
func lookupOwnership(username string) (uid, gid int, err error) {
    u, lookupErr := user.Lookup(username)
    if lookupErr != nil {
        return -1, -1, lookupErr
    }
    uid, _ = strconv.Atoi(u.Uid)
    gid, _ = strconv.Atoi(u.Gid)
    return uid, gid, nil
}
```

Called at line 182:

```go
uid, gid, ownerLookupErr := lookupOwnership(username)
if err := writeAuthorizedKeys(path, plan.lines, uid, gid, !fileExists); err != nil {
    return result, fmt.Errorf("os.ssh_key: write: %w", err)
}
if ownerLookupErr != nil {
    ctx.GetLogger().Debugf("os.ssh_key: ownership lookup skipped: %v", ownerLookupErr)
}
```

`ownerLookupErr` is captured but used only **after** `writeAuthorizedKeys`
has already run, and only at **Debug** log level (suppressed at
default verbosity). The handler proceeds to write the file with
uid=-1, gid=-1.

`writeAuthorizedKeys` (line 532-537):

```go
if uid >= 0 && gid >= 0 {
    _ = os.Chown(path, uid, gid)
    _ = os.Chown(parent, uid, gid)
}
```

When uid/gid are -1, the chown is **completely skipped**. The
file is left owned by the calling process (root if running with
sudo). `sshd` strict-mode checks then refuse to honor the
authorized_keys ("bad ownership"), silently failing the login.

When does user.Lookup fail in practice?
- LDAP/NSS service temporarily unreachable (common in CI bootstrapping)
- Username typo (e.g. `alice` doesn't exist on this host yet)
- Race against another step that creates the user

In every case the operator gets `step ok, file written` and a
broken SSH login.

### Path 2 — Chown silently swallows EPERM

```go
// line 533-535:
// Best-effort chown; ignore EPERM so unit tests on user-owned
// dirs don't fail.
_ = os.Chown(path, uid, gid)
_ = os.Chown(parent, uid, gid)
```

The errors from `os.Chown` are explicitly discarded. The comment
acknowledges this is for test compatibility, but it applies in
**every code path** including production.

A non-root operator running `mooncake apply` to install keys for
user `alice` cannot `chown(alice)`. The chown returns EPERM,
gets dropped, and the file is left with the operator's
ownership.

### Path 3 — fileExists / sshd directory permissions race

Adjacent: `MkdirAll(parent, 0o700)` (line 508) inherits the
existing parent permissions if the dir already exists. The
"Tighten in case MkdirAll inherited a wider mode" branch (line
511-514) only runs **when fileExists is false** — so a pre-
existing parent dir at 0755 stays at 0755. sshd also refuses
0755 .ssh dirs.

## Why it's a bug, not a smell

Reproducible:

```sh
# As root, lookup of nonexistent user fails:
mooncake apply -p - <<EOF
- os.ssh_key:
    user: notauserhere
    key: "ssh-ed25519 AAAA... alice"
EOF
# Today: step reports success, file written somewhere
#   (depends on path resolution falling through), no chown.
# Expected: step fails with "user notauserhere not found".

# As non-root operator, installing alice's key:
sudo -u operator mooncake apply -p - <<EOF
- os.ssh_key:
    user: alice
    key: "ssh-ed25519 AAAA... alice"
EOF
# Today: file is operator-owned. alice's sshd rejects.
# Expected: step fails with "cannot chown authorized_keys: EPERM (use sudo)".
```

## Suggested fix

### (a) Fail-fast on user.Lookup error

Move the lookup BEFORE the write, return error on failure:

```go
uid, gid, ownerLookupErr := lookupOwnership(username)
if ownerLookupErr != nil {
    return result, fmt.Errorf(
        "os.ssh_key: cannot determine uid/gid for user %s: %w "+
        "(create the user first, or set path: to skip uid lookup)",
        username, ownerLookupErr,
    )
}

if err := writeAuthorizedKeys(path, plan.lines, uid, gid, !fileExists); err != nil {
    return result, fmt.Errorf("os.ssh_key: write: %w", err)
}
```

The user-doesn't-exist case becomes a loud failure with a clear
remediation. If the operator deliberately wants to manage keys
for a not-yet-created user, they can set `path:` explicitly to
bypass the lookup.

### (b) Surface chown errors, with test-only escape

Replace the silent EPERM swallow with a check + error:

```go
if uid >= 0 && gid >= 0 {
    if err := os.Chown(path, uid, gid); err != nil {
        if errors.Is(err, fs.ErrPermission) {
            return fmt.Errorf(
                "os.ssh_key: chown %s to uid=%d gid=%d: %w "+
                "(run with sudo to install keys for another user)",
                path, uid, gid, err,
            )
        }
        return fmt.Errorf("os.ssh_key: chown %s: %w", path, err)
    }
    if err := os.Chown(parent, uid, gid); err != nil && !errors.Is(err, fs.ErrPermission) {
        return fmt.Errorf("os.ssh_key: chown %s: %w", parent, err)
    }
}
```

The "don't fail unit tests" intent is preserved for the parent-dir
chown (less load-bearing), but the file-itself chown becomes
critical. Tests that don't run as root should pass `path:` to a
tempdir; existing tests already do this.

### (c) Tighten parent perms unconditionally

```go
parent := filepath.Dir(path)
if err := os.MkdirAll(parent, sshDirMode); err != nil {
    return fmt.Errorf("mkdir %s: %w", parent, err)
}
// Always tighten — MkdirAll on existing dir is a no-op for mode,
// so this is the only path that fixes pre-existing 0755 .ssh dirs.
if err := os.Chmod(parent, sshDirMode); err != nil {
    // Permission-denied: continue (test escape); other errors fail.
    if !errors.Is(err, fs.ErrPermission) {
        return fmt.Errorf("chmod %s: %w", parent, err)
    }
}
```

## Verification

- Add `TestRun_UserLookupFailureRefusesWrite`: stub
  `lookupOwnership` to return an error, assert Run returns
  an error AND no file is written. Pre-fix the file is written.
- Add `TestRun_ChownFailureSurfaces`: run as a non-root user
  attempting to install keys for `root`, assert the error
  surfaces with the "run with sudo" remediation. Pre-fix the
  step succeeds and the file is operator-owned.
- Add `TestRun_TightensExistingSshDirMode`: pre-create
  `~/.ssh` at 0o755, run os.ssh_key, assert dir is 0o700 after.

## References

- `sshd(8)` strict-mode checks — refuses authorized_keys not
  owned by the user or with too-permissive directory perms.
- The chown semantics referenced (uid=-1 = "don't change") are
  standard POSIX (`chown(2)` man page).
- F016 family (silent failures in agentd) — same pattern: the
  worker handles errors quietly when they should surface.
