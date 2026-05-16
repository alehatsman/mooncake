---
id: F031
title: cmd/fleet readToken accepts `literal:<token>` without --insecure-token-on-cmdline flag and file:<path> without permission check
severity: smell
package: cmd
file: cmd/fleet.go
lines: 220-253
status: done
resolved_by: worktree-fix-f031
---

## What

`readToken` resolves the bearer token from one of three `--token-via`
sources:

```go
func readToken(c *cli.Context, src string) (string, error) {
    switch {
    case src == "stdin":             // prompt + Fscanln, ok
    case strings.HasPrefix(src, "file:"):
        path := strings.TrimPrefix(src, "file:")
        data, err := os.ReadFile(path)         // ← no mode check
        // ...
    case strings.HasPrefix(src, "literal:"):
        tok := strings.TrimSpace(strings.TrimPrefix(src, "literal:"))
        // ← no --insecure-* guard
        // ...
    }
}
```

Two adjacent gaps:

### (a) `literal:` mode is silently insecure

The mooncake CLI elsewhere refuses to take a sudo password on the
command line without an opt-in flag:

```
internal/apply/runner.go (validate):
  return fmt.Errorf("--sudo-pass requires --insecure-sudo-pass flag
    (WARNING: password will be visible in shell history and process list)")
```

(see `password.go:194-196`)

The bearer token has equivalent (arguably worse) blast radius —
it authenticates against the agentd daemon, which runs with the
permissions of the daemon process (often elevated). But
`--token-via literal:foo` accepts the token directly without
any insecure-acknowledgement. Operator habit established by the
sudo path doesn't carry over.

Concrete: `mooncake fleet bootstrap user@host --token-via
literal:abc123` puts "abc123" in:

- shell history (`~/.bash_history`, zsh hist)
- `ps` output for the lifetime of the process
- container / cgroup snapshots when run in CI

### (b) `file:<path>` mode doesn't check perms

`os.ReadFile(path)` accepts any mode. Compare with
`security.FilePasswordProvider`:

```go
// password.go:64-66
mode := info.Mode().Perm()
if mode != 0600 {
    return "", fmt.Errorf("password file must have 0600 permissions, found %04o", mode)
}
```

For tokens — same shape of secret, same risk model — there's no
mode check. A user who runs `mooncake fleet bootstrap ...
--token-via file:/etc/some-shared-config/token` against a
0644 file gets no warning that the token is world-readable.

(F030 separately argues the password-side exact-0600 check is too
strict; both should converge on "owner-only via bitmask").

## Why it's `smell` (not `bug`)

Functionally works. The user-supplied token reaches the daemon as
intended. The risk is around exposure, not correctness:

- A junior operator copies a `literal:` command from a teammate's
  Slack message → that token is now in their shell history.
- An audit log shows `mooncake fleet bootstrap … --token-via
  literal:abc123` in process arguments — the token leaks to
  log-shipping tools.
- A token file at 0644 (e.g. checked-in `secrets.txt` with
  inherited group access) reads without complaint.

None of these compromise the daemon by themselves; they make
later compromise more likely.

## Suggested fix

### (a) Require an explicit `--insecure-token-on-cmdline` flag for `literal:`

Mirror the sudo-pass pattern:

```go
case strings.HasPrefix(src, "literal:"):
    if !c.Bool("insecure-token-on-cmdline") {
        return "", errors.New(
            "--token-via literal:<token> requires --insecure-token-on-cmdline " +
            "(WARNING: token will be visible in shell history, ps output, " +
            "and audit logs). Prefer --token-via stdin or file:<path>.",
        )
    }
    tok := strings.TrimSpace(strings.TrimPrefix(src, "literal:"))
    // ... (rest unchanged)
```

Add the flag to `fleetBootstrapCommand` / `fleetPairCommand` flag
sets.

### (b) Check perms on `file:<path>`

```go
case strings.HasPrefix(src, "file:"):
    path := strings.TrimPrefix(src, "file:")
    info, err := os.Stat(path)
    if err != nil {
        return "", fmt.Errorf("stat token file %s: %w", path, err)
    }
    if info.Mode().Perm()&0o077 != 0 {
        return "", fmt.Errorf(
            "token file %s is group/world-accessible (mode %04o); chmod 600 or stricter",
            path, info.Mode().Perm(),
        )
    }
    data, err := os.ReadFile(path)
    // ... (rest unchanged)
```

Factor this with F030's fix to `FilePasswordProvider`: both are
"owner-only file, contents are a secret" — extract a shared
helper into `internal/security`:

```go
// internal/security/file_secret.go
func ReadOwnerOnlyFile(path string) ([]byte, error) {
    info, err := os.Stat(path)
    if err != nil { return nil, err }
    if info.Mode().Perm()&0o077 != 0 {
        return nil, fmt.Errorf("...")
    }
    return os.ReadFile(path)
}
```

Both `readToken` and `FilePasswordProvider` consume that.

## Verification

- After (a): `mooncake fleet bootstrap user@host --token-via
  literal:abc` errors with the WARNING message. Same with
  `--insecure-token-on-cmdline` succeeds.
- After (b): `chmod 644 token.txt; mooncake fleet bootstrap ...
  --token-via file:token.txt` errors with the chmod hint;
  `chmod 600 token.txt` succeeds.
- Manual: `mooncake fleet bootstrap --help` lists the new flag.

## References

- F030 — adjacent file-perm check on sudo password files;
  same shape, both should converge on owner-only-via-bitmask.
- `internal/apply/runner.go:296-298` — the sudo-pass insecure
  guard pattern this finding proposes mirroring.
- `internal/security/password.go` — the spot to factor the
  shared `ReadOwnerOnlyFile` helper.
