---
id: execution-engine
status: draft
owners: [aleh]
covers:
  - "internal/executor/**"
  - "internal/plan/**"
  - "internal/diff/**"
  - "internal/apply/**"
  - "internal/effects/**"
  - "internal/snapshot/**"
---

# Execution Engine

## Intent
The execution engine is mooncake's typed mutation kernel: it turns a loaded
config into a flat, fully-expanded plan, predicts what each step would change
without touching the system (dry-run + structural diff), and — on apply —
performs idempotent, audited mutations through a mode-aware effects layer. It
guarantees all-or-nothing transactions with automatic LIFO rollback, exposes a
reversible run result, and records system snapshots so changes are observable
and undoable.

## Behavior
- WHEN a config is loaded, the planner SHALL compile it into a flat `Plan`:
  expanding `import`/preset includes, unrolling `for_each` loops, binding
  variables and facts, resolving `!secret` refs, and tagging compound children
  (transaction/try) with their structural role.
- WHEN planning, the engine SHALL run each handler in `ModePlan` and emit a
  per-step inspection recording whether it would change state, why, and (for
  `Differ` handlers) a structural `Diff`.
- WHERE a handler runs in `ModePlan`, the effects `Performer` SHALL predict the
  outcome and perform no system mutation; in `ModeApply` it SHALL perform the
  side effect.
- WHEN `--dry-run` is set, no mooncake-driven mutation SHALL occur, yet template
  rendering and read-only state probes SHALL still run so the preview reports the
  same skip/run decisions the real run would make.
- WHEN `--diff` is requested, the engine SHALL render a per-step structural diff
  (e.g. unified content diff for file writes) for handlers that implement
  `Differ`.
- WHEN a step's idempotency guard (`creates`/`unless_exists`/`unless`/
  `unless_command`) is evaluated, it SHALL run in every mode including plan, so
  guards MUST be side-effect-free read probes.
- WHEN a handler reports no change for a step, that step SHALL count as `ok`
  (ran, unchanged) rather than `changed`, making re-applies idempotent.
- WHEN a step inside a `transaction` fails, the engine SHALL walk the completed
  body children in LIFO order calling each handler's `Reverse()`, leaving state
  equivalent to pre-transaction; an irreversible handler is a quiet skip, a
  `Reverse()` error halts the walk as a partial rollback.
- WHEN a transaction rolls back, its sibling `on_rollback` steps SHALL run; when
  it commits, they SHALL be skipped.
- WHEN a `try` branch errors, `catch` SHALL run and `finally` SHALL always run;
  even when `catch` handles the error the compound step's outcome is failure.
- WHEN a run completes, the engine SHALL expose a `KernelResult` whose
  `Reverse()` builds an inverse plan from the run's reversible (changed) steps in
  LIFO order, usable for cross-run rewind / agent undo.
- WHEN a handler implements `Reverser`, it SHALL capture apply-time pre-state
  (`ReverseData`) before mutating so the inverse step can be constructed later.
- WHEN a run executes, the engine SHALL publish structured events (plan loaded,
  per-step checked/started/completed, run completed) to subscribers for audit,
  TUI, run-log, and SSE consumers without bypassing the kernel boundary.
- WHEN requested, the engine SHALL capture a compact `SystemSnapshot` and compute
  a `Diff` (tools, hardware, OS, failed services) between two snapshots.
- WHEN a step sets `retry`/`timeout`/`continue_on_error`, the executor SHALL
  honor retry policy, bound wallclock, and continue past a failure respectively.
- WHEN a plan is saved and re-loaded, the engine SHALL apply it without
  re-reading the source config (saved-plan and in-memory-plan apply paths).

## Non-goals
- The YAML document/step schema and its validation — config-model spec.
- Per-action handler semantics and the action ABI itself (`internal/actions`) —
  per-action handler docs.
- Expression evaluation and template rendering internals — templating spec.
- Fleet / multi-host orchestration (`internal/fleet`, `agentd`) — it composes
  one `apply.Runner` per peer; that topology is out of scope here.
- Plan signing, policy/egress DSL, deterministic-replay command, per-action
  quotas — unbuilt roadmap (VISION stream 2).

## Checklist
- [x] Planner flattens config: includes/presets, loop unrolling, var/fact
  binding, secret resolution, compound-role tagging.
- [x] Mode-aware effects `Performer` (ModePlan predicts, ModeApply mutates).
- [x] `--dry-run` performs no mutation; guards/probes still evaluated.
- [x] Per-step `ModePlan` inspection with would-change + reason.
- [x] `--diff` structural diff per step via `Differ` (effects.ContentDiff +
  diff renderers).
- [x] Idempotency guards evaluated in all modes; unchanged steps count as `ok`.
- [x] `transaction` LIFO auto-revert via `Reverser`; `on_rollback` siblings.
- [x] `try`/`catch`/`finally` execution semantics.
- [x] `KernelResult.Reverse()` inverse-plan builder for cross-run rewind.
- [x] Apply-time `ReverseData` capture before mutation.
- [x] Structured event substrate (publisher/subscribers, run capture).
- [x] `SystemSnapshot` capture + snapshot `Diff`.
- [x] `retry` / `timeout` / `continue_on_error` execution controls.
- [x] Saved-plan and in-memory-plan apply paths.
- [ ] Plan signing (Sigstore) — unbuilt (VISION stream 2).
- [ ] Policy/egress DSL (`deny:` over Permissions/Diff) — unbuilt (VISION
  stream 2).
- [ ] Deterministic replay command — unbuilt (VISION stream 2).
