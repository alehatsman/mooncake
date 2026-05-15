# Wave 4 — Agent prompt (R2.1b)

**Date:** 2026-05-15
**Source:** [`2026-05-15-refactoring-plan.md`](./2026-05-15-refactoring-plan.md) §4 (R2.1b)
**Wave:** 4 of 4 — compose
**Gate:** **both** R1.1b and R2.1a in master before firing.
**Unlocks:** wave-4 lands → refactor plan complete; the kernel-surface
checkpoint from §9 of the refactor plan is fully met.

Wave 4 is the **smallest** PR of the entire plan. Once R1.1b shipped
`apply.KernelResult.Reverse()` and R2.1a shipped `fleet.Orchestrator`
with a flat return, this PR just composes them. Per-peer results become
`apply.KernelResult`s; fleet-scope wraps them in `FleetKernelResult`;
`Reverse()` composes by calling per-peer `Reverse()` and assembling
a `FleetPlan`. No new primitives.

The hard work was R1.1b. Wave 4 is a small follow-on; if you find
yourself wanting to redesign anything, STOP — you're past scope.

---

## Prompt — R2.1b: compose `FleetKernelResult` from per-peer `apply.KernelResult`

````
You are executing R2.1b of the Mooncake refactoring plan: compose
the fleet-scope kernel surface. R2.1a (Wave 3) left
fleet.Orchestrator.Run with a flat return. R1.1b (Wave 3) shipped
apply.KernelResult. This PR maps per-peer apply.KernelResults into
a FleetKernelResult with a composing Reverse().

## Read first (load context)

1. /home/aleh/projects/mooncake/docs-working-v2/arch-report/2026-05-15-refactoring-plan.md
   — find R2.1b under §4. Read the locked API contract.
2. /home/aleh/projects/mooncake/docs-working-v2/vision/kernel.md
   — fleet is a kernel rendering; FleetKernelResult is the natural
   typed shape.
3. /home/aleh/projects/mooncake/CLAUDE.md.
4. /home/aleh/projects/mooncake/internal/apply/result.go — read
   what R1.1b shipped. You're composing its KernelResult one level
   up. The Reverse() composition mirrors the apply-level walker
   that R1.1b implemented.
5. /home/aleh/projects/mooncake/internal/fleet/orchestrator.go —
   read what R2.1a shipped. You're changing its Run signature and
   building the per-peer KernelResults during the existing
   ApplyToPhase flow.

## Mission

Change fleet.Orchestrator.Run from a flat return to:

  func (o *Orchestrator) Run(ctx context.Context) (*FleetKernelResult, error)

Where FleetKernelResult composes per-peer apply.KernelResult:

  type FleetKernelResult struct {
      Plan    *plan.Plan                       // the plan that was dispatched
      Peers   map[PeerID]*apply.KernelResult   // per-peer outcome
      Summary FleetSummary                      // aggregate counts + per-peer status
  }

  // Reverse builds an inverse FleetPlan by calling peer.Reverse() on
  // each entry in Peers and assembling the results.
  func (r *FleetKernelResult) Reverse() (*FleetPlan, error)

The shape composes. R1.1b did the hard work of typing the per-peer
result. R2.1b just lifts that one level up.

## API contract (locked decision — do not redesign)

Match the contract above exactly. PeerID is the existing
fleet.Peer.Name (string); don't invent a new type. FleetPlan is a
new struct that mirrors plan.Plan but groups by peer:

  type FleetPlan struct {
      ByPeer map[PeerID]*plan.Plan
  }

(Or whatever exact shape the existing fleet code finds natural —
this is the only field you have any latitude on. Just don't add
fields beyond what's needed for Reverse + recap.)

## Files

New:
- internal/fleet/result.go — FleetKernelResult + Reverse() +
  FleetSummary + FleetPlan.

Modified:
- internal/fleet/orchestrator.go — Run() signature changes; the
  existing ApplyToPhase flow collects per-peer *apply.KernelResult
  (instead of whatever flat shape R2.1a chose) and the assembly into
  *FleetKernelResult happens at the end of Run.
- internal/fleet/orchestrator_test.go — adds a Reverse-coverage
  test:
    - runs a plan against ≥2 peers with reversible steps on each
      (e.g. file.write to a temp path on each peer)
    - calls Reverse() on the result
    - asserts the returned FleetPlan has per-peer entries that
      would restore pre-state if executed
- cmd/fleet.go — recap renderer reads FleetKernelResult.Summary.

Plus an MCP smoke test:
- internal/mcp/<some_test>.go — single test that constructs a
  fleet.Orchestrator and asserts the returned *FleetKernelResult
  has the documented fields.

## Reverse() implementation note

Trivial. For each (peerID, *apply.KernelResult) in Peers, call
peer.Reverse() to get a *plan.Plan; assemble into FleetPlan.ByPeer.
If any per-peer Reverse() returns an error, surface that — don't
swallow.

## Claims protocol

(task="R2.1b")

## Constraints

- DO NOT change the locked API contract.
- DO NOT redesign FleetPlan shape — pick whatever the existing fleet
  code makes natural. The point is to land R2.1b, not to invent
  fleet-plan semantics.
- DO NOT merge to master, DO NOT push. Worktree branch only.
- DO NOT --no-verify.

## Workflow

1. cd /home/aleh/projects/mooncake
2. Verify BOTH R1.1b and R2.1a are in master:
     git log --oneline | grep -E "R1.1b|R2.1a"
   should show both merges. If either is missing, STOP — R2.1b
   depends on both.
3. git worktree add /home/aleh/projects/mooncake-r2.1b-fleet-result -b worktree-r2.1b-fleet-result
4. cd /home/aleh/projects/mooncake-r2.1b-fleet-result
5. Append claimed line.
6. go test ./... baseline (ignore mDNS).
7. Append in-progress line.
8. Write internal/fleet/result.go (FleetKernelResult, FleetSummary,
   FleetPlan, Reverse).
9. Modify internal/fleet/orchestrator.go: change Run signature.
   Inside, ensure the existing flow collects per-peer
   *apply.KernelResult (R2.1a's flow may need a small tweak to
   capture full results instead of just exit codes).
10. Update cmd/fleet.go recap renderer.
11. Add Reverse-coverage test in orchestrator_test.go.
12. Add MCP smoke test.
13. go build ./... — must succeed.
14. go test ./... — must pass.
15. go vet ./... — clean.
16. git commit:
    "refactor(R2.1b): compose FleetKernelResult from per-peer apply.KernelResult

    R2.1a left fleet.Orchestrator.Run with a flat return. R1.1b
    shipped apply.KernelResult. This PR maps each peer's outcome
    into a *FleetKernelResult and composes Reverse() by calling
    per-peer KernelResult.Reverse() and assembling a *FleetPlan.

    Closes the kernel-surface checkpoint from the refactor plan's §9:
    both Kernel.Apply() (R1.1b) and Kernel.FleetApply() (R2.1b)
    return typed results carrying the kernel shape forward. The
    next frontend (fleet why, fleet drift remediation, MCP fleet
    apply_approved) can compose with the typed shape rather than
    re-deriving it.

    See docs-working-v2/arch-report/2026-05-15-refactoring-plan.md §4 (R2.1b).
    Wave 4 / refactor plan complete."
17. Append done line.

## Report

```
task:     R2.1b
status:   done | abandoned
worktree: <path>
branch:   <branch>
sha:      <commit sha>
api:      Orchestrator.Run returns *fleet.FleetKernelResult ✓ | ✗
reverse:  FleetKernelResult.Reverse() test PASS | FAIL
mcp:      internal/mcp smoke test PASS | FAIL
tests:    PASS | FAIL <details>
notes:    <surprising>
```
````

---

## After this lands

The refactor plan is complete. Per the success criteria in §9 of the
plan:

- cmd/ total LOC dropped by ≥ 800
- cmd/fleet.go shrunk by ≥ 500 LOC; fleetApplyAction gocyclo ≤ 10
- cmd/mooncake.go shrunk by ≥ 300 LOC; cmd.run gocyclo ≤ 10
- internal/executor non-test LOC reduced (control + secrets resolver
  moved out)
- New packages: internal/control, internal/secrets/resolver,
  internal/plan/filter, internal/apply, internal/fleet/orchestrator.go
- No new circular imports.
- internal/changegraph does NOT exist (Path A decision held)
- internal/apply.KernelResult, fleet.FleetKernelResult both exist
  with their Reverse() methods covered by tests
- internal/mcp imports internal/apply + internal/fleet directly
- mooncake apply and mooncake fleet apply behavior unchanged

Regenerate the arch snapshot to confirm structural metrics:
  scripts/arch-snapshot.sh
Then check docs-working/ARCH_SNAPSHOT.md for:
- internal/executor non-test source LOC under 2,200
- cmd instability still 1.00
- No new package at instability 0.40–0.60 with LOC > 1,500

If all the checkpoint criteria hold, the kernel surface that
vision/kernel.md claimed is now actually exposed in the code. The
next frontends (explain, rewind, MCP apply_approved, etc.) can build
on it without re-implementing orchestration in cmd.
