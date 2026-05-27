---
id: F056
title: plan.SavePlanToFile writes plans at default umask (typically 0644) — secret refs and playbook structure are world-readable on multi-user hosts (F037 family)
severity: smell
package: internal/plan
file: internal/plan/io.go
lines: 49
status: done
fixed: 2026-05-27 — `SavePlanToFile` now writes via `os.OpenFile(tmpPath, O_RDWR|O_CREATE|O_TRUNC, 0o600)` against `<filePath>.tmp`, then `os.Rename`s into place. Two wins in one change: (1) the file mode is 0o600 (owner-read/write only) instead of 0644 under typical umask 022; (2) the write is atomic via temp+rename — a mid-write failure leaves the destination untouched (was: partial/empty file at the destination). Cleanup of the temp file on the error path is deferred so write/close failures don't orphan a `.tmp` next to the destination.
post-fix verified: 2026-05-27 — three new tests in `io_test.go`: `TestF056_SavePlanToFile_Perms0o600` (subtests for both .json and .yaml) confirms the resulting file's `Mode().Perm()` is `0o600`. `TestF056_SavePlanToFile_AtomicCleanup` confirms no `.tmp` orphan after a successful write. `TestLoadPlanFromFile_RejectsUnknownYAMLFields` and `_JSON` cover the adjacent strict-decode change (same PR). Full `mooncake task ci` green.
discovered: 2026-05-27 — cold-read of internal/plan/. `SavePlanToFile` uses bare `os.Create(filePath)` which creates files with mode `0666 & ^umask`. The typical operator umask of `022` resolves to `0644` — world-readable. Plan files carry the playbook structure (every step, all variables, every secret reference). The redaction pass in `redactSecretMarkers` strips secret VALUES but leaves the refs (`!secret env:FOO`) and the full plan shape intact. On a shared host (CI runner, dev box with multiple users, a packaged-product worker that other system services can `read(2)`), the plan file's contents are exposed to anyone who can stat the directory it lives in. F037 closed this exact shape for the pilot agent's saved plans (`internal/pilot/loop.go` now uses `0o600`); the plan-IO save path was never updated to match.
related: F037 (pilot.RunLoop saved plans world-readable — same shape, fixed 2026-05-22), F039 (agentd run-state perms), F031 (`cmd/fleet readToken` perms-check). The "plan artifacts contain sensitive shape, default-umask is too loose" pattern recurs.
---

## What

`SavePlanToFile` (`internal/plan/io.go:26`) marshals a `*Plan` to JSON
or YAML and writes the result via `os.Create`:

```go
// internal/plan/io.go:49
file, err := os.Create(filePath) // #nosec G304 -- filePath is user-provided CLI argument
```

`os.Create` is equivalent to `os.OpenFile(path, O_RDWR|O_CREATE|O_TRUNC, 0o666)`.
The resulting file mode after the standard umask 022 is **0644** —
readable by every user on the host.

The plan's content is sensitive in two distinct ways:

1. **Secret refs.** Spec-23 §3 says refs are non-credential ("they
   point at where the value lives, not the value"). True for refs
   like `env:FOO` or `vault:secret/path` in isolation. But the ref
   set on a host is itself information — knowing the operator's
   playbook calls into `vault:prod/postgres/admin` reveals the
   privilege boundary. The redaction option
   (`MOONCAKE_SHOW_SECRET_REFS=1`) makes refs visible verbatim in
   the file by design; even with the default `!secret` redaction,
   the playbook's variable namespace, step order, action types,
   and resource handles all remain in the open.
2. **Plan as attack surface.** A reader of the plan can infer the
   host's entire intended configuration (every package, service,
   sysctl, ssh_key, etc.). On a shared host this is a recon
   shortcut.

The `#nosec G204` annotation handles the path-traversal concern
(filePath is user-provided), not the permission concern.

## Why "smell" not "risk"

The pre-fix state needs **three** preconditions to materialize as a
real leak:

1. The plan is saved to a path other users on the host can stat.
2. Another user on the host actively looks for plan artifacts.
3. The plan's contents materially help them (privilege escalation
   target, secret-store coordinates, etc.).

Hardened single-tenant environments (a personal dev box, an isolated
agentd container with no shell access) don't hit all three. But the
F037 fix for the pilot path shipped exactly the same scrutiny — the
project's accepted default for plan artifacts is `0o600`. The
`SavePlanToFile` path is the one remaining outlier.

## Proposed fix

Replace the bare `os.Create` with an explicit-mode call:

```go
// internal/plan/io.go
file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
if err != nil {
    return fmt.Errorf("failed to create plan file: %w", err)
}
```

`0o600` matches the F037 convention and is what
`internal/agentd/store.go` already uses for run-state JSON (also
sensitive). The user-side cost is zero — operators who want to share
a plan can `chmod` it explicitly after the fact, the same way they'd
share any other private artifact.

## Same-pattern audit

`grep -rnE 'os\.Create\(' internal/ cmd/ --include="*.go"` outside
`_test.go` returns ~15 sites. Most are auxiliary file creations
that don't carry sensitive content (artifact writers, scaffold
templates, fact-snapshot dumps); a focused audit of which ones
write plans / facts / runlog / secrets-adjacent content would
catch the next instance before it ships. Tracked under TODO.md
cross-cutting themes alongside the F029/F031 secret-perms family.
