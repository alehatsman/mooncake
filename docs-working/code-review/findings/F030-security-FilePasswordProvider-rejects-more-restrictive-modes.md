---
id: F030
title: security.FilePasswordProvider rejects 0400 / 0500 password files — only exact 0600 accepted, more-restrictive modes break
severity: smell
package: internal/security
file: internal/security/password.go
lines: 50-81
status: open
---

## What

`FilePasswordProvider.GetPassword` validates the sudo-password
file mode with an exact-equality check:

```go
mode := info.Mode().Perm()
if mode != 0600 {
    return "", fmt.Errorf("password file must have 0600 permissions, found %04o", mode)
}
```

This refuses every mode except exactly `0600`. In particular:

- **`0400`** (owner read-only) — *more* secure than 0600. Refused.
- **`0500`** (owner read+execute) — odd for a password file but
  doesn't compromise security. Refused.
- **`0640`, `0660`** — actually less secure (group access).
  Correctly refused.
- **`0644`, `0666`** — world-readable. Correctly refused.

The check correctly catches the dangerous cases but ALSO rejects
modes that are stricter than 0600. The error message
("must have 0600 permissions") doesn't help the user understand
that 0400 would have been *better*.

## Why it's a smell (not a bug)

Functionality: works fine for users who chmod 0600 explicitly.
But:

- The `ssh-keygen` / OpenSSH convention is **0400** for private
  keys after first use. A user who treats their sudo-password
  file like an SSH private key (run `chmod 0400 sudo_pass.txt`)
  hits a confusing refusal.
- The user fix is to `chmod 0600` — a *less* restrictive mode.
  That's a counterintuitive UX nudge that pushes users toward
  slightly-worse security.

## Suggested fix

Mask-based check accepts any owner-only mode:

```go
mode := info.Mode().Perm()
// Refuse any mode with group/other access bits. Owner bits are
// fine: 0400/0500/0600/0700 are all owner-only.
if mode&0o077 != 0 {
    return "", fmt.Errorf(
        "password file must not be group/world accessible (got %04o; chmod 600 or stricter)",
        mode,
    )
}
```

The error message now suggests the action ("chmod 600") rather
than the literal-must-be value, so a user with 0440 (group-read
allowed) gets the same correction guidance as one with 0644.

## Adjacent observations

1. **No size cap on `os.ReadFile`** (line 69). A password file
   should be a single line. An accidentally-pointed-at large file
   loads its full content into memory. Minor — defensive readers
   typically `io.LimitReader` here. Not worth fixing alone.

2. **`strings.TrimSpace`** (line 75) trims trailing newline,
   which is the right behavior for `echo "pass" > file`. But it
   also silently swallows a password that's *all whitespace*. A
   user who fat-fingers `chmod 0600 ""` ends up with an empty
   string after trim and gets the "password file is empty"
   error — which is the right error, just for the wrong reason
   if the file truly contained whitespace-only content.

3. **`checkFileOwnership`** (line 58) is platform-specific. On
   Windows it can't verify owner reliably, so the check might
   be looser there. Worth checking `password_windows.go` to
   confirm it doesn't bypass.

None of these are blocking; F030 is just the exact-equality
mode check.

## Verification

- Add `TestFilePasswordProvider_AcceptsModesStricterThan0600`:
  create a file at 0400, write "secret\n", assert GetPassword
  returns "secret" without error.
- `go test ./internal/security/...`
- Manual: `chmod 0400 ~/.mooncake/sudo_pass && mooncake apply
  --sudo-pass-file ~/.mooncake/sudo_pass …` works.

## References

- OpenSSH/sshd policy on private keys (0400 / 0600) for the
  parallel.
- `password_windows.go` for the ownership-check platform split.
