---
id: F058
title: a step interrupted mid-flight is double-counted as both Cancelled and Failed — handleStepError has no cancel-awareness, so syncResultEnvelope's Cancelled bump is shadowed by a Failed bump; defeats mapCancelExit's `Failed == 0` gate (timeout/fleet/MCP cancels exit 1 instead of 130)
severity: risk
package: internal/executor
file: internal/executor/executor.go
lines: 569-600 (syncResultEnvelope), 639-697 (handleStepError), 926-947 (ExecuteStep dispatch tail); cmd/kernel/apply.go:240-245 (mapCancelExit)
status: done
fixed: 2026-05-29 — `handleStepError` now gates the `incStat(ec.Svc.Stats.Failed)` bump on `!(ec.CurrentResult != nil && ec.CurrentResult.Cancelled)`. A step already classified Cancelled by `syncResultEnvelope` no longer also bumps `Stats.Failed`, so `mapCancelExit`'s `Cancelled > 0 && Failed == 0` gate holds and timeout/fleet/MCP cancels exit 130. The `step.failed` terminal event still fires (carries the handler's stdout/stderr; keeps streaming consumers from seeing an orphaned `step.started`) and rollback is untouched — only the run-counter bump is suppressed. This is the finding's recommended minimal landing (choice 1, "guard the counter only"); the handler-shape skew (handlers returning `(nil, err)` never reach `syncResultEnvelope` so they stay Failed-only) is the larger deferred change in §"Proposed fix" choice 2, left open.
followup-fixed: 2026-05-29 (choice 2, `fix-f058-classify-centralize`) — the handler-shape skew is now closed. `handleStepError` classifies cancel off `ec.Svc.Ctx.Err()` rather than `ec.CurrentResult.Cancelled`, making it uniform across handler return shapes: `(nil, err)` handlers (pkg install of a missing package, spec-69 RawRunners on error) that never reach `syncResultEnvelope` now also count as Cancelled (not Failed) on a mid-flight cancel, and `handleStepError` back-fills `ec.CurrentResult.Cancelled`/`CancelledReason` so the runlog/streaming view stays consistent. `handleStepError` is now the single `Stats.Cancelled` bump site; `syncResultEnvelope` lost its `stats *ExecutionStats` param and no longer touches any counter — it only tags the `*Result` envelope. New regression `TestHandleStepError_NilResultHandler_CancelClassified` pins the `(nil, err)` path; `TestHandleStepError_CancelledStepNotDoubleCounted` updated for the moved bump site; `sync_envelope_test.go` updated for the new signature (counter assertions dropped). `go test ./...` green (133 pkgs), `gofmt`/`go vet` clean. Note: no CLI `--timeout` flag exists, so the finding's `mooncake apply --timeout 2s` repro is hypothetical CLI surface — the cancel contract is exercised programmatically (apply/fleet/MCP) and is verified at the unit level.
post-fix verified: 2026-05-29 — new `TestHandleStepError_CancelledStepNotDoubleCounted` (internal/executor/cancel_classification_test.go) drives the real `dispatchRunner` → `handleStepError` integration with a cancelled run ctx and asserts `Cancelled == 1 && Failed == 0`; without the gate it fails with `Failed == 1`. Control `TestHandleStepError_PlainFailureStillCounted` pins that a genuine failure on a live ctx still lands `Failed == 1 && Cancelled == 0` (fix doesn't over-reach). `go vet` clean; full `internal/executor`, `internal/apply`, `internal/logger`, `cmd/kernel` test packages green.
discovered: 2026-05-29 — cold-read of the post-2026-05-27 executor deltas (heal seam, F6 ok-counter, F4 cancel classification). The F4 work (commit e35593b6) added `syncResultEnvelope` cancel classification + `Stats.Cancelled`, but the integration with the pre-existing `handleStepError` Failed-counter path was never reconciled.
related: F4 (proposal-02 CancelledReason classification, e35593b6), F016 (SIGINT hard-exit follow-up), the F2.11 deferred-os.Exit note (claims.jsonl 2026-05-28: "surface ctx-cancel as Cancelled in shell handler's processCommandResult so mapCancelExit lands 130 on clean drain").
---

## What

When the run-wide ctx is cancelled while a handler is in flight, the
**same step** bumps two mutually-exclusive run counters:

1. `dispatchRunner` calls `syncResultEnvelope(ec.Svc.Ctx, r, err, stats)`
   (`executor.go:1650`). Because `runCtx.Err() != nil`, it marks
   `r.Cancelled = true` and bumps `Stats.Cancelled`
   (`executor.go:576-582`), then `dispatchRunner` returns the handler's
   `err` (`executor.go:1654`).

2. That `err` propagates up: `DispatchStepAction` returns it, and
   `ExecuteStep` routes it into `handleStepError` (`executor.go:932`),
   which **unconditionally** bumps `Stats.Failed` (`executor.go:640`)
   and emits `step.failed`. `handleStepError` has no cancel-awareness —
   it never inspects `ec.CurrentResult.Cancelled` or `runCtx.Err()`.

Net: a step torn down mid-mutation lands as `cancelled=1` **and**
`failed=1`. The console recap (`console_subscriber.go:256-263`) renders
both buckets with no reconciliation, so the operator sees
`RECAP ... failed=1 ... cancelled=1` for a single interrupted step.

## Why this is a risk, not a cosmetic double-count

The double-count silently defeats the proposal-02 exit-code contract.
`mapCancelExit` (`cmd/kernel/apply.go:240`) is the shim that turns a
clean cancel into exit 130 so CI can tell a timeout apart from a real
failure:

```go
func mapCancelExit(kr *apply.KernelResult, runErr error) error {
    if kr != nil && kr.Summary.Cancelled > 0 && kr.Summary.Failed == 0 {
        return cli.Exit("", 130)
    }
    return runErr
}
```

The gate is `Cancelled > 0 && Failed == 0`. Because the interrupted
step bumps **both**, `Failed == 0` is false, so `mapCancelExit` falls
through to `return runErr` → **exit 1**. The programmatic-cancel paths
this shim exists to serve — `--timeout`-bounded runs, fleet-driven
cancel, MCP shutdown — therefore still exit 1, "indistinguishable from
real failures in CI," which is the exact failure the shim's own
doc-comment claims to fix.

The interactive SIGINT path is masked today only because
`runWithSignalCtx` (`apply.go:263`) hard-exits via `os.Exit(130)` in
its signal goroutine before the recap renders. That hard-exit is
explicitly described as a stopgap (`apply.go:256-262`) that should be
removed "once the classification is consistent across handler-killed
children." This finding is that inconsistency — except the bug is the
opposite of what that comment assumes: it assumes a killed child
*under*-counts Cancelled (lands only as Failed); in fact, for any
handler that returns a non-nil `*Result` alongside its error, it
*over*-counts (both).

## Handler-shape inconsistency (the part the existing comments miss)

Whether `Stats.Cancelled` bumps at all depends on the handler's return
shape, because `syncResultEnvelope` only runs inside the
`if r, ok := result.(*Result); ok && r != nil` branch
(`executor.go:1645-1650`):

- Handlers that return a populated `*Result` + err (shell, and most
  spec-69 `RawRunner`s) → `syncResultEnvelope` runs →
  **Cancelled += 1 AND Failed += 1** (double-count).
- Handlers that return `(nil, err)` or a non-`*Result` (e.g. pkg
  install of a missing package — see the typed-nil guard at
  `executor.go:1651-1653`) → `syncResultEnvelope` is skipped →
  **Failed += 1 only** (Cancelled never bumps).

So the recap's cancelled/failed split — and the exit code via
`mapCancelExit` — varies by which handler happened to be in flight when
ctx fired. Neither outcome yields the intended `cancelled=1 failed=0`.

## Reproduction (manual)

```yaml
- name: long sleep
  shell: sleep 60
```

```
mooncake apply --config <playbook> --timeout 2s; echo "exit=$?"
```

Expected (proposal-02 contract): the run reports the step as cancelled
and exits 130. Actual: recap shows `failed=1 ... cancelled=1` and the
process exits 1. (Use `--timeout`, not Ctrl-C — Ctrl-C is masked by the
`os.Exit` hard-exit in `runWithSignalCtx` and won't show the recap.)

Confirm the gate failure directly:

```
mooncake apply --config <playbook> --timeout 2s --format json \
  | jq '.summary | {cancelled, failed}'
# => {"cancelled": 1, "failed": 1}   ← both non-zero
```

## Proposed fix

Make the failure-bookkeeping path cancel-aware so a cancelled step is
counted as cancelled only. The decision point is already centralized —
`ec.CurrentResult.Cancelled` is set by `syncResultEnvelope` before
`DispatchStepAction` returns, so `ExecuteStep` can branch on it:

```go
// executor.go, ExecuteStep dispatch tail (~line 932)
stepErr := DispatchStepAction(step, ec)
if stepErr != nil {
    // F058: a step torn down by run-wide cancel is already counted in
    // Stats.Cancelled by syncResultEnvelope. Do not also count it as
    // Failed — emit a cancelled terminal event and return the err so
    // the loop unwinds, but skip the handleStepError Failed bump.
    if ec.CurrentResult != nil && ec.CurrentResult.Cancelled {
        emitStepCancelled(step, ec, stepID, stepName, depth, stepDuration)
        return stepErr
    }
    if err := handleStepError(step, ec, stepErr, stepID, stepName, depth, stepDuration); err != nil {
        return err
    }
    return nil
}
```

Two open design choices for the operator to decide:

1. **Terminal event.** Today the cancelled step emits whatever the
   handler emitted plus nothing terminal (if we skip `handleStepError`,
   no `step.failed` fires). Either add a `step.cancelled` event type, or
   keep emitting `step.failed` but gate only the *counter*. The
   counter+exit-code bug is the substantive part; the event shape is a
   smaller call. Minimal fix: in `handleStepError`, guard just the
   `incStat(ec.Svc.Stats.Failed)` line on
   `!(ec.CurrentResult != nil && ec.CurrentResult.Cancelled)` and leave
   the event emission alone — smallest possible blast radius, keeps the
   diagnostic `step.failed` with stdout/stderr, fixes the recap + exit
   code.

2. **Handler-shape consistency.** The `(nil, err)`-returning handlers
   still never bump Cancelled (syncResultEnvelope is skipped for them).
   If the goal is a uniform `cancelled=1 failed=0` regardless of handler
   return shape, the cleanest seam is to also classify in `ExecuteStep`
   off `ec.Svc.Ctx.Err()` rather than relying on the handler to return a
   `*Result`. That subsumes choice 1 and removes the
   `syncResultEnvelope`-only Cancelled bump entirely. Bigger change;
   defer unless the per-handler skew is judged worth closing now.

Recommended minimal landing: choice 1's "guard the counter only"
variant — one-line gate in `handleStepError`, plus the test below.

## Pre-fix smoke test (proves the bug exists)

Integration test in `cancel_test.go`: build a plan with one
`sleep`-style step, run `ExecutePlan` with a ctx that cancels ~50 ms in,
assert `*stats.Cancelled == 1 && *stats.Failed == 0`. Without the fix
this fails with `Failed == 1`. The existing `sync_envelope_test.go`
suite does **not** catch this — it exercises `syncResultEnvelope` in
isolation (verifying the `Result` struct's `Failed` stays false) and
never runs the `handleStepError` integration that re-bumps the run
counter.

## Note for TODO.md

This is the substantive open item from the 2026-05-29 re-review of the
post-2026-05-27 executor deltas. The heal seam (proposal-11), F6
ok-counter, and F4 `classifyCancelReason` helper itself were all read
end-to-end and are correct; this counter/exit-code reconciliation gap is
the one defect found.
