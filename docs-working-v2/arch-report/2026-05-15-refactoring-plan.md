# Refactoring Plan — Structured Execution

**Date:** 2026-05-15
**Source:** [`2026-05-15-arch-report.md`](./2026-05-15-arch-report.md)
**Anchor:** [`../vision/kernel.md`](../vision/kernel.md)
**Status:** Ready for execution. All bug-fix work paused; this is the
next phase.

This plan turns the arch report's recommendations into discrete,
independently-shippable items. Each item is sized for **one agent /
one worktree / one PR**. Items within the same phase are
parallel-safe. Phases are gated on the prior phase landing.

---

## 0. Premise

**What this plan actually does: expose the kernel.**

The kernel — typed mutation with Diff / Reverse / Cost / Permissions
on every node — already exists, scattered across
`internal/{plan, executor, actions, effects, runlog, events, facts}`.
See [`vision/kernel.md`](../vision/kernel.md) for the canonical
positioning.

What's missing is the **exported entry-point surface**. Today, every
frontend (CLI, MCP server, agent loop, agentd, fleet) re-implements
parts of the kernel's orchestration because there's no `Kernel.Apply()`
to call. `fleetApplyAction` in `cmd/fleet.go` is the kernel's "apply
to many peers" entry point trapped inside a CLI handler. The MCP
server does it differently. The agent loop does it differently again.

The arch report identified four structural pressure points; this
plan reads them through the kernel lens:

| Pressure point | Kernel lens |
|---|---|
| 1. `cmd/` carries application services | Kernel entry points trapped in CLI handlers |
| 2. `internal/executor` accreting eight runtime concerns | Kernel sub-systems unexported, all glued to the dispatcher |
| 3. `internal/config/config.go` monotonic growth (*watch only*) | The closed action set — paying the cost is what makes the kernel typed end-to-end |
| 4. Handler proliferation tail (*watch only*) | The kernel grows by adding handlers; the soft cap (1500 LOC) keeps it tractable |

Pressure points 3 and 4 are explicitly **don't-act**. This plan
addresses 1 and 2 — by name, they're the **frontend duplication
problem** and the **unexported kernel sub-system problem**.

**Decision on ChangeGraph (locked):** ChangeGraph is a **derived
view, not a new package**. The plan stays `[]Step` with implicit
edges (`TriggeredBy`, `Try/Catch/Finally`, `Transaction`,
`Reverse()`); future consumers (`explain`, `plan --format graph`,
`rewind`) materialize per-consumer views from those existing fields.
This plan does **not** introduce an `internal/changegraph` package.
See [§8 Anti-scope](#8-anti-scope--what-this-plan-does-not-do).

The execution principle: **prove the pattern with the smallest,
lowest-risk extraction first** (Phase 0 + 1), then propagate
(Phase 2). Each move is a pure relocation — every test that passes
before passes after — but the *exported names* now describe the
kernel rather than the CLI handler the code happened to grow up in.

---

## 1. Plan at a glance

| Phase | Item | Title | Effort | Risk | Review type | Depends on |
|---|---|---|---|---|---|---|
| 0 | R0.1 | Promote `transaction` + `trycatch` → `internal/control` | S | low | mechanical | — |
| 0 | R0.2 | Move `tag_check` → `internal/plan/filter` | S | low | mechanical | — |
| 0 | R0.3 | Promote `secret_resolve` → `internal/secrets/resolver` | S | low | mechanical | — |
| 0 | R0.4 | Document soft caps in `CONTRIBUTING.md` / `LLM_GUIDE.md` | XS | none | doc | — |
| 1 | **R1.1a** | Pure extraction: `internal/apply.Runner` shim over `cmd.run` (flat `(*RunResult, error)` return) | S | low | mechanical | R0.1, R0.3 |
| 1 | **R1.1b** | Crystallize `KernelResult` typed return + `Reverse()` method on `apply.Runner` | S–M | medium | **API surface** | R1.1a |
| 2 | **R2.1a** | Pure extraction: `internal/fleet.Orchestrator` shim over `fleetApplyAction` (flat `(RunSummary, error)` return) | M | medium | mechanical | R1.1a |
| 2 | **R2.1b** | Compose `FleetKernelResult` from per-peer `apply.KernelResult` + per-peer `Reverse()` | S | low | **API surface** | R1.1b, R2.1a |
| 3 | R3.1 | Rename `internal/registry` → `internal/presets/registry` | S | low | mechanical | — |

Effort scale: XS = doc only; S = ~1 day, single file moves;
M = ~2–3 days, single PR; L = ~3–5 days. After the R1.1 / R2.1 splits
no remaining item is L.

Review type:
- **mechanical** = "did the code move correctly?" — 15-min review.
- **API surface** = "is the typed return shape right? does `Reverse()`
  handle edge case X?" — deeper review, reviewer needs `vision/kernel.md`
  context.
- **doc** = soft caps numbers + rationale; fast review.

The split between R1.1a/R1.1b and R2.1a/R2.1b separates *mechanical
relocation* (pure code move, no semantic change) from *kernel-surface
crystallization* (typed return shapes, `Reverse()` semantics). Two
benefits: (1) mechanical PRs merge fast on any reviewer; (2) the API
PRs get focused review without being bundled with code moves that
require no argument.

**Watch-only / deferred items** are listed in §5 — *not actionable
without a trigger event*.

---

## 2. Phase 0 — Foundation moves (parallel-safe)

These four items can run in parallel. They are pure internal
relocations with no external API change, no behavior change, and no
cross-item coordination. Doing them first **establishes the
"executor as controller, not layer" pattern** and warms up the
worktree workflow before the bigger cmd extractions.

### R0.1 — Promote `transaction.go` + `trycatch.go` → `internal/control`

**Source:** arch report §3.2, recommendation 2.

**Motivation:** Compound-step state machines (transaction LIFO
rollback, try/catch/finally branch routing) are kernel primitives —
they describe **graph-shape concepts** that any future ChangeGraph
view will surface (grouping subgraph for `transaction`, branch edges
for `try/catch`). Today they sit inside `internal/executor/` for
historical reasons, glued to `ExecuteStep`. Exposing them as
`internal/control` is the first kernel sub-system promotion:
frontends that want to reason about compound steps (drift
auto-remediation, the future `apply_approved` MCP two-phase apply,
`explain` walking from a failed transaction back to its body) call
into `internal/control` directly, not into the executor's internals.

**Files touched:**
- Move `internal/executor/transaction.go` → `internal/control/transaction.go`
- Move `internal/executor/trycatch.go` → `internal/control/trycatch.go`
- Move associated tests (`executor_transaction_test.go`,
  `executor_trycatch_test.go`) → `internal/control/`
- Adjust imports in `internal/executor/executor.go` to call into
  `internal/control` for the compound-step branches
- Add `internal/control/doc.go` with package-level rationale

**DONE when:**
1. `go test ./...` passes
2. `grep -rn "ec.txnSkipReason\|ec.trySkipReason\|recordTxnBodyCompletion" internal/executor/` returns no results outside `executor.go`'s call sites (the bodies have moved).
3. `internal/executor` package non-test LOC drops by ≥300.
4. `internal/control` package exists with ≥2 files and a doc.go.
5. No new public API on `internal/executor` (the move is internal-to-internal).
6. `scripts/arch-snapshot.sh` shows `internal/control` at instability 0–0.3 (it imports executor types but is imported only by executor).

**Blast radius:** small — single-package internal-only churn. No
handler changes, no CLI changes, no agentd changes.

**Risk:** low. The package boundary already exists conceptually
(separate files, separate test files); the move just makes it a
compile-time boundary.

**Dependencies:** none.

**Open question:** there is *circular-import risk* — `internal/control`
will need types from `internal/executor` (ExecutionContext, Result),
and executor needs to call into control. Resolve by **moving the
control-flow types into control** and having executor import control,
not the reverse. If that's not possible without a deeper restructure,
land R0.1 as a documented partial (interfaces only in control,
implementations stay in executor) and file the rest as a follow-up.

---

### R0.2 — Move `tag_check.go` → `internal/plan/filter`

**Source:** arch report §3.2 table; recommendation 3.

**Motivation:** Tag filtering is a *plan-time* decision per spec-32 —
the planner already sets `step.Skipped=true` for tag-filtered steps,
and the executor "trusts the planner's decision" (verbatim from
`executor.go:351`). The fact that `tag_check.go` lives in `executor/`
is purely historical. Moving it is the kind of small alignment that
compounds.

**Files touched:**
- Move `internal/executor/tag_check.go` → `internal/plan/filter/tags.go`
- Move `internal/executor/tag_check_test.go` → `internal/plan/filter/tags_test.go`
- Update planner call sites (currently they import `internal/executor`
  just for this; should import `internal/plan/filter` instead)
- Verify no `tag_check`-named symbol remains in executor

**DONE when:**
1. `go test ./...` passes.
2. `grep -rn "MatchesSkipTags\|ShouldSkipByTags\|matchesTagFilter" internal/executor/` returns no results.
3. The MatchesSkipTags helper added by fix-mt-58 lives in `internal/plan/filter`, called from `internal/plan/planner.go`.

**Blast radius:** small. The function is called from planner (cross-
package) and was a "leaked" executor responsibility.

**Risk:** very low.

**Dependencies:** none. Can run concurrently with R0.1.

---

### R0.3 — Promote `secret_resolve.go` → `internal/secrets/resolver`

**Source:** arch report §3.2 table.

**Motivation:** `!secret` typed-ref resolution is the kernel's
**pre-execute walk** — a pure traversal over the typed plan that
resolves typed refs against the configured providers. Frontends
that want to pre-process a plan before submission (MCP `check_plan`,
agent loop's "validate before propose," future `apply_approved`
hash-then-sign flow) need this walk callable without dragging in
3,270 LOC of executor. Promoting it to `internal/secrets/resolver`
exposes it as a kernel service: input typed plan + provider set →
output typed plan with refs resolved.

**Files touched:**
- Move `internal/executor/secret_resolve.go` → `internal/secrets/resolver/resolve.go`
- Move `internal/executor/secret_resolve_test.go` → `internal/secrets/resolver/resolve_test.go`
- Adjust import in executor's call site (probably `executor.go` or a
  preflight stage)
- Add `internal/secrets/resolver/doc.go`

**DONE when:**
1. `go test ./...` passes.
2. `walkAndResolveSecrets` (gocyclo 23 in executor today) lives in
   `internal/secrets/resolver`.
3. Executor's call site is a single-line call into the new package.
4. No package other than executor / mcp / agent imports
   `internal/secrets/resolver` yet (we're not opportunistically
   widening usage — just relocating).

**Blast radius:** small.

**Risk:** low. The function is well-bounded — pure walk of the step
tree with no state.

**Dependencies:** none.

---

### R0.4 — Document soft caps

**Source:** arch report recommendation 5.

**Motivation:** The report identifies three soft caps that should be
review-time prompts, not CI gates. Writing them down makes the
architecture self-policing as the project grows. Reviewers (human or
AI) get a checklist they can apply without reading the arch report
each time.

**Files touched:**
- Append a new section to `CLAUDE.md` and `LLM_GUIDE.md`:
  - **Handler LOC > 1,500** → split per-OS sub-packages OR split into
    sibling action types
  - **`internal/config` universal Step field count > 40** → flag for
    review; should anyone really need this on every step?
  - **gocyclo > 35** in any non-test function → refactor when next
    touched
- (Optional) `scripts/check-arch-caps.sh` that prints which functions
  / handlers / fields are over the caps. *Not* a pre-commit hook —
  this is a soft signal, not a gate.

**DONE when:**
1. Soft-caps section exists in both CLAUDE.md and LLM_GUIDE.md with
   the three caps and one-sentence rationale per cap.
2. (If implemented) `scripts/check-arch-caps.sh` runs and reports
   current violations. Today's known violations:
   - `internal/actions/file` 2,044 LOC
   - `internal/actions/tool` 1,676 LOC
   - `internal/actions/service` 1,466 LOC (just under)
   - `copy.Execute` gocyclo 41
   - `os_systemd.computePlan` gocyclo 34 (just under)
   - `fleetApplyAction` gocyclo 49 (will be fixed by R2.1)

**Blast radius:** zero. Doc-only.

**Risk:** none.

**Dependencies:** none.

---

## 3. Phase 1 — Expose the kernel's `Apply()` entry point

R1.1 splits into a pure-extraction PR (R1.1a) and a kernel-surface
PR (R1.1b). The split separates *mechanical relocation* from *API
design decision*: R1.1a is reviewable in 15 minutes ("did the code
move correctly?"); R1.1b is the deeper review ("is `KernelResult` the
right shape? does `Reverse()` handle edge case X?").

### R1.1a — Pure extraction: `internal/apply.Runner` shim over `cmd.run`

**Source:** arch report §3.1, recommendation 4. Split out from
original R1.1.

**Motivation:** `cmd.run` (`cmd/mooncake.go:236`, gocyclo **33**) is
the kernel's `Apply()` entry point trapped inside a CLI handler.
This PR is the **code-move-only** half: lift the body out, leave
the return shape flat (matching today's behavior), `cmd.run` becomes
a shim. No new typed API; no `Reverse()` method. That's R1.1b.

**Goal:** turn `cmd.run` into a thin shim that builds a
`*apply.Config` from CLI flags and calls `apply.NewRunner(cfg).Run()`.
Return shape stays whatever the current `cmd.run` body produces
(typically `error`, possibly with a `*RunResult` carrying the recap).

**Files touched:**

New:
- `internal/apply/runner.go` — `Runner` struct + `Run()` method.
  Body lifted verbatim from `cmd.run`.
- `internal/apply/config.go` — `Config` struct (the fields that
  `cmd.run` accepts as CLI flags).
- `internal/apply/runner_test.go` — at least one apply-path
  integration test that exercises the new entry point on an
  `examples/...` plan.
- `internal/apply/doc.go` — package doc that cites kernel.md as the
  rationale for the package's existence.

Modified:
- `cmd/mooncake.go` — `run` collapses to ~50 LOC of flag parse →
  `apply.Config` construction → `Runner.Run()` call → recap render.
- `cmd/cmd_test.go` — relevant tests retargeted to the new API
  where cheaper. Test set should otherwise be identical.

**DONE when:**
1. `go test ./...` passes.
2. `cmd.run` gocyclo is ≤ 10 (currently 33). Measured via
   `gocyclo cmd/mooncake.go`.
3. `cmd/mooncake.go` LOC drops by ≥ 300.
4. `internal/apply/runner.go` exists with `Runner.Run(ctx) error`
   (or `(*RunResult, error)` — whatever shape the lift naturally
   produces; do **not** introduce `*KernelResult` here, that's
   R1.1b).
5. End-to-end manual: `mooncake apply -c examples/...` produces
   identical output (recap, exit code, audit events) before and after.

**Blast radius:** medium. Touches the largest file in `cmd/` and the
most-used CLI path. But no semantic change — every test that passed
before passes after.

**Risk:** low (mechanical). Three sub-risks:
- (a) Hidden side effects in `cmd.run` (logging setup, env-var
  reading) get missed during extraction. *Mitigation:* run the full
  cmd_test.go suite locally before/after; diff stdout/stderr from a
  scripted apply on `examples/` plans.
- (b) The shape of `apply.Config` doesn't accommodate fleet-apply.
  *Mitigation:* don't try to. R1.1a covers local apply only; fleet
  apply is R2.1a's problem and uses its own orchestrator that may
  internally call `apply.Runner` per peer.
- (c) Circular import. *Mitigation:* run `go list -e ./...` after
  the move; cycles surface immediately.

**Dependencies:**
- R0.1 (control package) — soft; if `transaction` has moved to
  `internal/control`, `apply.Runner` calls into it. If R0.1 slipped,
  it calls into the in-executor code.
- R0.3 (secrets resolver) — soft; `apply.Runner` invokes the
  resolver as a pre-execute walk.

If R0.1 / R0.3 land first, R1.1a is cleaner. If they slip, R1.1a
still works but imports the unmoved files.

---

### R1.1b — Crystallize `KernelResult` typed return + `Reverse()` method

**Source:** kernel discussion (Option 2 locked); follows R1.1a.

**Motivation:** R1.1a left `apply.Runner` returning a flat shape.
R1.1b crystallizes the kernel-surface contract from
[`vision/kernel.md`](../vision/kernel.md): a typed `*KernelResult`
carrying plan + per-step results + audit substrate + a `Reverse()`
method, so downstream frontends (explain, rewind, MCP
`apply_approved`, agent loop's "undo last action") don't re-derive
any of it.

**API contract (locked decision):**

```go
package apply

// Run now returns the typed KernelResult.
func (r *Runner) Run(ctx context.Context) (*KernelResult, error)

// KernelResult is the kernel's typed "what just happened" shape.
// Input to Reverse, Explain, and MCP/SDK consumers without further
// re-derivation.
type KernelResult struct {
    Plan    *plan.Plan          // the typed plan that was executed
    Steps   []executor.StepResult // per-step outcome (changed/failed/skipped/diff/cost/etc)
    Events  []events.Event      // audit substrate — the run's event tail
    Summary RunSummary          // ok/changed/skipped/failed counts + duration
}

// Reverse builds the inverse plan from this run's reversible subset.
// Returns the empty plan if nothing was reversible.
func (r *KernelResult) Reverse() (*plan.Plan, error)
```

The `Reverse()` method is the load-bearing add. Today, building a
reverse plan from a completed run is **not exposed as a kernel
operation** — `transaction:` blocks do it in-process; nothing else
can. Adding it here unlocks (a) cross-run rewind, (b) MCP rollback
tools, (c) the agent loop's "undo your last action" — all from one
function.

**Files touched:**

New:
- `internal/apply/result.go` — `KernelResult` + `Reverse()` + `RunSummary`.

Modified:
- `internal/apply/runner.go` — `Run()` signature changes; body
  builds a `*KernelResult` instead of returning the flat shape.
- `internal/apply/runner_test.go` — adds a Reverse-coverage test.
- `cmd/mooncake.go` — recap renderer now reads `KernelResult.Summary`
  instead of the flat shape.

**DONE when:**
1. `go test ./...` passes.
2. `Runner.Run` signature is `(ctx context.Context) (*KernelResult, error)`.
3. `KernelResult.Reverse()` is covered by a test that:
   - runs a plan with at least one reversible step (e.g. `file.write`)
   - calls `Reverse()` on the result
   - asserts the returned plan would restore pre-state if executed
4. `internal/mcp` package can import `internal/apply` and call
   `Runner.Run` — verified by a single test in `internal/mcp` that
   constructs a `Runner` (doesn't have to actually execute; just
   prove the import works without circular deps + the returned
   `*KernelResult` has the documented fields).
5. End-to-end manual: `mooncake apply -c examples/...` produces
   identical output (recap, exit code, audit events) before and after.

**Blast radius:** small in code (single new file, two modified). The
API shape is the substantive change.

**Risk:** medium (API design). One sub-risk:
- (d) **`KernelResult.Reverse()` semantics drift.** The method
  has to handle: reversible-step subset (some handlers refuse
  Reverse), transaction-boundary preservation (don't reverse across
  transaction edges in the wrong order), already-reversed steps
  (don't double-reverse). *Mitigation:* the implementation is **lift
  the existing transaction-rollback walker** from `internal/control/`
  (post R0.1) or `internal/executor/transaction.go` (pre R0.1) into
  a generic "build inverse plan from N step results" helper. Same
  algorithm, different input shape. Do not invent new Reverse
  semantics here.

**Dependencies:**
- R1.1a — hard. R1.1b modifies `apply.Runner.Run`; the package must
  exist first.
- R0.1 (control package) — soft; cleaner Reverse-walker lift if the
  transaction code has moved to `internal/control`.

---

## 4. Phase 2 — Expose the kernel's `FleetApply()` entry point

R2.1 splits the same way R1.1 did: mechanical extraction (R2.1a)
followed by kernel-surface crystallization (R2.1b). The split is
**stronger here** than for R1.1 — `fleetApplyAction` is gocyclo 49
and the mechanical move alone is M-effort; bundling the typed
kernel surface on top makes the PR unreviewable.

### R2.1a — Pure extraction: `internal/fleet.Orchestrator` shim over `fleetApplyAction`

**Source:** arch report §3.1 table; recommendation 1. Split out from
original R2.1.

**Motivation:** `fleetApplyAction` (`cmd/fleet.go:336`) at gocyclo
**49** is the kernel's `FleetApply()` entry point trapped inside a
CLI handler. Six responsibilities: peer filtering, plan-snapshot
upload, ordered phase rollout, per-peer SSE fan-in, recap
aggregation, exit-code computation. This PR is the **code-move-only**
half: each responsibility becomes a method on `Orchestrator`,
`Run()` returns a flat shape matching today's behavior. No typed
`FleetKernelResult` yet — that's R2.1b.

**Goal:** turn `fleetApplyAction` into a thin shim that builds a
`*fleet.Config` from CLI flags and calls
`fleet.NewOrchestrator(cfg).Run()`. Six private methods on
`Orchestrator` factor the responsibilities cleanly.

**Files touched:**

New:
- `internal/fleet/orchestrator.go` — `Orchestrator` struct + methods:
  - `(o *Orchestrator) FilterPeers(ctx) ([]Peer, error)`
  - `(o *Orchestrator) UploadPlan(ctx, peer) error`
  - `(o *Orchestrator) ApplyToPhase(ctx, phase []Peer) ([]PeerResult, error)`
  - `(o *Orchestrator) Run(ctx) (RunSummary, error)` — top-level
    entry point. Flat return; matches today's shape.
- `internal/fleet/config.go` — `Config` struct (the fields that
  `fleetApplyAction` accepts as CLI flags).
- `internal/fleet/orchestrator_test.go` — lock down the exit-code
  matrix first (partial-success / any-fail / all-skip), then test
  each method.

Modified:
- `cmd/fleet.go` — `fleetApplyAction` collapses to flag parse +
  `Orchestrator.Run()` + recap render.
- Probably also touches `cmd/fleet_filter_test.go`, retargeted to
  test the new package.

**DONE when:**
1. `go test ./...` passes.
2. `fleetApplyAction` gocyclo is ≤ 10 (currently 49). Measured via
   `gocyclo cmd/fleet.go`.
3. `cmd/fleet.go` LOC drops by ≥ 500.
4. `internal/fleet/orchestrator.go` exists with the six methods.
5. Exit-code matrix tests pass (all-success → 0, any-fail → 1,
   partial-success per current rules).
6. Manual verification: `mooncake fleet apply -p tag=linux examples/...`
   produces identical per-peer output, identical RECAP, identical
   exit code before/after.

**Blast radius:** large within `cmd/fleet.go`, zero elsewhere.

**Risk:** medium (mechanical move of complex code). Three sub-risks:
- (a) Hidden side effects in `fleetApplyAction`. *Mitigation:* full
  cmd_test.go suite locally before/after.
- (b) Per-peer SSE fan-in has timing semantics (concurrent streams,
  per-peer phasing). *Mitigation:* preserve the goroutine topology
  byte-for-byte. The extraction is a code-shape change, not a
  concurrency-model change.
- (c) Error aggregation across peers — subtle exit-code logic
  (any-fail → nonzero, partial-success rules). *Mitigation:* lock
  the exit-code matrix down as the *first* test in
  `orchestrator_test.go`, then refactor with that suite in place.

**Dependencies:**
- R1.1a — hard. The pattern from R1.1a (cmd-handler-becomes-shim,
  orchestrator-owns-body) is what R2.1a copies. Trying R2.1a first
  would mean inventing the pattern on the project's most complex
  function.

---

### R2.1b — Compose `FleetKernelResult` from per-peer `apply.KernelResult`

**Source:** kernel discussion (Option 2 locked); follows R2.1a +
R1.1b.

**Motivation:** R2.1a left `Orchestrator.Run` with a flat return.
R2.1b crystallizes the fleet-scope kernel surface: each peer's
result is an `apply.KernelResult` (from R1.1b); `FleetKernelResult`
maps peer → result and composes `Reverse()` from per-peer
`KernelResult.Reverse()` calls.

This PR is **small** because the shape composes. R1.1b did the
hard work of typing the per-peer kernel result; R2.1b lifts it one
level up.

**API contract (parallel to R1.1b, same locked decision):**

```go
package fleet

// Run signature changes from R2.1a.
func (o *Orchestrator) Run(ctx context.Context) (*FleetKernelResult, error)

// FleetKernelResult is the fleet-scope analog of apply.KernelResult.
// Input to fleet why, fleet drift, MCP fleet tools without
// re-derivation.
type FleetKernelResult struct {
    Plan    *plan.Plan                       // the plan that was dispatched
    Peers   map[PeerID]*apply.KernelResult   // per-peer outcome
    Summary FleetSummary                      // aggregate counts + per-peer status
}

// Reverse builds an inverse FleetPlan by calling peer.Reverse() on
// each entry in Peers and assembling the results.
func (r *FleetKernelResult) Reverse() (*FleetPlan, error)
```

**Files touched:**

New:
- `internal/fleet/result.go` — `FleetKernelResult` + `Reverse()` +
  `FleetSummary` + `FleetPlan`.

Modified:
- `internal/fleet/orchestrator.go` — `Run()` signature changes;
  body builds a `*FleetKernelResult` by collecting per-peer
  `apply.KernelResult`s from the existing `ApplyToPhase` flow.
- `internal/fleet/orchestrator_test.go` — adds a per-peer
  `Reverse()` coverage test.
- `cmd/fleet.go` — recap renderer reads `FleetKernelResult.Summary`.

**DONE when:**
1. `go test ./...` passes.
2. `Orchestrator.Run` signature is
   `(ctx context.Context) (*FleetKernelResult, error)`.
3. `FleetKernelResult.Reverse()` is covered by a test that:
   - runs a plan against ≥2 peers with reversible steps on each
   - calls `Reverse()` on the result
   - asserts the returned FleetPlan would restore pre-state per peer
4. `internal/mcp` can call `fleet.NewOrchestrator(cfg).Run(ctx)`
   and inspect the returned `*FleetKernelResult` — verified by a
   smoke test in `internal/mcp` that constructs an Orchestrator
   (does not execute).
5. Manual verification: `mooncake fleet apply -p tag=linux examples/...`
   output unchanged.

**Blast radius:** small in code. The fleet-scope API contract is the
substantive change.

**Risk:** low (API design is mechanical given R1.1b is in place). One
sub-risk:
- (d) **`PeerID` shape choice.** `map[PeerID]` requires `PeerID` to
  be a stable comparable type. *Mitigation:* use the existing
  `fleet.Peer.Name` (string) as `PeerID`; don't invent a new type.

**Dependencies:**
- R2.1a — hard. Orchestrator must exist before its return type can
  change.
- R1.1b — hard. `FleetKernelResult` composes `apply.KernelResult`;
  the latter must exist first.

---

## 5. Phase 3 — Housekeeping

### R3.1 — Rename `internal/registry` → `internal/presets/registry`

**Source:** arch report §4.2.

**Motivation:** Two packages with sibling-similar names but entirely
different domains:
- `internal/registry` — **preset** registry (`mooncake presets list/
  install/recommend`)
- `internal/register` — **action handler** registry (the side-
  effect-imports pattern)

The names confuse new contributors and AI agents (verified — has come
up in multiple sub-agent runs). Renaming is cheap and improves
search-grep usability.

**Files touched:**
- Move `internal/registry/*` → `internal/presets/registry/*`
- Update all importers (~1 — only `cmd/presets.go` and tests)
- `go vet ./...` and `gofmt` after the move

**DONE when:**
1. `go test ./...` passes.
2. `grep -rn "alehatsman/mooncake/internal/registry" .` returns no
   results outside the moved package.
3. `grep -rn "alehatsman/mooncake/internal/register\b" .` still
   resolves only to the action-handler register package (the renames
   are not accidentally collided).

**Blast radius:** very small (cut: 1 importer per goda).

**Risk:** very low.

**Dependencies:** none. Can run any time, including in parallel with
any other phase.

---

## 6. Watch-only / deferred (no action without trigger)

These are explicitly *not* in the plan. They're listed so future
reviewers don't re-discover them.

| Item | Trigger that promotes it to action |
|---|---|
| **W.1** — Split `internal/agentd` HTTP routing from business logic | A second transport gets proposed (gRPC, unix-socket-IPC for SDK, anything non-HTTP). At that point the HTTP-bound orchestration becomes the bottleneck. |
| **W.2** — Strengthen `internal/effects` boundary | A third effect kind gets proposed (Windows-elevated, remote-via-agentd). Today's two callers (file handler with two paths) don't justify the refactor. |
| **W.3** — Handler size hard cap | Soft cap from R0.4 is violated repeatedly across reviews. Currently 3 handlers over the cap; no compounding pattern yet. |
| **W.4** — `internal/config.Step` universal-field cap | Universal field count crosses 40 (today ≈ 25). Currently growing ~1 field per sprint. |
| **W.5** — Cross-platform handler per-OS sub-packages | `internal/actions/service` exceeds 2,000 LOC or gocyclo > 50 on any service-related function. |

---

## 7. Sequencing & parallelism

After the R1.1 / R2.1 splits, the plan reads as four sequential
waves with parallelism within each wave:

```
       ┌──────────────────────────────────────────────────────────────┐
       │  Wave 1 — 5 parallel PRs (foundation + housekeeping)         │
       │                                                              │
       │  R0.1    R0.2    R0.3    R0.4    R3.1                        │
       │  control tag    secrets soft    registry                     │
       │          filter         caps    rename                       │
       └─────────────────────────────┬────────────────────────────────┘
                                     │
                                     ▼
       ┌──────────────────────────────────────────────────────────────┐
       │  Wave 2 — single PR (pattern proof, mechanical)              │
       │                                                              │
       │  R1.1a — pure extraction: apply.Runner shim over cmd.run     │
       │          flat return; no KernelResult yet                    │
       └─────────────────────────────┬────────────────────────────────┘
                                     │
                  ┌──────────────────┴──────────────────┐
                  ▼                                     ▼
       ┌──────────────────────────┐    ┌──────────────────────────────┐
       │  Wave 3a (API surface)   │    │  Wave 3b (mechanical, big)   │
       │                          │    │                              │
       │  R1.1b — KernelResult    │    │  R2.1a — fleet.Orchestrator  │
       │          + Reverse()     │    │          shim, flat return   │
       └──────────────┬───────────┘    └──────────────┬───────────────┘
                      │                               │
                      └───────────────┬───────────────┘
                                      ▼
       ┌──────────────────────────────────────────────────────────────┐
       │  Wave 4 — single PR (compose, small)                         │
       │                                                              │
       │  R2.1b — FleetKernelResult composed from per-peer            │
       │          apply.KernelResult; Reverse() composes              │
       └──────────────────────────────────────────────────────────────┘
```

**Wave-by-wave reviewer load:**

| Wave | PRs | Wall-clock | Review type | Reviewer load |
|---|---|---|---|---|
| 1 | 5 (parallel) | 1–2 days | mechanical × 4 + doc × 1 | low — any Go-fluent reviewer |
| 2 | 1 | 1 day | mechanical | low |
| 3 | 2 (parallel) | 1–2 days | API surface + mechanical | one API reviewer + one mechanical reviewer |
| 4 | 1 | 1 day | API surface (small) | low — composes existing shapes |

**Total: 9 PRs, ~5–7 days wall-clock with a small agent fleet.** The
key parallelism win is wave 3: once R1.1a lands, R1.1b (API design
on the local-apply shape) and R2.1a (mechanical fleet extraction)
touch different files and can run concurrently in separate worktrees.

**Recommended cadence for an agent fleet:**

- **Day 1:** five worktrees in parallel for wave 1. All should land
  same-day or next-day given the small blast radius. Review each as
  a 15-minute mechanical check.
- **Day 2:** start wave 2 (R1.1a) in one worktree. Single-agent
  ownership; the pattern proof is the deliverable. Review carefully —
  R2.1a will copy this PR's shape.
- **Day 3–4:** wave 3 in two parallel worktrees. Don't bundle them
  on one agent; they need independent review attention. The API
  PR (R1.1b) is where `KernelResult` field names and `Reverse()`
  semantics get argued; that argument should not be diluted by a
  larger mechanical-move review happening in the same PR.
- **Day 5:** wave 4 (R2.1b). Small follow-on; composes existing
  shapes. Should land same-day.

**If using only one agent at a time:** execute in order
R0.1 → R0.2 → R0.3 → R0.4 → R3.1 → R1.1a → R1.1b → R2.1a → R2.1b.
Total estimated wall-clock: 7–10 days serially. Total estimated LOC
moved: ~3,500. Total estimated LOC of new code (besides moves):
~400 (mostly the two typed result structs + `Reverse()` walker
helper).

---

## 8. Anti-scope — what this plan does NOT do

These came up while drafting and were intentionally excluded:

- **Creating an `internal/changegraph` package.** ChangeGraph is a
  **derived view**, not a new data structure. The implicit edges in
  today's `[]Step` (`TriggeredBy`, `Try/Catch/Finally`, `Transaction`,
  `Reverse()`) are sufficient. Each future consumer (`explain`,
  `plan --format graph`, `rewind`) materializes its own view from
  those fields. Building the typed graph type before three concrete
  uses is the "abstraction first" smell — the kernel framing (see
  [`vision/kernel.md`](../vision/kernel.md)) explicitly bounds this.
- **Splitting `internal/config/config.go`.** The arch report
  explicitly recommends *watch, don't act*. The 1,866 LOC is the
  cost of the closed-action-set bet. Touching it has cascading
  blast radius (76 importers) for no architectural payoff.
- **Refactoring large handlers** (`file`, `tool`, `service`,
  `package`, `os_mount`). Soft cap policy from R0.4 covers the
  question; act when next touched, not preemptively.
- **Adding new interfaces beyond `KernelResult` + `FleetKernelResult`.**
  Those two are typed return shapes at clear kernel seams (R1.1, R2.1),
  not new abstraction layers. No new dependency injection. No plugin
  contracts. No `Kernel` interface that everything implements.
- **Touching `cmd/presets.go`.** It's the second-largest cmd file at
  1,516 LOC, but the report identifies `cmd/fleet.go` (gocyclo 49)
  and `cmd/mooncake.go` (`run` gocyclo 33) as the higher-leverage
  targets. `cmd/presets.go` can wait for its own pass when one of
  its functions exceeds gocyclo 35.
- **Performance work.** Out of scope for a structural plan.
- **Documentation rewrite.** The `docs-next/` rot (~50 example files
  using retired `register:` syntax) is a separate cleanup; not
  blocking on any R-item here.

---

## 9. Success criteria for the whole plan

After all of Phase 0 + 1 + 2 + 3 land:

**Structural metrics:**

- `cmd/` total LOC drops by ≥ 800 (from ~10,547 to ≤ 9,750)
- `cmd/fleet.go` shrinks by ≥ 500 LOC; `fleetApplyAction` gocyclo ≤ 10
- `cmd/mooncake.go` shrinks by ≥ 300 LOC; `cmd.run` gocyclo ≤ 10
- `internal/executor` non-test LOC drops by ≥ 500 (control +
  secrets resolver moved out)
- No new circular imports
- `scripts/arch-snapshot.sh` shows:
  - `internal/executor` non-test source LOC under 2,200
  - `cmd` instability still 1.00 (still no internal imports of cmd)
  - No new package at instability 0.40–0.60 with LOC > 1,500
    (the "mid-band god package" smell to avoid)

**Kernel-surface checkpoint** — the plan's real deliverable beyond
the LOC counts:

- `internal/apply.Runner.Run` exists and returns `*apply.KernelResult`
  with the locked contract from R1.1b (R1.1a ships with the flat
  return shape; R1.1b is what materializes the typed contract).
- `apply.KernelResult.Reverse()` produces an executable inverse
  plan, covered by a test.
- `internal/fleet.Orchestrator.Run` exists and returns
  `*fleet.FleetKernelResult` composed from per-peer
  `apply.KernelResult`s (after R2.1b; R2.1a ships flat).
- `internal/mcp` imports `internal/apply` and `internal/fleet` directly
  for at least one tool path that previously had to shell out to the
  CLI. (Optional for plan completion but strongly preferred as the
  end-of-plan proof.)
- New packages exist: `internal/control`, `internal/secrets/resolver`,
  `internal/plan/filter`, `internal/apply`, `internal/fleet/orchestrator.go`
- `internal/changegraph` does **not** exist (locked Path A decision).

**Behavior preservation:**

- `mooncake apply` and `mooncake fleet apply` behavior unchanged —
  the entire plan is a refactor, not a feature change.

If the structural metrics hold but the kernel-surface checkpoint
doesn't (e.g. R1.1b never lands and `apply.Runner` stays on a flat
`error` return), the plan **regresses** even though the LOC counts
moved. The point of the plan is the kernel surface; the LOC counts
are an artifact.

The split makes this failure mode possible to recognize: if R1.1a
lands and R1.1b stalls in review, the plan is **half-shipped** —
the code is in `internal/apply/` but the kernel-surface promise
isn't kept. Don't start R2.1a in that state; finish R1.1b first.

If any criteria don't hold after wave 3 lands, **stop and re-review**
before starting wave 4. The plan assumes the moves are all pure
relocations + the two typed return shapes; if observable behavior
changed or the return shapes drifted, the assumption is wrong and
the next wave will compound the drift.
