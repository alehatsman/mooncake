# Spec 69: Step Execution Pipeline — Centralize Cross-Cutting Concerns

**Status:** 🟡 Phase 1 in flight (`ctx.Privileged()` primitive + pkg.upgrade
migration). Phases 2-3 (retry + override centralization, broad handler
migration) drafted but not started.
**Epic:** E10 Handler ABI maturation — continuation of spec-22's
ABI work.
**Effort:** M (1–2 weeks across phases 1-3)
**Value:** Eliminates two bug classes at the source instead of paying
per-handler. Unblocks the dotfiles "shell-free" migration that surfaced
the gap.

---

## Problem

Three cross-cutting concerns currently live **inside** each action
handler, with no central enforcement that any handler implements them
correctly. Audit on 2026-05-17:

### 1. Privilege escalation (sudo)

14 handlers declare `Sudo: true` in their `Permissions()`. Of those:

- **4 escalate correctly** — `os.systemd` (post-today's fix), `package`,
  `service`, and `file` use one of three different conventions:
  `security.BecomeRunner`, sibling-package helpers, or
  `ctx.Effects().WriteFile`'s `PerformerOpts.Become` flag.
- **7+ silently skip escalation** — `pkg.upgrade`, `pkg.repo`,
  `pkg.hold`, `os.firewall`, `os.sysctl`, `os.mount`, `os.cron`,
  `os.group`. Bare `exec.Command("apt-get", "upgrade")` /
  `os.WriteFile("/etc/cron.d/foo", ...)`. The action plans, the
  executor sees `as_user: root` and asks for the sudo password, but
  the handler's actual write fails with EACCES because nothing ever
  wrapped the syscall in `sudo`.

The os.systemd fix that shipped today (`fix(os.systemd): escalate via
BecomeRunner instead of assuming root`) is the second instance of this
pattern in two days. There's no reason to expect the count to stop at
two.

### 2. Retry loops

Every step in the schema can declare:

```yaml
retry:
  attempts: 3
  delay: 10s
  backoff: linear
```

But only **4 of ~22 retry-meaningful actions** actually implement a
retry loop in their handler: `shell`, `command`, `http_request`,
`download`. The other 18 — including `pkg`, `git.clone`,
`file.template`, `file.copy`, `pkg.upgrade`, every `os.*` — silently
ignore the field. A user writing `retry:` on `git.clone` (a textbook
"network op, flaky, please retry" call) gets zero retries.

Even the four that implement retry don't agree:

- `shell/handler.go:executeWithRetry` and
  `command/handler.go:executeWithRetry` are near-duplicate ~60-line
  functions. **MT-48** (May 15) had to fix the same bug in both — a
  `retry × failed_when:false` short-circuit interaction.
- **MT-62** (same week) added `linear`/`exponential` backoff support…
  to `shell/handler.go` only. `command`, `download`, and
  `http_request` still slept for the bare delay. The backoff field
  was effectively a partial-rollout silent footgun.

### 3. `failed_when` / `changed_when` overrides

Same shape: `shell` and `command` implement them; everyone else
ignores them. Most users don't trip on this because the field is rare,
but when MT-48 showed it interacts with retry, the combinatorial cost
of "concern × concern × handler" became visible.

### What these three have in common

All three are **policy applied around the work**, not the work
itself. Today each handler must implement all three, in agreement,
with no shared primitive. The audit shows what actually happens:

- Handlers ship without the policy (most retry-able actions never got
  a retry loop).
- Bug fixes have to be applied N times and miss some (MT-62).
- New cross-cutting concerns inherit the disease (sudo joined the
  party two days ago).

There's no test or lint that catches a handler that declared one
policy but didn't implement it. The disease is invisible until a
production apply fails — like today's `pkg.upgrade` EACCES.

---

## Goals

- **G1** Centralize privilege escalation: one primitive
  (`ctx.Privileged()`) that handlers call when they need root, with no
  per-handler escalation boilerplate.
- **G2** Centralize retry: executor wraps `handler.Run` in a retry
  loop driven by `step.Retry`. Handler implementations contain no
  retry logic.
- **G3** Centralize `failed_when` / `changed_when` overrides: executor
  applies them after the (final) handler invocation.
- **G4** Plan-time enforcement: a step whose action declares
  `Sudo: true` and runs as non-root with no sudo password fails at
  *plan*, not at *apply*. Matches existing `RequiredBinaries`
  preflight from spec-22.
- **G5** Migration plan that is incremental and non-breaking. Existing
  handlers keep working; they're moved to the new primitives in waves.
- **G6** Tests at the executor level that prove the policies fire for
  ANY action, including a "dumb test action" that does nothing but be
  retried and escalated.

**Out of scope (separate specs / follow-ups):**

- Generalized "before/after hooks" beyond the three concerns above.
- Async / parallel step execution.
- Reverse/rollback interaction with retry (spec 22 territory; the
  retry centralization respects existing reverse semantics).

---

## Design

### Phase 1: Privilege primitive

**New on `actions.Context`:**

```go
// Privileged returns a runner for shelling out to commands or
// performing file ops that need root (or `as_user: <name>`).
//
// The runner reads ec.Svc.SudoPass and step.AsUser internally and
// chooses the right wrapping:
//   - already running as the target user → bare exec
//   - target is root and SudoPass set    → sudo -S
//   - target is root, no SudoPass, not   → ErrBecomeNoSudoPass
//     already root
//   - non-Linux platforms with become     → ErrBecomeUnsupported
//     unsupported
//
// All callers go through this so the "I declared Sudo:true but
// forgot to actually escalate" bug class becomes structurally
// impossible.
Privileged() PrivilegedRunner
```

```go
type PrivilegedRunner interface {
    // Run executes program+args with the resolved privilege level.
    // Output/error semantics match exec.Cmd.CombinedOutput.
    Run(ctx context.Context, program string, args ...string) ([]byte, error)

    // RunWithInput is the stdin variant for commands like
    // `apt-get install -y` that occasionally consume stdin.
    RunWithInput(ctx context.Context, stdin []byte, program string, args ...string) ([]byte, error)
}
```

Internally `Privileged()` is a thin facade over `security.BecomeRunner`
that already exists. The primitive's value is not new mechanism — it's
that **handlers no longer construct BecomeRunner themselves**, so they
can't forget.

`ctx.Effects()` (the Performer for file ops) already exists and
already respects `PerformerOpts.Become`. The privileged-runner is the
analogous primitive for command execution.

### Phase 2: Retry centralization

**Executor step pipeline (pseudo-code):**

```go
func (e *Executor) executeStep(ctx Context, step *config.Step) Result {
    if !evalWhen(step.When, ctx)              { return skipped("when=false") }
    if !matchTags(step.Tags, e.activeTags)    { return skipped("tag filter") }

    // spec-22 preflight already handles RequiredBinaries.
    // Add: Sudo:true + non-root + no SudoPass → fail.
    if err := preflightPermissions(step, ctx); err != nil {
        return failPlan(err)
    }

    handler := registry.Lookup(step)
    maxAttempts := step.RetryAttempts() + 1

    var result Result
    var err error
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        result, err = handler.Run(ctx, step)
        if err == nil                         { break }
        if !isRetryable(handler, err, step)   { break }
        if attempt < maxAttempts {
            sleep(scaleBackoff(step.RetryDelay(), step.RetryBackoff(), attempt))
        }
    }

    // Overrides run once, on the final outcome — preserves MT-48.
    result, err = applyOverrides(step.FailedWhen, step.ChangedWhen, result, err, ctx)

    if step.As != ""                          { ctx.Register(step.As, result) }
    emitEvents(result)
    return result
}
```

**Optional handler-level retry policy hook:**

Some handlers want a non-default "is this error retryable?" rule —
notably `http_request`, which retries on 5xx/429/timeout but not on
4xx. Add an optional interface:

```go
type RetryDecider interface {
    Retryable(err error, step *config.Step) bool
}
```

Handlers that implement it override the default ("any non-nil error is
retryable when retry is configured"). `http_request` migrates by
exposing its existing `retry_on` policy through this interface.

### Phase 3: Override centralization

The executor pipeline above already handles `failed_when` /
`changed_when` after the retry loop. Handlers stop calling
`finishResult` / `applyOverrides` themselves.

The subtle interaction MT-48 documented (retry must operate on the
raw outcome, NOT the post-failed_when verdict) is preserved by design:
overrides run once after the retry loop, not inside it.

### Phase 4: Plan-time enforcement

Add to executor's preflight (spec-22 already preflights
`RequiredBinaries`):

```go
if perm.Sudo && !alreadyRoot() {
    if sudoPass == "" && step.AsUser != currentUser {
        return fmt.Errorf("step %q requires root but no sudo password is configured (use --sudo-pass / --sudo-pass-file / --ask-become-pass)", step.Name)
    }
}
```

This makes the "permission denied" failures we hit today appear at
*plan* time with an actionable message, instead of at apply time with
an opaque syscall error.

### Phase 5: Handler migrations

Two kinds of migrations, both mechanical:

**Migration A — privilege:** for every handler that calls
`exec.Command(...)` for an operation that needs root, replace with
`ctx.Privileged().Run(...)`. For every `os.WriteFile` to a privileged
path, replace with `ctx.Effects().WriteFile(...,
PerformerOpts{Become: step.ShouldBecome()})`.

Targets (priority order): `pkg.upgrade`, `pkg.repo`, `pkg.hold`,
`os.firewall`, `os.sysctl`, `os.mount`, `os.cron`, `os.group`,
`os.ssh_key`. (`os.systemd`'s recent fix can be cleaned up in the
same pass to use the new primitive — current implementation still
goes through BecomeRunner directly.)

**Migration B — retry/overrides:** for `shell`, `command`,
`http_request`, `download` — delete the handler-internal retry loop +
`applyOverrides` / `finishResult` calls. Wire `http_request` through
the new `RetryDecider` interface for its 5xx/429-only policy. Verify
existing tests (especially MT-48's `TestRetry_TriggeredEvenWhenFailedWhenIsFalse`)
pass against the executor-level retry.

### What stays the same

- The Handler.Run signature is unchanged. Handlers stay testable in
  isolation.
- Existing tests for spec-22 (Diff / Reverse / Cost / Permissions)
  don't move — those are pre/post around `Run`, this spec just adds
  more pre/post.
- `security.BecomeRunner` keeps existing — `Privileged()` wraps it.
- `ctx.Effects()` keeps existing — `Privileged()` is its sibling for
  command execution.

---

## Migration order + rollback

1. **Phase 1**: Add `Privileged()` primitive, plumb through `ctx`.
   Migrate `pkg.upgrade` (the bug that prompted this spec). Validate
   against dotfiles `--tags wsl` apply. **No behavior change for any
   other handler.**
2. **Phase 4**: Plan-time `Sudo:true` preflight check. Adds a new
   error class but only catches bugs that were silently failing at
   apply already.
3. **Phase 5A**: Migrate the 7 confirmed-broken sudo handlers to use
   `ctx.Privileged()`. One PR per handler is fine — they're
   independent.
4. **Phase 2-3**: Executor retry/override centralization. Migrate
   `shell`/`command`/`http_request`/`download` off their internal
   retry loops in the same PR. Must preserve MT-48's invariant
   (overrides apply after retry, not during).
5. **Phase 5B**: Verify `pkg`, `git.clone`, `file.*` actions now
   actually retry when users declare `retry:` on them. Update the
   "what retries" docs.

Each phase is independently revertable. Phase 1 (this spike) is the
proof point — if `Privileged()` is the right primitive shape, the
rest is mechanical.

---

## Tests

- **Pipeline-level:** a fake action that records attempts/sudo-state
  in a slice. Drive it with `retry: { attempts: 3, delay: 10ms }` +
  `as_user: root` + `--sudo-pass-file ...` and assert: 3 attempts
  recorded, each ran under sudo, final result respects
  `failed_when:false`.
- **Plan-time enforcement:** action declares `Sudo:true`, run plan
  with no sudo password and non-root user. Expect plan failure with
  the documented message — not a silent "would-change" entry that
  fails at apply.
- **MT-48 regression:** existing `TestRetry_TriggeredEvenWhenFailedWhenIsFalse`
  passes after `shell` migrates off its internal retry.
- **MT-62 regression:** new test that exercises backoff on a handler
  that doesn't have its own retry loop today (e.g. `pkg`) and asserts
  the backoff strategy is honored.

---

## Why now

- The os.systemd sudo bug shipped today as a one-off fix. Without
  centralization, every new `os.*` / `pkg.*` action is a chance for
  the next one.
- The dotfiles "shell-free" migration (sibling effort in
  alehatsman/dotfiles) needs `pkg.repo`, `pkg.upgrade`, and several
  `os.*` actions to actually escalate. Patching them ad-hoc costs
  the same as fixing the disease once.
- MT-48/MT-62 are the same evidence from a different cross-cutting
  concern; the codebase has already paid the bill twice.
- Spec-22 already shipped the preflight infrastructure
  (`Permissions()`); this spec is the executor finally using it.

---

## Status notes

- 2026-05-17: spec drafted alongside Phase 1 spike. `ctx.Privileged()`
  primitive landing in same branch as the `pkg.upgrade` migration.
- Sibling work: alehatsman/dotfiles is paused on the shell-free
  migration pending pkg.upgrade + pkg.repo escalation correctness.
