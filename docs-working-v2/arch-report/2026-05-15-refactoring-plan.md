# Refactoring Plan — Structured Execution

**Date:** 2026-05-15
**Source:** [`2026-05-15-arch-report.md`](./2026-05-15-arch-report.md)
**Status:** Ready for execution. All bug-fix work paused; this is the
next phase.

This plan turns the arch report's recommendations into discrete,
independently-shippable items. Each item is sized for **one agent /
one worktree / one PR**. Items within the same phase are
parallel-safe. Phases are gated on the prior phase landing.

---

## 0. Premise

The arch report identified four structural pressure points:

1. `cmd/` carries application services (largest package, gocyclo
   concentration).
2. `internal/executor` accreting eight runtime concerns into one
   package.
3. `internal/config/config.go` monotonic growth (closed-action-set
   cost — *watch only*).
4. Handler proliferation tail (*watch only*).

Pressure points 3 and 4 are explicitly **don't-act**. This plan
addresses 1 and 2.

The execution principle: **prove the pattern with the smallest, lowest-
risk extraction first** (Phase 0 + 1), then propagate (Phase 2). Each
move is a pure relocation with no behavior change — the work is
mechanical and reviewable in a single sitting.

---

## 1. Plan at a glance

| Phase | Item | Title | Effort | Risk | Depends on |
|---|---|---|---|---|---|
| 0 | R0.1 | Promote `transaction` + `trycatch` → `internal/control` | S | low | — |
| 0 | R0.2 | Move `tag_check` → `internal/plan/filter` | S | low | — |
| 0 | R0.3 | Promote `secret_resolve` → `internal/secrets/resolver` | S | low | — |
| 0 | R0.4 | Document soft caps in `CONTRIBUTING.md` / `LLM_GUIDE.md` | XS | none | — |
| 1 | R1.1 | Extract `internal/apply.Runner` out of `cmd.run` (apply path) | M | medium | R0.1, R0.3 |
| 2 | R2.1 | Extract `internal/fleet.Orchestrator` out of `cmd/fleet.go` | L | medium | R1.1 |
| 3 | R3.1 | Rename `internal/registry` → `internal/presets/registry` | S | low | — |

Effort scale: XS = doc only; S = ~1 day, single file moves;
M = ~2–3 days, single PR; L = ~3–5 days, possibly split into a series.

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
rollback, try/catch/finally branch routing) are *distinct* from leaf-
step dispatch. They sit in `internal/executor/` for historical reasons,
not because they share state with `ExecuteStep`. Promoting them out
shrinks executor toward its actual responsibility (dispatch + step
lifecycle + result capture) and sets the precedent for further
executor splits.

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

**Motivation:** `!secret` typed-ref resolution is a *pre-execute walk*
over the step tree. It's used today by the executor; it's exactly the
kind of thing the MCP server (`internal/mcp`) and the agent loop
(`internal/agent`) would want to call before submitting a plan,
without dragging in 3,270 LOC of executor. Promoting it to its own
package unlocks that reuse.

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

## 3. Phase 1 — Prove the cmd-extraction pattern

### R1.1 — Extract `internal/apply.Runner` out of `cmd.run`

**Source:** arch report §3.1, recommendation 4.

**Motivation:** `cmd.run` (`cmd/mooncake.go:236`, gocyclo **33**) is
the local-apply dispatcher. Today it does: config resolution, vars
layering, tag filtering, plan building, plan-or-execute selection,
artifacts writing, run audit. None of that is "CLI parse → call →
render." The MCP server (`internal/mcp`) wants this path; today it
can't have it.

**Goal:** turn `cmd.run` into a thin shim that builds a
`*apply.Config` from CLI flags and calls `apply.NewRunner(cfg).Run()`.

**Files touched:**

New:
- `internal/apply/runner.go` — `Runner` struct + `Run()` method
- `internal/apply/config.go` — `Config` struct (the fields that
  `cmd.run` accepts as CLI flags)
- `internal/apply/runner_test.go` — at least one apply-path
  integration test that exercises the new entry point
- `internal/apply/doc.go`

Modified:
- `cmd/mooncake.go` — `run` collapses to ~50 LOC of flag parse +
  `Runner.Run()` call + recap rendering
- `cmd/cmd_test.go` — relevant tests retargeted to the new API where
  cheaper

**DONE when:**
1. `go test ./...` passes.
2. `cmd.run` gocyclo is ≤ 10 (currently 33). Measured via
   `gocyclo cmd/mooncake.go`.
3. `cmd/mooncake.go` LOC drops by ≥ 300.
4. `internal/apply/runner.go` exists with `func (r *Runner) Run(ctx context.Context) error`.
5. `internal/mcp` package can import `internal/apply` and call
   `Runner.Run` — verified by a single test in `internal/mcp` that
   constructs a `Runner` (doesn't have to actually execute; just
   prove the import works without circular deps).
6. End-to-end manual: `mooncake apply -c examples/...` produces
   identical output (recap, exit code, audit events) before and after.

**Blast radius:** medium. Touches the largest file in `cmd/` and the
most-used CLI path. But no behavior change — every test that passed
before passes after.

**Risk:** medium. Three sub-risks:
- (a) Hidden side effects in `cmd.run` (logging setup, env-var
  reading) get missed during extraction. *Mitigation:* run the full
  cmd_test.go suite locally before/after; diff stdout/stderr from a
  scripted apply on `examples/` plans.
- (b) The shape of `apply.Config` doesn't accommodate fleet-apply.
  *Mitigation:* don't try to. R1.1 covers local apply only; fleet
  apply is R2.1's problem and uses its own orchestrator that may
  internally call `apply.Runner` per peer.
- (c) Circular import: `internal/apply` may want to import
  `internal/executor`, which is fine — but if the extraction also
  pulls in `internal/secrets/resolver` (from R0.3), check the
  resolved import graph doesn't cycle. *Mitigation:* run `go list -e`
  with cycle detection as part of CI.

**Dependencies:**
- R0.1 (control package) — soft; if executor's compound-step concerns
  have moved out, `apply.Runner` calls a smaller executor surface.
- R0.3 (secrets resolver) — soft; similar.

If R0.1 / R0.3 land first, R1.1 is cleaner. If they slip, R1.1 still
works but imports the unmoved files.

---

## 4. Phase 2 — Cmd extraction at scale

### R2.1 — Extract `internal/fleet.Orchestrator` out of `cmd/fleet.go`

**Source:** arch report §3.1 table; recommendation 1.

**Motivation:** `fleetApplyAction` (`cmd/fleet.go:336`) at gocyclo
**49** is the deepest business-logic function in the project.
Reading it: peer filtering, plan-snapshot upload, ordered phase
rollout, per-peer SSE fan-in, recap aggregation, exit-code
computation — six responsibilities. The pattern from R1.1 applies
here exactly: each responsibility becomes a method on a
`fleet.Orchestrator` struct.

**Files touched:**

New:
- `internal/fleet/orchestrator.go` — `Orchestrator` struct + methods:
  - `(o *Orchestrator) FilterPeers(ctx) ([]Peer, error)`
  - `(o *Orchestrator) UploadPlan(ctx, peer) error`
  - `(o *Orchestrator) ApplyToPhase(ctx, phase []Peer) ([]PeerResult, error)`
  - `(o *Orchestrator) Run(ctx) (RunSummary, error)` — top-level
    entry point
- `internal/fleet/orchestrator_test.go`
- (Optional) split into `internal/fleet/{filter,upload,phase,recap}.go`
  if each method is large enough to warrant its own file

Modified:
- `cmd/fleet.go` — `fleetApplyAction` collapses to flag parse +
  `Orchestrator.Run()` + render
- Probably also touches `cmd/fleet_filter_test.go`, retargeted to
  test the new package

**DONE when:**
1. `go test ./...` passes.
2. `fleetApplyAction` gocyclo is ≤ 10 (currently 49). Measured via
   `gocyclo cmd/fleet.go`.
3. `cmd/fleet.go` LOC drops by ≥ 500.
4. `internal/fleet/orchestrator.go` exists with the documented
   methods.
5. The MCP server can call `fleet.NewOrchestrator(cfg).Run(ctx)`
   without going through `cmd` — verified by a smoke test in
   `internal/mcp` that constructs an Orchestrator (does not execute).
6. Manual verification: `mooncake fleet apply -p tag=linux examples/...`
   produces identical per-peer output, identical RECAP, identical
   exit code before/after.

**Blast radius:** large within `cmd/fleet.go`, zero elsewhere.

**Risk:** medium. Same three sub-risks as R1.1, plus:
- (d) Per-peer SSE fan-in has timing semantics (concurrent streams,
  per-peer phasing). *Mitigation:* preserve the goroutine
  topology byte-for-byte. The extraction is a code-shape change,
  not a concurrency model change.
- (e) Error aggregation across peers. The current code has subtle
  exit-code logic (any-fail → nonzero, partial-success rules).
  *Mitigation:* lock the exit-code matrix down as a test case in
  `orchestrator_test.go` first, then refactor.

**Dependencies:**
- R1.1 must land first. R1.1 establishes the orchestrator-extraction
  pattern and proves it with a smaller, simpler target. R2.1 then
  *follows the same shape*. Trying R2.1 first would mean inventing
  the pattern on the project's most complex function.

**Sub-split option:** if R2.1 is too big for one PR, split into:
- R2.1a: introduce `Orchestrator` struct with `Run()` that just
  inlines today's `fleetApplyAction` body unchanged. Pure relocation.
- R2.1b: factor `Run()` into the five typed methods.

This is a defensible split if the reviewer asks for it; not required.

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

```
                        ┌─────────────────────────────┐
                        │   Phase 0 (parallel-safe)   │
                        │                             │
                        │   R0.1   R0.2   R0.3   R0.4 │
                        └──────────────┬──────────────┘
                                       │
                                       ▼
                        ┌─────────────────────────────┐
                        │   Phase 1 (pattern proof)   │
                        │                             │
                        │            R1.1             │
                        │   (soft-depends R0.1, R0.3) │
                        └──────────────┬──────────────┘
                                       │
                                       ▼
                        ┌─────────────────────────────┐
                        │   Phase 2 (at scale)        │
                        │                             │
                        │            R2.1             │
                        │      (depends R1.1)         │
                        └─────────────────────────────┘

                        ┌─────────────────────────────┐
                        │   Phase 3 (independent)     │
                        │                             │
                        │            R3.1             │
                        │  (no deps; any time)        │
                        └─────────────────────────────┘
```

**Recommended cadence for an agent fleet:**

- **Day 1:** kick off R0.1, R0.2, R0.3, R0.4, R3.1 in five worktrees
  in parallel. All should land same-day or next-day given the small
  blast radius.
- **Day 2:** start R1.1 in one worktree. Single-agent ownership; the
  pattern proof is the deliverable.
- **Day 3–4:** review R1.1 carefully — the pattern from this PR is
  what R2.1 will copy. Don't rush.
- **Day 5–7:** R2.1 in one worktree. Consider the R2.1a/R2.1b split
  if reviewer asks.

**If using only one agent at a time:** execute in order. Total
estimated wall-clock: 7–10 days. Total estimated LOC moved: ~3,500.
Total estimated LOC of new code (besides moves): ~400 (mostly new
struct types and method bodies that were inlined before).

---

## 8. Anti-scope — what this plan does NOT do

These came up while drafting and were intentionally excluded:

- **Splitting `internal/config/config.go`.** The arch report
  explicitly recommends *watch, don't act*. The 1,866 LOC is the
  cost of the closed-action-set bet. Touching it has cascading
  blast radius (76 importers) for no architectural payoff.
- **Refactoring large handlers** (`file`, `tool`, `service`,
  `package`, `os_mount`). Soft cap policy from R0.4 covers the
  question; act when next touched, not preemptively.
- **Adding interfaces / abstraction layers.** Every R-item in this
  plan is a *relocation* or a *struct introduction at a clear seam*.
  No new interfaces, no new dependency injection, no plugin contracts.
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

- `cmd/` total LOC drops by ≥ 800 (from ~10,547 to ≤ 9,750)
- `cmd/fleet.go` shrinks by ≥ 500 LOC; `fleetApplyAction` gocyclo ≤ 10
- `cmd/mooncake.go` shrinks by ≥ 300 LOC; `cmd.run` gocyclo ≤ 10
- `internal/executor` non-test LOC drops by ≥ 500 (control +
  secrets resolver moved out)
- New packages exist: `internal/control`, `internal/secrets/resolver`,
  `internal/plan/filter`, `internal/apply`, `internal/fleet/orchestrator.go`
- No new circular imports
- `scripts/arch-snapshot.sh` shows:
  - `internal/executor` non-test source LOC under 2,200
  - `cmd` instability still 1.00 (still no internal imports of cmd)
  - No new package at instability 0.40–0.60 with LOC > 1,500
    (the "mid-band god package" smell to avoid)
- `mooncake apply` and `mooncake fleet apply` behavior unchanged —
  the entire plan is a refactor, not a feature change.

If any of those criteria don't hold after Phase 2 lands, **stop and
re-review** before starting Phase 3. The plan assumes the moves are
all pure relocations; if observable behavior changed, the assumption
is wrong and the next phase will compound the drift.
