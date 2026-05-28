# Result-envelope followups (proposal-01 / -02 / -06)

**Status:** Punch list for a fresh agent. Bundle shipped 2026-05-28
on master via `076af5b8` (envelope + handler sweep) and `fe30524a`
(cleanup: missed handlers, OpReverted wire-up, exit code 130). Both
gates green. This file lists what's still loose.

**Update 2026-05-28:** F2, F4, F5, F6 shipped — see per-section
`SHIPPED` markers. Still open: F1 (dispatchRunner refactor, deferred
by policy), F3 (apt/dnf dedup, deferred by policy), F7 (status-enum
docs), F8 (observe.target const), F9 (YAML migration doc), and the
F2.11 finale (drop `os.Exit(130/143)` hard-kill, blocked on shell
handler surfacing ctx-cancel as Cancelled rather than Failed).

**Context for the fresh agent:** read these first.

- `docs-working/streams/core/proposals/proposal-01-result-schema-conventions.md`
- `docs-working/streams/core/proposals/proposal-02-recap-counter-discipline.md`
- `docs-working/streams/core/proposals/proposal-06-failed-vs-error-distinction.md`
- `internal/executor/result.go` — envelope fields + helpers + Status() precedence
- `internal/executor/executor.go` — `syncResultEnvelope` (sync helper) +
  `dispatchRunner` (the over-cap function)
- `internal/executor/capture.go:markStepReverted` — OpReverted wire-up
- `cmd/kernel/apply.go:mapCancelExit` — 130 exit-code shim
- `examples/register/README.md` — the envelope shape, as documented to operators

---

## Categorization

Items are tagged with:

- **Carrying-cost** — something already in the codebase that costs
  every reader (linter warning, code smell, soft-cap violation)
- **Accuracy gap** — the envelope reports something subtly wrong or
  lossy under known conditions
- **Polish** — observable inconsistency, no correctness issue
- **Doc** — code is right; users don't know about it yet

---

## F1. dispatchRunner gocyclo 41 > soft-cap 35

**Tag:** Carrying-cost. Pre-existing violation. The proposal-06 sync
landed at gocyclo 45, restored to 41 by extracting
`syncResultEnvelope`. 41 is what the function was at before this
work — the bundle didn't make it worse, but it didn't make it
better either, and the soft cap (`CLAUDE.md` Architecture soft caps
§3) says "refactor on next touch".

**What's still inside `dispatchRunner` (internal/executor/executor.go:1421):**

The body has three concerns mashed together:

1. **Dispatch dispatch** — pick `runner.Run` vs `runner.RunRaw`,
   honor mode, wire the retry loop, fix up `Data["attempts"]`.
2. **Override application** — `applyResultOverrides` + the MT-48 /
   B0 `failed_when` reconciliation logic (the gnarly branch at
   ~lines 1455-1483 with three nested if/else).
3. **Result capture + plan/apply emission tail** — `ec.CurrentResult`
   assignment, `syncResultEnvelope` call, the plan-mode
   StepChecked event with Diff/Cost fan-out (~lines 1493-1572).

**Suggested decomposition:**

- Extract `runRawWithRetryAndOverrides(ec, step, rr)` covering
  concern 1 + 2 — every conditional branch in the retry+override
  zone moves there. The MT-48 / B0 gates are unit-testable in
  isolation (and the existing tests in `dispatch_runner_test.go`
  pin them).
- Extract `emitPlanModeStepChecked(ec, step, runner, result)` for
  the ModePlan tail (Diff/Cost callout + event emit).
- `dispatchRunner` becomes a thin coordinator: pick runner,
  delegate, capture, branch on mode.

**Validation:**

- `gocyclo -over 0 internal/executor/executor.go | grep
  dispatchRunner` should report ≤ 35
- All `dispatch_runner_test.go` tests pass unchanged
- `task ci` clean (the budget-status pass should report
  dispatchRunner is no longer in the gocyclo-over-cap list)

**Don't:** rewrite the logic. The MT-48 invariant (retry decides on
raw err, never on post-override verdict) and the B0 fix (don't
silently clear err without `failed_when`) are subtle. The goal is
the same behavior in smaller functions.

---

## F2. F016 — ctx threading through every handler — SHIPPED 2026-05-28

**Landed:** 11 commits `d8c0e6ee` → `6925b967` on master.

Handler-side ctx threading complete across the punch list:
F2.1 actions.Context.Ctx() foundations + 3 implementers
(ExecutionContext / reverseContext / MockContext); F2.2 shell
setupCommandContext with SIGINT-during-sleep regression test
(0.15s cancel vs 30s pre-F2); F2.3 wait_command pollCtx; F2.4
pkg_repo/brew (ListTaps + Exec + HTTPFetchKey); F2.5 pkg.hold +
apt + dnf (12 hook signatures + HTTPFetchKey); F2.6 git_clone +
git_config; F2.7 assert (5 sites + ctxFor helper); F2.8 os_user +
os_group platform_{linux,darwin,windows} + dscl.Run signature;
F2.9 observe_process; F2.10 windows.hyperv_firewall_rule +
service/windows.

**Still open — F2.11:** drop the `os.Exit(130/143)` hard-kill in
`runWithSignalCtx`. Blocked on shell handler's
`processCommandResult` surfacing ctx-cancel as a non-nil err (so
syncResultEnvelope's existing classification kicks in and
Stats.Cancelled gets bumped). Without that, dropping the hard-kill
regresses TestIssue87_SIG{INT,TERM}ExitsCleanly because
mapCancelExit sees kr.Summary.Cancelled == 0 and falls through to
runErr → exit 1 instead of 130.

---

## F2. F016 — ctx threading through every handler (original brief, kept for reference)

**Tag:** Accuracy gap. Highest-effort item in this file. Wide-blast-
radius surgery.

**The gap:** today the executor sees `ec.Svc.Ctx` cancellation
**between** steps (the loop check at `ExecuteSteps`) and inside the
`syncResultEnvelope` helper (the per-handler ctx-aware
classification). What it doesn't see: a handler that ignores ctx
and completes normally during a cancel.

Concrete worst case: SIGINT during a long-running `shell: sleep 30`.
Today the CLI signal handler in `cmd/kernel/apply.go:runWithSignalCtx`
hard-exits via `os.Exit(130)` after firing the signal — the shell
child gets killed by the OS, the recap never renders, the run logs
never close. That's the F016 followup in `apply.go`'s own comment.

**What "done" looks like:**

1. Every action handler accepts the run-wide ctx (either via the
   existing `ec.Svc.Ctx` reach-through or — cleaner — by giving the
   `actions.Context` interface a `Done() <-chan struct{}` method).
2. Every shell-out (`exec.Command`, `net/http`, `os.Open` of a
   slow filesystem) uses the ctx-aware variant: `exec.CommandContext`,
   `http.NewRequestWithContext`, etc. Most are already in place for
   wait.*, http_request, observe_http; need an audit for the rest.
3. When ctx fires mid-handler, the handler returns
   `(partial-result, ctx.Err())` instead of looping or blocking. The
   existing `syncResultEnvelope` then classifies that as Cancelled.
4. Drop the `os.Exit(code)` hard-kill in `runWithSignalCtx` — the
   apply now drains cleanly on signal, runs.jsonl gets closed, the
   recap renders with `cancelled=N`.

**Order to do this in:**

1. Define the `Done()` channel on `actions.Context` (or document
   `ctx.Svc.Ctx` as the canonical reach-through).
2. Sweep handlers grouped by what they shell out to: shell/command,
   http_request + observe_http + wait_http (already mostly ctx-
   aware), pkg.* (apt/dnf/brew shell-outs), os.* (useradd, mount,
   etc.), container.*, git.*. Each handler should be one PR.
3. Once every handler is ctx-aware, drop the `os.Exit` hard-kill.

**Validation per handler:**

A test that:

1. Constructs a ctx with `WithCancel`.
2. Spawns a goroutine that calls `Run(ctx, longRunningStep)`.
3. Cancels the ctx after a short delay.
4. Asserts: handler returns within reasonable bounds (not the full
   timeout); result has `Cancelled=true`,
   `CancelledReason=sigint`; `Error` contains the underlying
   `context.Canceled` or a wrap.

---

## F3. dupl 49-line clone: pkg_repo/apt vs pkg_repo/dnf

**Tag:** Carrying-cost. `task ci` dupl pass surfaces it
informationally (not blocking):

```
49L  internal/actions/pkg_repo/apt/apt.go:249-297
 <--> internal/actions/pkg_repo/dnf/dnf.go:227-275
```

**What duplicates:** both apt and dnf drivers have the same
plan→capture→apply→reverse-prep skeleton. The differences are
small: file paths (`/etc/apt/sources.list.d/` vs
`/etc/yum.repos.d/`), the binary name (`apt-get` vs `dnf`), the
keyring layout. The control flow is identical.

**Suggested approach:** lift the shared skeleton to
`pkg_repo/shared/driver.go` parameterized by a small
`driverConfig` struct:

```go
type driverConfig struct {
    SourcesDir, KeyringDir string
    UpdateCmd []string       // apt-get update / dnf makecache
    InstallSourcesFile func(name, body string) error
    ...
}
```

apt and dnf become 20-line shells that fill in the config and call
`shared.RunDriver(cfg, r, result)`.

**Don't generalize beyond what's already shared.** brew is
sufficiently different (no sources file, taps via a CLI verb) that
forcing it into the shared driver would be worse. Keep brew its own
shape.

**Validation:** existing apt + dnf tests still pass; line count of
each driver drops by ~40%; dupl no longer flags the pair.

---

## F4. CancelledReason is coarse — SHIPPED 2026-05-28

**Landed:** `e35593b6` on master.

`syncResultEnvelope` now reads `context.Cause(runCtx)` against typed
sentinels — `ErrCancelSignal` / `ErrCancelFleet` / `ErrCancelMCP` —
instead of collapsing every non-deadline cancel onto `"sigint"`.
Plain `WithCancel` no-cause cancels now map to `"cancelled"` generic.
Live producers wired: `cmd/kernel/apply.runWithSignalCtx` and
`internal/fleet/Orchestrator.installSignalHandler`. `ErrCancelFleet`
and `ErrCancelMCP` declared but unwired pending fleet-kill wire and
MCP shutdown surface. 4 new sync_envelope tests pin Signal / Fleet /
MCP / Unknown attribution paths.

---

## F4. CancelledReason is coarse (original brief, kept for reference)

**Tag:** Accuracy gap. Single-bit `sigint` vs `timeout` distinction
today.

**What's happening:** `syncResultEnvelope` derives
`CancelledReason` from `runCtx.Err()`:

- `context.DeadlineExceeded` → `CancelledReasonTimeout`
- anything else (`context.Canceled`) → `CancelledReasonSigint`

But "anything else" actually covers:

- OS signal (SIGINT, SIGTERM) — true SIGINT case
- `fleet exec --kill` propagating cancel via the wire protocol
- MCP shutdown cancelling the inflight tool call
- Programmatic `apply.Runner` caller cancelling its own ctx

These all map to `"sigint"` today, which is wrong for three of the
four.

**Suggested approach:** Go 1.20+ `context.WithCancelCause` +
`context.Cause(ctx)`. Producers (signal handler, fleet kill
wire-handler, MCP shutdown) attach a typed cause via
`context.WithCancelCause`; `syncResultEnvelope` reads
`context.Cause(runCtx)` and classifies.

```go
var (
    ErrCancelSignal = errors.New("cancel: os signal")
    ErrCancelFleet  = errors.New("cancel: fleet kill")
    ErrCancelMCP    = errors.New("cancel: mcp shutdown")
)
```

New `CancelledReason` enum values: `fleet_kill`, `mcp_shutdown`,
keep `sigint` as the "OS signal" specific bucket, keep `timeout` for
`DeadlineExceeded`. Default → `cancelled` (generic) when no cause
is attached.

**Validation:** new unit tests in `sync_envelope_test.go` for each
cause path. End-to-end test: fleet exec kills a peer mid-run, peer
reports `cancelled_reason: fleet_kill` in `runs.jsonl`.

---

## F5. Plan-mode Operation contract is informal — SHIPPED 2026-05-28

**Landed:** `14dcf3c1` on master.

Plan-mode handlers now emit `OpNoop` in already-converged branches,
matching the apply-mode envelope for the same input. ~15 handler
families swept. Plan→apply numerical comparisons stay honest across
idempotent runs.

---

## F5. Plan-mode Operation contract is informal (original brief, kept for reference)

**Tag:** Polish. No behavioral bug, but the codebase is split.

**What's inconsistent:** during `ctx.Mode() == ModePlan`, handlers
do one of two things with `result.Operation`:

- Set the would-be apply verb (`OpCreate`, `OpUpdate`, `OpDelete`)
  plus `WouldChange=true`. Example: `os_user`, `os_group`,
  `os_cron`, `file`, most mutation handlers.
- Set `OpNoop` because nothing would change. Example: the "already
  at desired state" branches in `os_firewall`, `os_ssh_key`, the
  windows handlers' state-already-matches branches.

Both interpretations are defensible:

- Interpretation A: Operation = "what apply WOULD do" — predictive.
- Interpretation B: Operation = "what we predict the apply
  outcome's Operation would be" — including OpNoop for would-be-
  idempotent runs.

Today's mix happens because each handler authored its plan branch
independently.

**Suggested resolution:** codify B (Operation reflects the predicted
apply-time verb, including `OpNoop` for predicted no-ops). Then
sweep handlers that left Operation as the would-be-mutation verb in
"nothing to do" branches.

**Why B over A:** B makes the envelope semantic stable across plan
and apply for the same input — a plan that predicts OpNoop and an
apply that lands OpNoop produce the same `result.operation` value.
A makes them disagree on idempotent runs, which trips downstream
typed-diff consumers.

**Validation:** new test per handler family asserting that plan
mode on an already-converged target reports `Operation=OpNoop`,
`WouldChange=false`.

**Files to sweep:** `os_user`, `os_group`, `os_cron`, `os_systemd`,
`os_sysctl`, `os_mount`, `os_ssh_key`, `file`, `copy`, `download`,
`template`, `text_*`, `git_*`, `pkg.*`. Probably ~15 files.

---

## F6. Recap "ok" is derived in the renderer — SHIPPED 2026-05-28

**Landed:** `7f719195` on master (feat commit `f46a0135`).

`OK *int` added to `ExecutionStats`, bumped at four sites:
`postExecuteSuccess` when changed==false (apply no-op), plan-mode
dispatch when wouldChange==false, the unknown-action plan path, and
MT-45 rollback (alongside Reverted, preserving OK+Changed==Executed).
`events.RunCompletedData` carries `OkSteps`; `apply.RunSummary` adds
`OkSteps` next to the legacy `Ok` field (kept as SuccessSteps for
MCP/agentd/history back-compat). 3 renderer sites read OkSteps with
pre-F6 subtraction fallback. 2 end-to-end tests pin the invariant.

---

## F6. Recap "ok" is derived in the renderer (original brief, kept for reference)

**Tag:** Polish. Data-model smell, no numerical bug.

**Current code** (`internal/logger/console_subscriber.go:235`):

```go
ok := data.SuccessSteps - data.ChangedSteps
if ok < 0 { ok = 0 }
```

The executor doesn't own an `ok` counter — it owns `SuccessSteps`
(steps that didn't fail), `ChangedSteps`, `FailedSteps`,
`SkippedSteps`, `RevertedSteps`, `CancelledSteps`. The renderer
subtracts to derive "ok = ran but no change".

**Why this is a smell:** every downstream consumer that wants the
`ok` number reimplements the subtraction. agentd's run-completed
JSON, MCP's apply response shaper, future SDK clients — they all
have to know "ok is implicit, subtract Changed from Success and
clamp".

**Suggested fix:** add `OK *int` to `ExecutionStats`; bump it in
the executor at the same place `Changed` is bumped (mutually
exclusive — a step is `ok` or `changed`, not both). Expose
`OkSteps` on `RunCompletedData` and `RunSummary`. Renderer reads
the field directly.

**Don't:** remove `SuccessSteps` — backward-compat for existing
agentd/MCP consumers. Keep both; they're trivially related.

**Validation:** numerical match — for any run, `OkSteps +
ChangedSteps == SuccessSteps` always. Existing recap tests should
keep passing unchanged.

---

## F7. `result.status` enum grew two values — agents don't know

**Tag:** Doc.

**What changed:** `Result.Status()` (`internal/executor/result.go:220`)
now returns `cancelled` and `reverted` in addition to the legacy
`ok / changed / failed / skipped`. External consumers that switch
on `status` need to know about the new buckets.

**Known consumers** (a quick grep can extend this list):

- `internal/logger/console_subscriber.go` — renders the per-step
  marker icon; already handles all the values via `Status()`.
- `cmd/agentd/runs.go` — agentd's run-tail streamer; same.
- `internal/mcp/tools.go` — MCP tool response shaping.
- `internal/pilot/output_capture.go` — pilot loop's per-step summary
  fed back to the LLM.
- External: any user playbook with `when: result.status == "X"`.

**Suggested doc additions:**

- `examples/register/README.md` — list the full status enum (already
  partly documented; add `cancelled` and `reverted` rows).
- `LLM_GUIDE.md` — if it documents the register shape (it might
  not), add the same.
- Migration note in the proposal-02 doc reminding users that
  `when: result.status in ['ok', 'changed']` is too narrow if
  they actually want "succeeded somehow".

**Validation:** none beyond docs being accurate. There's no code to
write here.

---

## F8. `observe.* target` convention

**Tag:** Polish.

**What's inconsistent:** the 9 observe.* handlers pass different
shapes for `Target`:

| Handler | Target |
|---|---|
| observe.cpu | `"host"` |
| observe.memory | `"host"` |
| observe.gpu | `"host"` |
| observe.disk | absolute path |
| observe.http | URL |
| observe.logs | `source + ":" + identifier` |
| observe.port | `host:port` addr |
| observe.process | `"name=foo"` / `"pattern=^bar$"` selector |
| observe.service | service name |

The "system-wide" observations (cpu, memory, gpu) all share `"host"`
which is fine — but it's a literal string with no canonical
declaration anywhere. If observe gains more system-wide checks
(observe.uptime, observe.load) they need to know to use `"host"`
too.

**Suggested cleanup:** a `const ObserveTargetHost = "host"` exported
from `internal/actions/observe.go` so it's a single referenced
symbol rather than a copy-pasted string literal.

If you want to go further: standardize the non-host shapes too.
e.g. `observe.process` could use a stable `proc:name=foo` /
`proc:pattern=^bar$` shape rather than the bare selector, making
the target visually self-describing.

**Don't churn this unless someone has a concrete need.** It's an
ergonomic nit, not a correctness issue.

---

## F9. Migration-note for YAML users: `failed_when` semantics

**Tag:** Doc. Highest user-facing impact of anything in this file.

**What changed:** the proposal-06 central err→envelope sync
(`syncResultEnvelope`) made `result.failed` truthful in cases where
it used to be `false`. Handlers that returned
`(Result-with-Failed=false, err)` — wait.*, os.mount, os.firewall,
the entire spec-69 B0 cluster — now have `result.failed == true`
post-bundle.

**Why this is observable from YAML:**

Pre-bundle (silent-failure shape):

```yaml
- wait.http: { url: http://localhost:9999/never, timeout: 1s }
  register: r
  failed_when: false   # operator explicitly suppresses
- when: r.failed
  shell: echo "wait failed"
```

Pre-bundle: `r.failed` was `false` even on timeout (the bug),
`when:` skipped the echo. Post-bundle: `r.failed` is `true` on
timeout, `when:` fires the echo. The new behavior is correct; the
old playbook breaks if its author was depending on the bug.

**Action items:**

- Add a section in `examples/register/README.md` under "Migration
  notes" describing the change.
- Optional: a `mooncake doctor` lint rule that flags `failed_when:
  false` on actions where the err return now reaches the envelope.
  Probably overkill.

---

## What's NOT in this list (intentionally)

- **More-handler Operation/Target sweep beyond what's already
  done.** All registered handlers are now covered. If a future
  proposal adds a new action, it needs to set Operation+Target as
  part of the handler-author checklist (see
  `docs-working/streams/core/proposals/proposal-01-result-schema-conventions.md`
  §Implementation pattern).
- **schema.json regen.** Proposal-01 envelope is on `executor.Result`,
  which is runtime shape, not schema-tracked. `task schema-generate`
  produces no diffs. Confirmed during the bundle work.
- **The `OpReverted` enum value.** It's now wired by
  `capture.go:markStepReverted`. The recap counter and per-step
  envelope agree.
- **Exit code 130 for non-SIGINT cancel.** Wired by
  `cmd/kernel/apply.mapCancelExit`, tested in `apply_exit_test.go`.

## Suggested ordering for a fresh agent

**Shipped 2026-05-28:** F2 (handler ctx threading, 11 commits), F4
(CancelledReason via context.Cause), F5 (plan-mode OpNoop sweep),
F6 (recap `ok` as first-class counter).

What's left, in priority order:

1. **F2.11 finale** — drop `os.Exit(130/143)` in `runWithSignalCtx`.
   Blocked on shell handler `processCommandResult` surfacing
   ctx-cancel as non-nil err so syncResultEnvelope's
   classification kicks in. ~30 min once that's in place.
2. **F9 (YAML migration doc)** — pure docs, ~30 min, biggest
   user-visible value of anything still open. Handled by the docs
   agent.
3. **F7 (status-enum doc)** — pure docs; handled by docs agent.
4. **F1 (dispatchRunner refactor)** — defer until the next time
   someone needs to touch dispatch for unrelated reasons. The
   gocyclo violation is informational; routinely-refactoring
   isn't the cap's intent.
5. **F3 (apt/dnf dedup)** — only do this when one of the drivers
   needs a real change anyway, so the shared extraction is paid
   for by real demand.
6. **F8 (observe.target const)** — polish. Pick up when you're
   already in the neighborhood.
