---
id: F053
title: executor.runWithRetry uses uncancellable time.Sleep between attempts — Ctrl-C / context cancel cannot interrupt long retry delays
severity: risk
package: internal/executor
file: internal/executor/retry.go
lines: 104
status: open
discovered: 2026-05-27 — cold-read of internal/executor/ per PICKUP item #1. retry.go is the spec-69 phase-2 retry+override centralization promoted out of internal/actions/shell/backoff.go. The retry loop sleeps between attempts via the bare `time.Sleep(delay)` call:

```go
// internal/executor/retry.go:62-115
func runWithRetry(
    step *config.Step,
    log logger.Logger,
    attemptFn func(attempt int) (actions.Result, error),
    isRetryable func(actions.Result, error) bool,
) (actions.Result, error) {
    // ...
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        // ...
        if attempt < maxAttempts && step.RetryDelayDuration() != "" {
            base, parseErr := time.ParseDuration(step.RetryDelayDuration())
            if parseErr != nil { /* log+continue */ } else {
                delay := scaleRetryDelay(base, step.RetryBackoffStrategy(), attempt)
                // ...
                time.Sleep(delay)   // ← uncancellable
            }
        }
    }
    // ...
}
```

`runWithRetry` does not receive a `context.Context`; the `attemptFn` closure carries ctx implicitly (the handler's own subprocess is bound to ctx via `exec.CommandContext`). Between attempts, however, the executor sits in `time.Sleep` with no Done-channel awareness.

Worst-case blast radius: spec-69 promoted `scaleRetryDelay` from the shell handler to the executor, which means **every** spec-69-migrated action (shell, command, download, http_request, package, os_user, os_cron, pkg_upgrade…) now retries through this path. A user with `retry: { attempts: 5, delay: 30s, backoff: exponential }` on any retryable action faces, between attempts 5 and 6, a `30s · 2^4 = 480s` (8-minute) hard-blocking sleep where SIGINT / context cancel produces zero response until the timer expires. Smaller examples are no better in spirit: `retry: { attempts: 3, delay: 10s, backoff: linear }` between attempts 3 and 4 sleeps 30s — long enough that an impatient operator will assume mooncake hung and `kill -9` the process.

Compounding factor: `scaleRetryDelay` caps the exponential shift at `1<<30` (line 38) so a typo'd `attempts: 50` doesn't overflow the duration arithmetic, but `1<<30 · base` still produces astronomical delays (`1B · 1s = ~34 years`). The cap protects the math, not the operator.

related: F014 (`fleet.Apply WithoutCancel hangs Ctrl-C`), F016 (`agentd.Worker no-cancel context`), F042 (`facts.Collect no ctx / per-cmd timeout`), F051 (`os_* handlers context.TODO()`). Same family — every place mooncake blocks without a Done channel is a latent UX cliff.
---

## What

The retry loop in `internal/executor/retry.go` sleeps between attempts
using `time.Sleep(delay)`. `runWithRetry`'s signature carries no
`context.Context`, and the `time.Sleep` call has no `select` over a
cancel channel. Result: an in-flight retry cannot be interrupted; the
operator's Ctrl-C is queued until the sleep ends and the next
`attemptFn` invocation observes the cancelled context (or doesn't —
some action handlers don't propagate ctx into their own work either).

The bug is shared by `runWithRetry`'s sole caller, `dispatchRunner`
(`internal/executor/executor.go:1384`):

```go
result, err = runWithRetry(&step, ec.GetLogger(), func(attempt int) (actions.Result, error) {
    lastAttempt = attempt
    return rr.RunRaw(ec, &step)
}, isRetryable)
```

`ec` carries no `context.Context` field that `runWithRetry` could
reach — `actions.Context` exposes variables, template, etc., but not
the outer ctx. The ctx that the executor receives from
`ExecutePlan(ctx, ...)` is propagated into handlers via the
`actions.Context` *implementations* (which embed an unexported
ctx), not via the executor's own retry path.

## Why this is a risk, not a smell

1. **It's reachable today.** Any step in any of the spec-69-migrated
   actions can carry a `retry:` block. The default `retry_attempts: 0`
   means most operators are unaffected, but the moment a playbook
   author writes `retry: { attempts: 3, delay: 10s }` to harden a
   flaky network call, the unkillable-sleep window opens.
2. **The blast radius scales with `backoff: exponential`.** spec-69
   phase 2 introduced the linear/exponential modes to make retry more
   useful for transient failures — but exponential is exactly the
   shape that turns one retry-delay value into many minutes of
   un-cancellable wait.
3. **It violates the project's own established discipline.** F014 /
   F016 / F042 / F051 all closed by plumbing ctx into the relevant
   call path. The retry sleeper is the same anti-pattern, missed
   because spec-69 promoted the loop without auditing the cancel
   path.

## Reproduction (manual)

```yaml
- name: artificially-failing step
  shell: false
  retry:
    attempts: 5
    delay: 30s
    backoff: exponential
```

Run `mooncake apply --config <playbook>`; mid-retry, press Ctrl-C.
Observe: the next `[WARNING] Waiting Ns before retry (backoff=…)`
log line prints, mooncake sits idle for the full delay, and only
then does the cancel propagate. Repeat with `attempts: 2` to bound
the test.

## Proposed fix

Plumb `context.Context` into `runWithRetry`. The minimal contract:

```go
// internal/executor/retry.go
func runWithRetry(
    ctx context.Context,
    step *config.Step,
    log logger.Logger,
    attemptFn func(ctx context.Context, attempt int) (actions.Result, error),
    isRetryable func(actions.Result, error) bool,
) (actions.Result, error) {
    // ...
    if attempt < maxAttempts && step.RetryDelayDuration() != "" {
        // ... compute delay ...
        timer := time.NewTimer(delay)
        select {
        case <-timer.C:
            // delay elapsed — go to next attempt
        case <-ctx.Done():
            timer.Stop()
            return lastResult, ctx.Err()
        }
    }
}
```

`attemptFn` should also receive ctx so the closure body — currently
`return rr.RunRaw(ec, &step)` — can refresh whatever the handler
needs from ctx (which the actions interface already exposes via
`actions.Context`). The signature change is internal; only
`dispatchRunner` calls `runWithRetry`, so the migration is
one site.

Plumbing ctx through requires:

1. `executor.ExecutePlan` already takes `ctx context.Context` — thread
   it onto `ExecutionContext` (a new field `ec.Ctx context.Context`).
2. `dispatchRunner` reads `ec.Ctx` and forwards it into
   `runWithRetry`.
3. `runWithRetry` swaps `time.Sleep` for `select { <-timer.C / <-ctx.Done() }`.

The fix is local to `internal/executor/`; no handler change is
required (handlers that already respect ctx keep working; handlers
that don't see no new behaviour during the sleep window).

## Pre-fix smoke test (proves the bug exists)

The repro above is the smoke test. A unit test using a fake
`attemptFn` that always errors + a `delay: 10s` + a `context.Cancel`
fired from a goroutine asserts that `runWithRetry` returns
`ctx.Err()` within ~1s of the cancel rather than blocking the full
10s. Mirror the
`internal/actions/wait_command/handler_test.go:TestRunCommand_CtxCancel`
shape.

## Adjacent observation (deferred — note in TODO.md, do not file separately)

`runWithRetry`'s "all attempts failed" error message at line 112 says
`command failed after N attempts: %w`. Spec-69 phase 2 promoted this
helper out of shell's backoff.go but the message text wasn't
generalised; it still implies shell/cmd. Any spec-69-migrated action
(template, file.write, pkg.upgrade) using retry surfaces a confusing
"command failed" message even when no command ran. Fix: change to
`step failed after %d attempts: %w`. Mechanical; group with the F053
ctx-plumbing PR.
