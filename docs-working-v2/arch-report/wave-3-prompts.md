# Wave 3 — Agent prompts (R1.1b + R2.1a)

**Date:** 2026-05-15
**Source:** [`2026-05-15-refactoring-plan.md`](./2026-05-15-refactoring-plan.md) §3 (R1.1b), §4 (R2.1a)
**Wave:** 3 of 4 — parallel: kernel-surface API + fleet mechanical move
**Gate:** R1.1a in master before firing either of these.
**Unlocks:** Wave 4 (R2.1b) can run after both R1.1b and R2.1a land.

Wave 3 is **two parallel PRs**. They touch different files, share no
state, and can run in different worktrees concurrently:

- **R1.1b** (API surface) — adds `KernelResult` typed return + `Reverse()`
  method to the `apply.Runner` shipped in Wave 2. Small file count,
  deep review (API design decision).
- **R2.1a** (mechanical) — extracts `fleetApplyAction` (gocyclo 49)
  into `internal/fleet.Orchestrator`. Same shape as R1.1a, larger
  scope. Flat return for now; R2.1b composes the typed shape.

Review note: R1.1b is the **API-surface review** of the wave. The
field names, the `Reverse()` semantics, the `*KernelResult` shape —
all get argued here. R2.1a is mechanical and reviews like R1.1a did.
Don't bundle them on one agent or one reviewer.

---

## Prompt A — R1.1b: crystallize `KernelResult` + `Reverse()` on `apply.Runner`

````
You are executing R1.1b of the Mooncake refactoring plan:
crystallize the kernel-surface API on apply.Runner. R1.1a (Wave 2)
left Runner.Run returning a flat shape. This PR introduces the typed
*KernelResult return + Reverse() method.

## Read first (load context)

1. /home/aleh/projects/mooncake/docs-working-v2/arch-report/2026-05-15-refactoring-plan.md
   — find R1.1b under §3. Read the locked API contract and DONE
   criteria carefully.
2. /home/aleh/projects/mooncake/docs-working-v2/vision/kernel.md
   — the kernel framing this PR materializes. The four typed ABI
   properties (Diff/Reverse/Cost/Permissions) become a typed return
   shape here.
3. /home/aleh/projects/mooncake/CLAUDE.md — project conventions.
4. /home/aleh/projects/mooncake/internal/apply/runner.go — read
   what R1.1a left. You're modifying its return signature.

## Mission

Change apply.Runner.Run from a flat return to:

  func (r *Runner) Run(ctx context.Context) (*KernelResult, error)

Where KernelResult is the kernel's typed "what just happened" shape:

  type KernelResult struct {
      Plan    *plan.Plan            // the typed plan that ran
      Steps   []executor.StepResult // per-step outcome
      Events  []events.Event        // audit substrate — run's event tail
      Summary RunSummary            // ok/changed/skipped/failed + duration
  }

  // Reverse builds the inverse plan from this run's reversible
  // subset. Returns the empty plan if nothing was reversible.
  func (r *KernelResult) Reverse() (*plan.Plan, error)

The Reverse() method is the load-bearing add. Today, building a
reverse plan from a completed run is NOT exposed as a kernel
operation — transaction: blocks do it in-process (lifted to
internal/control by R0.1); nothing else can. Adding it here unlocks:

- Cross-run rewind (mooncake rewind --to <ts>)
- MCP rollback tools
- Agent loop's "undo your last action" affordance

All from one function.

## API contract (locked decision — do not redesign)

The contract above is the locked decision from the kernel-surface
discussion. DO NOT add fields beyond the four shown. DO NOT change
the Reverse() signature. DO NOT introduce a Result interface. If
something doesn't fit, append an abandoned claim with the specific
mismatch and stop — don't redesign.

## Files

New:
- internal/apply/result.go — KernelResult struct + Reverse() +
  RunSummary type.

Modified:
- internal/apply/runner.go — Run signature changes; body builds a
  *KernelResult instead of returning the flat shape.
- internal/apply/runner_test.go — adds at least one test that:
  - runs a plan with at least one reversible step (file.write is
    easiest)
  - calls Reverse() on the result
  - asserts the returned plan would restore pre-state if executed
  (the existing apply-path integration test from R1.1a stays.)
- cmd/mooncake.go — recap renderer now reads KernelResult.Summary
  instead of the flat shape.

Plus an MCP smoke test:
- internal/mcp/<some_test>.go — single test that constructs an
  apply.Runner and asserts the returned *KernelResult has the
  documented fields. Doesn't have to actually execute; just prove
  the import works without circular deps.

## Reverse() implementation note

DO NOT invent new Reverse semantics. The transaction-rollback walker
already exists in:

  internal/executor/transaction.go : handleTxnBodyFailure
  (which calls executor.runReverse per completed step)

Or, post-R0.1, parts of the state-machine live in internal/control.
Lift the **walker pattern** (LIFO over completed steps; call
Reverse() on each; assemble inverse Plan) into a helper function in
internal/apply/result.go. Same algorithm; input is the
KernelResult.Steps slice; output is a *plan.Plan.

Edge cases the walker must handle (already handled by the existing
transaction walker — copy that logic, don't re-derive):

- Some steps refuse Reverse (Reverser interface absent OR returns
  irreversible error). Those steps are SKIPPED in the inverse plan;
  no error.
- Steps with Result.Changed=false don't need to be reversed.
- The order is LIFO (last completed → first reversed).

## Claims protocol

(same as Wave 2 / R1.1a; task="R1.1b")

## Constraints (do not break)

- DO NOT change the locked API contract. Match it exactly.
- DO NOT touch fleet code. R2.1a is a separate parallel PR.
- DO NOT merge to master, DO NOT push. Worktree branch only.
- DO NOT --no-verify.

## Workflow

1. cd /home/aleh/projects/mooncake
2. git worktree add /home/aleh/projects/mooncake-r1.1b-kernel-result -b worktree-r1.1b-kernel-result
3. cd /home/aleh/projects/mooncake-r1.1b-kernel-result
4. Verify R1.1a is in master: `git log --oneline | grep "R1.1a"` should
   show its merge. If not, STOP — R1.1b depends on R1.1a.
5. Append claimed line.
6. go test ./... baseline (ignore mDNS).
7. Append in-progress line.
8. Write internal/apply/result.go (KernelResult, RunSummary, Reverse).
9. Modify internal/apply/runner.go: change Run signature, build the
   *KernelResult.
10. Update cmd/mooncake.go recap renderer.
11. Update internal/apply/runner_test.go: add Reverse coverage test.
12. Add internal/mcp smoke test that constructs a Runner.
13. go build ./... — must succeed.
14. go test ./... — must pass.
15. go vet ./... — clean.
16. git commit:
    "refactor(R1.1b): crystallize KernelResult + Reverse() on apply.Runner

    R1.1a left apply.Runner with a flat return. This PR materializes
    the kernel-surface contract from vision/kernel.md: typed
    *KernelResult carrying plan + per-step results + audit substrate
    + a Reverse() method. The cross-run reverse-plan-construction
    walker (today only reachable inside transaction:) is now exposed
    as a public method on the kernel's apply result.

    Unlocks: cross-run rewind, MCP rollback tools, agent-loop undo —
    all from a single typed function.

    See docs-working-v2/arch-report/2026-05-15-refactoring-plan.md §3 (R1.1b)."
17. Append done line.

## Report

```
task:     R1.1b
status:   done | abandoned
worktree: <path>
branch:   <branch>
sha:      <commit sha>
api:      Runner.Run returns *apply.KernelResult ✓ | ✗
reverse:  KernelResult.Reverse() test PASS | FAIL
mcp:      internal/mcp smoke test PASS | FAIL
tests:    PASS | FAIL <details>
notes:    <surprising>
```
````

---

## Prompt B — R2.1a: pure extraction of `fleet.Orchestrator` from `fleetApplyAction`

````
You are executing R2.1a of the Mooncake refactoring plan: pure
mechanical extraction of fleetApplyAction (gocyclo 49, the deepest
business-logic function in the project) into a new
internal/fleet.Orchestrator. fleetApplyAction becomes a thin shim.
No typed FleetKernelResult yet — that's R2.1b in Wave 4.

## Read first (load context)

1. /home/aleh/projects/mooncake/docs-working-v2/arch-report/2026-05-15-refactoring-plan.md
   — find R2.1a under §4. Read full Motivation / Files / DONE /
   Risk sections.
2. /home/aleh/projects/mooncake/docs-working-v2/vision/kernel.md
   — fleet is a kernel rendering: one kernel, multi-host topology.
3. /home/aleh/projects/mooncake/CLAUDE.md — project conventions.
4. /home/aleh/projects/mooncake/internal/apply/runner.go — read
   what R1.1a established. R2.1a copies its pattern at fleet scope.

## Mission

fleetApplyAction (cmd/fleet.go:336, gocyclo 49) does:
- peer filtering
- plan-snapshot upload
- ordered phase rollout
- per-peer SSE fan-in
- recap aggregation
- exit-code computation

This PR is the CODE-MOVE-ONLY half. Each responsibility becomes a
method on Orchestrator. Run() returns a flat shape (RunSummary,
error) matching today's behavior. The typed *FleetKernelResult
return is R2.1b's job in Wave 4 — don't bundle it here.

## Files

New:
- internal/fleet/orchestrator.go — Orchestrator struct + methods:
    (o *Orchestrator) FilterPeers(ctx) ([]Peer, error)
    (o *Orchestrator) UploadPlan(ctx, peer) error
    (o *Orchestrator) ApplyToPhase(ctx, phase []Peer) ([]PeerResult, error)
    (o *Orchestrator) Run(ctx) (RunSummary, error)  — top-level
- internal/fleet/config.go — Config struct holding fields that
  fleetApplyAction accepts as CLI flags.
- internal/fleet/orchestrator_test.go — the FIRST test in this
  file locks down the exit-code matrix (all-success → 0, any-fail
  → 1, partial-success rules per current behavior). Then test
  each method.

Modified:
- cmd/fleet.go — fleetApplyAction collapses to flag parse +
  Orchestrator.Run() + recap render. ~50 LOC.
- cmd/fleet_filter_test.go — retargeted to test the new package
  where cheaper.

(Optional: if any single method's body is large enough, split into
separate files in internal/fleet/{filter,upload,phase,recap}.go.)

## Watch / risks

- (a) Hidden side effects in fleetApplyAction. Diff scripted
  output before/after on a 2-peer toy config.
- (b) Per-peer SSE fan-in has timing semantics: concurrent
  goroutines, per-peer phasing, channel coordination. Preserve
  the goroutine topology BYTE-FOR-BYTE. This is a code-shape
  change, not a concurrency-model change. If you find yourself
  rewriting the channel topology, STOP — you're past the
  refactor's scope.
- (c) Error aggregation across peers has subtle exit-code rules
  (any-fail → nonzero, partial-success per current behavior).
  Lock the matrix down as the FIRST test in
  orchestrator_test.go, before doing the extraction. Use that
  test as the safety net during the refactor.

## Claims protocol

(task="R2.1a")

## Constraints

- PURE relocation. Every test that passes before passes after.
- DO NOT introduce FleetKernelResult. DO NOT add Reverse() at
  fleet scope. Those are R2.1b in Wave 4.
- DO NOT touch internal/apply (R1.1b is concurrent; different files).
- DO NOT merge to master, DO NOT push. Worktree branch only.
- DO NOT --no-verify.

## Workflow

1. cd /home/aleh/projects/mooncake
2. git worktree add /home/aleh/projects/mooncake-r2.1a-fleet-orch -b worktree-r2.1a-fleet-orch
3. cd /home/aleh/projects/mooncake-r2.1a-fleet-orch
4. Verify R1.1a is in master.
5. Append claimed line.
6. go test ./... baseline.
7. Append in-progress line.
8. WRITE THE EXIT-CODE MATRIX TEST FIRST. Before touching
   fleetApplyAction. Use it as the safety net.
9. Do the extraction:
   - mkdir -p internal/fleet
   - Create internal/fleet/orchestrator.go with the 6 methods.
   - Create internal/fleet/config.go.
   - Reduce cmd/fleet.go's fleetApplyAction to flag-parse +
     Orchestrator.Run() + render.
10. go build ./... — must succeed.
11. go test ./... — must pass.
12. go vet ./... — clean.
13. Verify gocyclo: `gocyclo cmd/fleet.go | grep fleetApplyAction`
    ≤ 10 (was 49).
14. Verify LOC: `wc -l cmd/fleet.go` ≥ 500 lines smaller.
15. Scripted parity test against a 2-peer toy config (if you have
    one set up — otherwise skip and document in report).
16. git commit:
    "refactor(R2.1a): extract fleet.Orchestrator from fleetApplyAction

    Pure mechanical extraction of the deepest business-logic
    function in the project (gocyclo 49) into
    internal/fleet.Orchestrator. fleetApplyAction becomes a
    flag-parse + Orchestrator.Run() + recap-render shim. Six
    methods on Orchestrator factor the responsibilities cleanly:
    FilterPeers, UploadPlan, ApplyToPhase, Run (+ helpers).

    Return shape stays flat — FleetKernelResult composition comes
    in R2.1b (Wave 4).

    The exit-code matrix is locked down as the FIRST test in
    orchestrator_test.go so the refactor proceeds against a
    safety net.

    No behavior change.

    See docs-working-v2/arch-report/2026-05-15-refactoring-plan.md §4 (R2.1a)."
17. Append done line.

## Report

```
task:     R2.1a
status:   done | abandoned
worktree: <path>
branch:   <branch>
sha:      <commit sha>
gocyclo:  fleetApplyAction = N  (was 49; target ≤ 10)
loc:      cmd/fleet.go = N  (was ~897; target ≥ 500 LOC drop)
matrix:   exit-code-matrix test PASS | FAIL
tests:    PASS | FAIL <details>
parity:   scripted fleet apply byte-identical | <divergence>
notes:    <surprising>
```
````

---

## After both land

When both R1.1b and R2.1a have merged into master:

- Wave 3 is complete.
- Wave 4 (R2.1b — compose FleetKernelResult) becomes unblocked.
- See [`wave-4-prompts.md`](./wave-4-prompts.md).

If R1.1b lands but R2.1a stalls in review (or vice versa), that's
fine — they're independent. Wave 4 only needs both to be in master
before firing.
