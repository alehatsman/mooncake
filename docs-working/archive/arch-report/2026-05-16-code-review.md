# Code Quality Review — 2026-05-16

**Date:** 2026-05-16
**Codebase:** `github.com/alehatsman/mooncake` @ `6f4cde0`
**Scope:** Targeted review of the highest-smell areas from the arch snapshot.
Areas examined: `explain`, `copy`, `executor`, `mcp`, `config.Step`,
`actions/file`, `agentd/worker`, `register`.

---

## 0. Executive summary

Four structural debts dominate. In order of leverage:

1. **Spec-16 incomplete** — `Execute`/`DryRun`/`Check` legacy paths still
   exist alongside `Run()` in `copy`, `file`, `service`, and more. Every
   handler carries two implementations of the same operation.
2. **MCP boundary broken** — `internal/mcp` calls `executor.ExecutePlan`
   directly, bypassing `apply.Runner`. It hand-wires publishers, sinks,
   and result reconstruction that `apply.Runner` already does.
3. **`agentd/worker` calls `executor.Start` directly** — same problem as
   MCP, but at the daemon layer.
4. **`config.Step` has 74 optional action pointers** — exactly-one
   constraint is runtime-only (no compile-time union). Planner-internal
   state lives on the same struct as user input.

Three of these four are the same root cause: `apply.Runner` shipped
(R1.1b) but its callers (`mcp`, `agentd`, handlers) were not migrated.

The fourth (`config.Step`) is a Go language limitation; the best
available improvement is an `ActionType` enum + separation of
user-input fields from planner-populated fields.

---

## 1. `explain.DisplayFacts` — gocyclo 53

**File:** `internal/explain/explain.go` (215 lines, function is 200 of them)

**Diagnosis:** High cyclomatic count is *structural*, not dangerous. Every
optional field on `facts.Facts` gets its own existence guard before
printing. The nolint comment on line 13 is correct.

**Real issues:**
- Table formatting for storage (lines 128–158) is inline — no helper.
- CPU flag filter (lines 37–50) hardcodes prefixes (`avx`, `sse`, `fma`,
  `aes`) inline rather than in a named list.
- Section headers are repeated string literals.

**Easy wins (no behaviour change):**
- Extract `formatStorageTable(disks []Disk) string` — ~25 lines.
- Extract `filterRelevantCPUFlags(flags []string) []string` — names the
  hardcoded list, makes it testable.

**Verdict:** Low priority. No architectural risk. Pick up on next touch.

---

## 2. `copy.(*Handler).Execute` — gocyclo 41

**File:** `internal/actions/copy/handler.go` (731 lines)

**Diagnosis:** `Execute()` (lines 93–288) and `Run()` (lines 589–730) are
~90% duplicate. Both expand paths, check source existence, verify
checksums, determine copy necessity, handle symlinks, copy, chown, and
verify post-copy. Spec-16 added `Run()` *alongside* `Execute()` rather
than replacing it.

**Concrete smell:**
- Symlink path (lines 219–239): early return breaks out of the write
  pipeline, creating two parallel state machines.
- Idempotency decision (lines 172–191): 6-level nesting (symlink→symlink,
  symlink→regular, regular→regular).
- `executeSudoCommand` duplicated in move path and chown path.

**Easy win:**
- Delete `Execute()` and `DryRun()`. All callers go through `Run()`.
  `Run()` already handles `ModePlan` and `ModeApply` via `ctx.Mode()`.
- Extract idempotency decision into
  `func (h *Handler) checkIdempotency(...) (IdempotencyDecision, error)`.

**Larger issue:**
- The Handler interface itself still declares `Execute`, `DryRun`, `Check`.
  Until those are removed from the interface, handlers carry dead weight.
  Needs a spec: "delete legacy Handler interface methods after all handlers
  migrated to Run-only."

---

## 3. `executor.ExecuteStep` — gocyclo 37

**File:** `internal/executor/executor.go` (1,367 lines, `ExecuteStep` is
lines 528–759)

**Diagnosis:** Three distinct concerns woven together:
1. Skip-reason evaluation (lines 564–587) — should skip? why?
2. Plan-mode dispatch (lines 608–640) — two near-identical blocks for
   `Runner`-implementing handlers vs unknown handlers.
3. Post-success plumbing (lines 709–757) — record, emit, register,
   capture, cleanup; spread across 50 lines with no grouping.

**Structural marker no-ops** (lines 540–561): Transaction and Try parents
are no-ops in `ExecuteStep` — they exist only so the planner can expand
their children. The checks are defensive-only, but they consume lines and
add branches.

**Easy wins (pure extraction, no behaviour change):**
- `checkShouldSkip(step, ec) (shouldSkip bool, reason string, err error)`
- `dispatchPlanMode(step, ec, handler) error` — collapses the two
  near-identical plan-mode blocks.
- `postExecuteSuccess(step, ec, result)` — record + emit + register +
  capture + cleanup.

**Larger issue:**
- Compound-step semantics (Try, Transaction, on_change) are partially
  re-implemented in executor: the planner expands children with
  `TryParent`/`TxnParent` set; the executor re-checks those fields for
  skip logic. This is correct but duplicative. Ideal: planner emits
  children with all gating pre-applied; executor is a dumb runner.
  Would require planner to know error outcomes at plan-time — out of
  scope until a future spec.

---

## 4. `internal/mcp/tools.go` — architectural gap

**File:** `internal/mcp/tools.go` (530 lines)

**Diagnosis:** `runConfig()` (line 440) calls `executor.ExecutePlan`
directly. This means MCP:
- Wires publisher + event sink by hand (lines 432–437).
- Collects results via a custom `runCollector` struct (lines 232–296).
- Manually reconstructs the result map (lines 450–475).

`apply.Runner` already does all of this. MCP is reimplementing the kernel
apply surface that `apply.Runner.Run()` was designed to replace.

This is the **last ✗ criterion** from the refactor-plan success checklist:
`internal/mcp imports internal/apply directly`.

**Easy win (medium effort, high value):**

Replace the body of `runConfig()`:

```go
// Before
planner, _ := plan.NewPlanner()
planData, _ := planner.BuildPlan(...)
// hand-wire publisher, sink, collector...
runErr := executor.ExecutePlan(planData, ...)
// reconstruct result map manually

// After
runner, err := apply.NewRunnerFromPlan(planData, log, publisher)
if err != nil { ... }
runErr := runner.Run(ctx)
result := runner.Result() // *apply.KernelResult
```

- Deletes `runCollector` (64 lines), manual event subscription, manual
  result reconstruction.
- MCP becomes a thin JSON-over-wire wrapper, not an execution orchestrator.

**Note:** `apply.NewRunnerFromPlan` signature must be verified before
implementing. May need a small API addition if it doesn't exist yet.

---

## 5. `internal/agentd/worker.go` — same problem as MCP

**File:** `internal/agentd/worker.go` (313 lines)

**Diagnosis:** `executeRun()` (lines 102–214) calls `executor.Start`
directly (line 178). Worker hand-wires:
- `RunEventSink` subscription (lines 142–154).
- Publisher creation and flush (lines 150–154, 189–191).
- Manual result assembly from `RunCapture` + `daemonSummarySink`
  (lines 228–264).

After R2.1c, the daemon now writes `result.json`. But it assembles
`*apply.KernelResult` by hand instead of getting it from `apply.Runner`.

**Easy win:**
Replace `executor.Start` call with `apply.Runner`. Worker becomes:
setup → call runner → persist `runner.Result()` → update run status.
Deletes ~50 lines in `executeRun` and simplifies `writeResult`.

**Blocked — needs API addition.** Worker subscribes `RunEventSink` (SSE
hub + `events.jsonl`) and `daemonSummarySink` to the publisher before
calling `executor.Start`. `apply.NewRunner` creates its publisher
internally with no way to inject extra subscribers. Fix requires adding
`Subscribers []events.Subscriber` to `apply.Config`. Until that lands,
worker stays on `executor.Start` — the manual result assembly in
`writeResult` is redundant but harmless.

---

## 6. `internal/config/config.go` — structural debt

**File:** 1,866 lines. `Step` struct: ~220 lines (lines 1291–1510+).

**Diagnosis:**
- 74 optional action pointers on `Step` — exactly-one constraint is
  runtime-only (`step.Validate()`). No compile-time union.
- Planner-populated fields (`TryParent`, `TxnParent`, `TriggeredBy`,
  `LoopContext`, etc.) live on the same struct as user-writable fields.
  Step is both user input and internal state.
- Compound-step fields (`Transaction`, `Try`, `Catch`, `Finally`,
  `OnChange`, `OnRollback`) coexist with action fields. A step with
  both `action: shell` and `transaction: [...]` is valid Go but invalid
  config.

**Easy win (no code change):**
- Document the invariants at the top of the `Step` struct comment.
- Add a comment block distinguishing: user-input fields / planner-internal
  fields / compound-step containers.

**Medium win:**
- Create `ActionType` enum + replace the 74 action pointer fields with
  a single `ActionData interface{}`. Enforces exactly-one at struct
  level (only one field). Mechanical but large (all handler field access
  changes from `step.FileCopy.Src` to typed assertion on ActionData).

**Larger issue:**
- Separating `Step` (user input) from `StepPlan` (planner-expanded, with
  internal fields) is the right long-term shape. Would clean up executor's
  knowledge of `TryParent`/`TxnParent`. Large refactor; needs its own spec.

---

## 7. `internal/actions/file/handler.go` — Spec-16 incomplete

**File:** 1,210 lines (2,044 counting test file).

**Diagnosis:** Same dual-path smell as `copy`. `Execute()` (lines 160–216)
and `Run()` (lines 878–963) are ~70% duplicate. State handlers
(`createDirectory`, `touchFile`, `createSymlink`) are implemented twice:
once for the legacy path (direct `os.*` calls) and once for Spec-16
(through `ctx.Effects()`). Event emission is synchronous in the legacy
path, reconstructed from `Effect` in the Spec-16 path — timestamps differ.

**Easy win:** Delete `Execute()` and `DryRun()`. Same prescription as `copy`.

**Larger issue:** `Performer` interface and legacy direct `os.*` calls are
permanently divergent. Choosing one is an architectural decision, not a
refactor. Spec-16 path (`ctx.Effects()`) wins — it centralises system
calls for dry-run/observability. Legacy path should be deleted once all
callers are migrated.

---

## 8. `internal/register/register.go` — not a smell

**File:** 77 lines, 62 blank imports.

**Diagnosis:** Correct design for Go's circular-import constraint.
High instability (0.98) is expected — register is the fan-out leaf.

**Cosmetic improvement only:** Organize imports into named sections
(file management, text, os, pkg, network, etc.) for scannability.
No architectural change needed.

---

## Priority order for easy wins

| # | File | Change | Effort | Value | Status |
|---|---|---|---|---|---|
| 1 | `internal/mcp/tools.go` | Replace `executor.ExecutePlan` with `apply.Runner` | M | High — closes last ✗ criterion | **DONE** |
| 2 | `internal/agentd/worker.go` | Replace `executor.Start` with `apply.Runner` | M | High — consistent kernel boundary | **DONE** |
| 3 | `internal/actions/copy/handler.go` | Delete `Execute`/`DryRun`, unify through `Run` | S | Medium — removes 196 lines of duplication | **DONE** |
| 4 | `internal/executor/executor.go` | Extract `checkShouldSkip`, `dispatchPlanMode`, `postExecuteSuccess` | S | Medium — clarity, no behaviour change | **DONE** |
| 5 | `internal/actions/file/handler.go` | Delete `Execute`/`DryRun` | S | Medium — same as copy | **DONE** |
| 6 | `internal/explain/explain.go` | Extract table/CPU-flag helpers | XS | Low — cosmetic only | **DONE** |
| 7 | `internal/register/register.go` | Section comments on imports | XS | Low — cosmetic only | **DONE** |
| 8 | `internal/config/config.go` | Document struct invariants | XS | Low — pays forward | **DONE** |

## Completed (2026-05-16, worktree-arch-wins)

All 8 items shipped. All tests pass (105 packages, 0 failures).

### Side-effects fixed during copy migration

- **`apply.Config.ExtraSubscribers`** added — enables agentd to inject daemon-specific
  subscribers (SSE hub, events.jsonl) into the kernel without bypassing the apply boundary.
- **`apply.Config.Names`** added — wires spec-50 step-name filter through apply.Runner.
- **`apply.Runner` ExtraSubscribers lifecycle** fixed — runner now calls `sub.Close()` after
  `publisher.Flush()` so async-writing subscribers (RunEventSink's writeLoop) flush before
  `Run()` returns. Previously the publisher closed channels but never called subscriber.Close().
- **Atomic `writeKernelResult`** in agentd worker — tmp-file + rename eliminates the
  race between hub.Close() (stream termination) and result.json appearing on disk. The old
  `os.WriteFile` had a truncation window that made the JSON decode fail non-deterministically.
- **`copy.Run()` symlink handling** added — `FollowSymlinks=false` path was missing from the
  Spec-16 `Run` method. Added `os.Lstat` + `ctx.Effects().Symlink()` branch so symlink sources
  are preserved instead of dereferenced.
- **`observe_disk` macOS build fix** — `int64(st.Bsize)` cast resolves int32×int64 mismatch.

### Round 2 (executor + file + config)

- **`executor.dispatchPlanMode`** extracted — collapses the two near-identical plan-mode
  dispatch blocks in `ExecuteStep` (Runner fast-path + unknown-action fallback) into one helper.
- **`executor.postExecuteSuccess`** extracted — the stats/emit/ChangedByStepID/txn/capture
  block at end of `ExecuteStep` becomes a named helper, reducing the function body by ~55 lines.
- **`file.Execute` / `file.DryRun` deleted** — same prescription as copy. Tests migrated from
  `h.Execute(` to `h.Run(`; `TestHandler_DryRun` series replaced with `TestHandler_PlanMode`
  and `TestDefaultModeFor`.
- **`config.Step` struct invariants documented** — three-section comment block at struct
  header (USER-INPUT / COMPOUND-STEP CONTAINERS / PLANNER-INTERNAL) plus inline section
  markers and exactly-one note on the action fields.

## Open items

None. All planned easy wins shipped.
