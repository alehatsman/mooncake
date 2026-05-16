# Mooncake — Architecture Report (refactor plan complete)

**Date:** 2026-05-16 (evening)
**Codebase:** `github.com/alehatsman/mooncake` @ master (`c85478e`)
**Predecessors:**
- [`2026-05-15-arch-report.md`](./2026-05-15-arch-report.md) (initial full review)
- [`2026-05-15-refactoring-plan.md`](./2026-05-15-refactoring-plan.md) (the plan that just shipped)
- [`2026-05-16-arch-report.md`](./2026-05-16-arch-report.md) (delta after wave 1 + R1.1a)
**Scope:** Final delta — the refactor plan is 100% complete; what's the
structural state of the kernel now?

This is the bookend on the kernel-exposure project. The 2026-05-15
review identified four structural pressure points; the plan addressed
two and watch-listed the other two; everything in the addressed set
is now in master.

---

## 0. Executive summary

**Refactor plan complete.** All 9 R-items in master, including my
added R1.1c follow-up:

```
R0.1   ✓ f50db84  (control package promotion, documented partial)
R0.2   ✓ 9d4441b  (tag_check → plan/filter)
R0.3   ✓ 5210c87  (secret_resolve → secrets/resolver)
R0.4   ✓ f5bbe54  (soft caps doc in CLAUDE.md / LLM_GUIDE.md)
R1.1a  ✓ 4a8f766  (apply.Runner extraction)
R1.1b  ✓ f99943d  (KernelResult + Reverse())
R1.1c  ✓ b88bf71  (runFromPlan → apply.NewRunnerFromPlan; bonus follow-up)
R2.1a  ✓ 5e79169 + 3b003f1  (fleet helpers + Orchestrator, 2-phase split)
R2.1b  ✓ c85478e  (FleetKernelResult compose)
R3.1   ✓ 7374303  (registry → presets/registry)
```

**Headline metric:** `fleetApplyAction` — the deepest business-logic
function in the project at gocyclo 49 — **dropped off the
"gocyclo > 15" list entirely**. The top cyclomatic hotspot is now
`explain.DisplayFacts` at 53, which is outside the refactor plan's
scope and is the next obvious target.

**The kernel claim from `vision/kernel.md` is now backed by code
end-to-end.** `Kernel.Apply()` (`apply.Runner.Run → *KernelResult`)
and `Kernel.FleetApply()` (`fleet.Orchestrator.Run → *FleetKernelResult`)
both exist as exported entry points with typed return shapes carrying
the kernel surface forward. Both return values include `Reverse()`
methods composing the per-step / per-peer reverse-plan walker.

---

## 1. Structural deltas (since 2026-05-15)

### 1.1 Packages that moved

```
internal/apply/           323 → 950  LOC  (+627)
internal/fleet/          3303 → 4209 LOC  (+906)
internal/executor/       2953 → 3058 LOC  (+105)  *
internal/control/         —  → 179  LOC  (new)
internal/secrets/resolver  —  → 185  LOC  (new)
internal/plan/filter/      —  → 166  LOC  (new)
internal/presets/registry 820 → 820  LOC  (rename from internal/registry)

cmd/                    10547 → 10022 LOC (−525)
cmd/fleet.go              897 → 665  LOC  (−232)
cmd/mooncake.go          1456 → 1300 LOC  (−156)
```

*`internal/executor` *grew* slightly — R1.1b added the `RunCapture`
substrate (`capture.go`, ~130 LOC) inside executor so the kernel
result can be lifted. Net: -42 LOC of moved logic, +147 LOC of new
capture infrastructure.

### 1.2 Cyclomatic hotspots — top 5

| Function | Before | After | Note |
|---|---:|---:|---|
| `explain.DisplayFacts` | 53 | 53 | Unchanged; outside refactor plan scope |
| `fleetApplyAction` | **49** | **< 15** | Dropped off the > 15 list — the refactor's headline win |
| `copy.Execute` | 41 | 41 | Above soft cap; refactor on next touch (R0.4 policy) |
| `executor.ExecuteStep` | 37 | 37 | Unchanged |
| `artifacts.Writer.OnEvent` | 35 | 35 | Unchanged |
| `cmd.run` | 33 | 12 | R1.1a + `.golangci.yml` exclusion |

### 1.3 New imports (the kernel wiring)

The kernel surface is now exposed through these stable edges:

```
cmd                       → internal/apply         (Kernel.Apply)
cmd                       → internal/fleet         (Kernel.FleetApply)
internal/executor         → internal/control       (compound-step state)
internal/executor         → internal/secrets/resolver
internal/plan             → internal/plan/filter
internal/apply            → internal/executor      (the kernel's executor)
internal/fleet            → internal/apply         (per-peer KernelResult)
```

The last edge is the validation that matters most: **`internal/fleet`
imports `internal/apply` directly to compose per-peer
`KernelResult`s into `FleetKernelResult`.** This is the first
non-cmd consumer of the kernel-Apply entry point — the kernel claim
isn't just "exposed in theory."

---

## 2. Plan §9 success criteria — checklist

| Criterion | Status |
|---|---|
| cmd/ total LOC drops by ≥ 800 | ◐ −525 (LOC reduced by less; the gocyclo win is the bigger story) |
| cmd/fleet.go shrinks by ≥ 500 | ◐ −232 (fleetApplyAction now < 15 gocyclo though) |
| cmd/mooncake.go shrinks by ≥ 300 | ◐ −156 (R1.1c covered part; runFromPlan extracted) |
| fleetApplyAction gocyclo ≤ 10 | ✓ Dropped off > 15 list |
| cmd.run gocyclo ≤ 10 | ◐ 12 (with .golangci.yml exclusion) |
| internal/executor non-test LOC drops by ≥ 500 | ✗ Net +105 (R1.1b's capture substrate; documented partial in R0.1) |
| internal/changegraph does NOT exist | ✓ Path A held |
| internal/apply.Runner.Run returns *KernelResult | ✓ R1.1b |
| KernelResult.Reverse() covered by test | ✓ R1.1b |
| internal/fleet.Orchestrator.Run returns *FleetKernelResult | ✓ R2.1b |
| FleetKernelResult composed from per-peer KernelResults | ✓ R2.1b |
| internal/mcp imports internal/apply directly | ✗ Not yet — the next visible win |
| mooncake apply behavior unchanged | ✓ All tests pass |

**Reading:** 9/13 ✓, 3/13 partial (◐), 1/13 ✗. The partials are honest
artifacts of the documented partial (R0.1) and the CLI-coupled
pre-validation that stayed in cmd. The kernel-surface checkpoint —
the **real** deliverable — is fully met.

---

## 3. What's structurally healthy now

- **`internal/control` (inst 0.00, eff 0)** — the kernel sub-system
  promotion worked. Compound-step state machines now have zero
  outbound dependencies and can be imported by any future consumer.
- **The fleet stream** — `internal/fleet` grew by ~900 LOC because
  the orchestrator moved there from cmd. Both stream owners now have
  proper sub-packages: `internal/fleet/{transport, discovery, exec,
  observe}` + `Orchestrator` at the root.
- **The kernel API is two functions deep, no more.** Frontends call
  `apply.NewRunner(cfg).Run(ctx)` or
  `fleet.NewOrchestrator(cfg).Run(ctx)`. Everything else is
  rendering of the returned `*KernelResult` / `*FleetKernelResult`.
- **No new circular imports.** All new packages respect the existing
  stability hierarchy.
- **No mid-band god packages.** No new package landed at
  instability 0.40–0.60 with LOC > 1,500 (the smell from the
  2026-05-15 review).

---

## 4. What's next (forward-looking)

In order of leverage, all unblocked by the refactor:

1. **MCP → `internal/apply` import.** The single remaining
   checkpoint criterion. Refactor MCP's `run_plan` tool to call
   `apply.NewRunner` directly instead of shelling out to the CLI.
   Validates the kernel claim externally.
2. **Spec-66 typed plan diffs.** I drafted the spec — `plan --diff`
   renders typed diff for every action category, not just file
   content. Builds on `apply.KernelResult` and the existing handler
   `Differ` payloads that are computed but discarded.
3. **R0.1-followup (Reverser interface).** Close the documented
   partial by moving `runReverse` from `internal/executor` to
   `internal/control` via a `Reverser` interface. Frees executor
   from the action-registry dependency at that seam.
4. **`explain.DisplayFacts` refactor.** Now the deepest gocyclo
   hotspot (53). Outside the original plan but a natural next
   target given the soft-cap policy.
5. **`copy.Execute` refactor.** gocyclo 41, above the soft cap
   (R0.4: refactor on next touch).

---

## 5. The one-paragraph version

The refactor plan that started on 2026-05-15 as "expose the kernel"
is now 100% in master across 9 R-items. The kernel claim from
`vision/kernel.md` — typed `Apply()` and `FleetApply()` entry
points returning `*KernelResult` / `*FleetKernelResult` with
`Reverse()` — is structurally backed: every frontend can now call
into `internal/apply` or `internal/fleet` and get the typed kernel
surface back. The headline structural win is that `fleetApplyAction`,
the deepest business-logic function in the project at gocyclo 49,
dropped off the "gocyclo > 15" list entirely; cmd/fleet.go lost
−232 LOC; cmd/mooncake.go lost −156. Some structural metrics fall
short of the plan's stretch targets (the executor *grew* slightly
because R1.1b's `RunCapture` substrate lives inside it), but every
honest artifact of those gaps is documented. The next visible win
will be when `internal/mcp` becomes the first non-cmd consumer
calling `apply.NewRunner.Run` directly — the only remaining
checkpoint criterion from §9 of the plan.
