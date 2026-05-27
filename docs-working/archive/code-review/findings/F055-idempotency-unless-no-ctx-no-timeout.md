---
id: F055
title: executor.checkIdempotencyConditions runs `unless:` command without ctx/timeout — same F051/F053 family, but for every step's pre-flight idempotency guard
severity: risk
package: internal/executor
file: internal/executor/executor.go
lines: 302
status: done
fixed: 2026-05-27 — `checkIdempotencyConditions` now reads `ec.Svc.Ctx` and shells out via `exec.CommandContext(ctx, "sh", "-c", command)`. Added a per-guard hard-timeout cap via `context.WithTimeout(runCtx, idempotencyUnlessTimeout)` where `idempotencyUnlessTimeout = 10 * time.Second` — bounds well-behaved guards (typical `test -f` / `pgrep` / `kubectl get` complete in <1s on a healthy host) without breaking slow-but-legitimate probes. Nil ctx falls back to `context.Background()` so legacy callers still work, and the timeout still applies (the cap is the safety net even without an outer ctx cancel).
post-fix verified: 2026-05-27 — two new tests in `cancel_test.go`: `TestF055_UnlessGuardRespectsCtxCancel` (a `unless: sleep 30` step with ctx cancelled at ~50ms returns within 5s; pre-fix this blocked the full 30s) + `TestF055_UnlessGuardHardTimeout` (background ctx, `unless: sleep 30` returns within 13s thanks to the 10s cap; sanity-checks that the file.write proceeded after the unless timed out non-zero). Full `mooncake task ci` green.
discovered: 2026-05-27 — round-2 cold-read of internal/executor/ per PICKUP item #1. `checkIdempotencyConditions` is called from `ExecuteStep` BEFORE the action's own dispatch path runs. When a step's `unless:` (or the spec-21-era equivalent `unless_command:` / `Shell.Unless`) is set, the helper shells out to evaluate it:

```go
// internal/executor/executor.go:296-306
if g.unless != "" {
    command, err := ec.Svc.Template.Render(g.unless, vars)
    if err != nil {
        return false, "", &RenderError{Field: "unless command", Cause: err}
    }
    // #nosec G204 -- This is a provisioning tool designed to execute commands from user configs.
    cmd := exec.Command("sh", "-c", command)
    if err := cmd.Run(); err == nil {
        return true, fmt.Sprintf("unless: %s", command), nil
    }
}
```

`exec.Command` is used instead of `exec.CommandContext`; the call carries no per-command timeout and no `ctx.Done()` integration. `runWithRetry` got the F053 ctx-plumbing treatment, but `checkIdempotencyConditions` was missed — and it runs on EVERY step that declares any of the idempotency-guard aliases (universal-step `unless` / `unless_command` / `creates`, plus shell-level `Shell.Unless` / `Shell.Creates`).

Worst-case blast radius: an operator writes a plausible-looking guard like

```yaml
- name: ensure cluster is ready before proceeding
  unless: kubectl get nodes --request-timeout=300s
  shell: kubectl apply -f manifest.yml
```

— `kubectl` 4xx-storms or network hangs cause the `sh -c` subprocess to wait indefinitely. Mooncake sits in `cmd.Run()` with no ctx awareness; Ctrl-C / context cancel does NOT interrupt because Go's `os/exec` only respects the embedded ctx for `CommandContext` invocations. The operator sees no step.started event (the guard runs before step.started), no progress, no log line — just an unkillable mooncake until the underlying subprocess decides to exit.

The "#nosec G204" annotation is about shell-injection (intentional — `sh -c` lets users compose arbitrary guards), NOT about cancellation. The real concern is the missing ctx, not the security model.

related: F014 (`fleet.Apply WithoutCancel`), F016 (`agentd.Worker no-cancel`), F042 (`facts.Collect no ctx / per-cmd timeout`), F051 (`os_* context.TODO()`), F053 (`runWithRetry time.Sleep`). Same family — every place mooncake blocks on an external process without watching `ctx.Done()` is the same anti-pattern. F051 / F053 closed the worst offenders; F055 is the next-tier site that survived the prior audits because `checkIdempotencyConditions` sits in a pre-flight check, not inside a handler.
---

## What

The idempotency-guard helper (`checkIdempotencyConditions`,
`internal/executor/executor.go:246`) is the canonical entry point for
the `unless:` / `creates:` shape that every step type supports.
Universal step-level fields (`Step.UnlessCommand`, `Step.Unless`,
`Step.Creates`, `Step.UnlessExists`) and the shell-action-level
equivalents (`Shell.Unless`, `Shell.Creates`) all funnel through it.

For each `unless:` guard the helper:

1. Renders the command template against the current variable scope.
2. Shells out via `exec.Command("sh", "-c", command).Run()` to test
   the guard.
3. If `cmd.Run()` returns `nil` (zero exit), the step skips.

Step 2 has the bug: no ctx, no timeout, no cancellation.

## Why it matters

1. **Reachable on the default path.** Any step with any of the
   idempotency-guard aliases hits this code. They're documented in
   `docs-next/guide/best-practices.md` as the recommended way to
   make a step idempotent. Users following the docs encounter this
   site.
2. **The guard runs BEFORE the handler.** A retryable action whose
   `retry:` block carries `ctx`-cancellable sleeps (post-F053)
   still goes through this no-ctx pre-flight. F053 closed half the
   gap; the other half is here.
3. **No visibility window.** Unlike inside-handler hangs (the
   handler emits step.started before stalling), the guard runs
   between `checkSkipConditions` and `step.started`. The operator
   sees nothing — no log line, no event — for the full duration of
   the hung subprocess.

## Proposed fix

Pass the run-wide ctx through to `checkIdempotencyConditions` (the
function already takes `ec`, so it can read `ec.Svc.Ctx`
without a signature change). Replace the bare `exec.Command` with
`exec.CommandContext`:

```go
// internal/executor/executor.go
runCtx := context.Background()
if ec.Svc != nil && ec.Svc.Ctx != nil {
    runCtx = ec.Svc.Ctx
}
// #nosec G204 -- This is a provisioning tool designed to execute commands from user configs.
cmd := exec.CommandContext(runCtx, "sh", "-c", command)
if err := cmd.Run(); err == nil { ... }
```

Optionally also impose a per-guard hard timeout (5–10 s) via
`context.WithTimeout`. Idempotency guards are by convention cheap
checks; anything that takes >5 s probably belongs in a `when:`
expression evaluated against pre-computed facts, not a synchronous
shell-out. A 10 s timeout would catch genuine misuse without
breaking the common case (a `kubectl get` or `pgrep` runs in
<1 s on a healthy host).

The fix is local to one function; no handler change is required.

## Pre-fix smoke test (proves the bug exists)

```yaml
- name: install hello
  unless: sleep 300
  shell: echo hello
```

Run `mooncake apply --config <playbook>`; press Ctrl-C. Observe:
mooncake sits unresponsive for the full 5-minute sleep before the
cancel propagates. The expected post-fix behavior: Ctrl-C aborts
within a second; the run returns `context.Canceled`.

## Same pattern checklist (cross-cutting audit)

A `grep -rn 'exec\.Command\(' internal/` outside `_test.go` returns
~30 sites. Most are inside spec-69-migrated handlers (shell, cmd,
download, pkg, os_user, …) which already plumb ctx via
`exec.CommandContext`. The exceptions are:

- `internal/executor/executor.go:302` ← THIS FINDING
- `internal/actions/preset/<...>` — preset handler internals, separate audit
- A few legacy-Execute paths in actions that haven't been spec-69-migrated
  (covered by F011 / F032)

So F055 is the single high-leverage executor-side site. Closing it
puts the executor's blocking-call audit at 100% (transaction
rollback events emit synchronously — no exec; retry sleep is ctx-
cancellable post-F053; ExecuteSteps loop checks `ctx.Err()` per
step). The remaining `exec.Command` sites all live in handler
packages and are tracked under F051 / F011's broader scope.
