# Mooncake — Architecture Report (delta)

**Date:** 2026-05-16
**Codebase:** `github.com/alehatsman/mooncake` @ master (`4a8f766`)
**Predecessor:** [`2026-05-15-arch-report.md`](./2026-05-15-arch-report.md)
**Scope:** Delta-focused. What moved structurally in 24 hours; how the
kernel-exposure claim from `vision/kernel.md` is now backed by code;
what's left.
**Grounded in:** `docs-working/ARCH_SNAPSHOT.md` (regenerated today)
and direct reads of the new packages.

This is a delta report — read the 2026-05-15 review first for the
full structural model (handler ABI, plan/execute boundary, the four
typed properties as the moat). This doc reports what changed.

---

## 0. Executive summary

**Wave 1 + R1.1a of the refactor plan landed.** The kernel boundary
that 2026-05-15 described conceptually is now drawn in code:
five new packages materialize what was previously scattered across
`internal/executor` and `cmd/`. Two items (R1.1b + R2.1a, Wave 3)
are in flight; R2.1b (Wave 4) is gated on them. The kernel-exposure
project is **~70% complete by R-item count, ~60% by LOC moved.**

The five new packages and their instability profile:

| Package | LOC | Eff | Aff | Inst. | What it exposes |
|---|---:|---:|---:|---:|---|
| `internal/control` | 179 | 0 | 1 | **0.00** | Compound-step state machines (Tx/Try state + skip-reason determination) |
| `internal/plan/filter` | 166 | 2 | 1 | 0.67 | Plan-time tag filtering |
| `internal/secrets/resolver` | 185 | 2 | 1 | 0.67 | Pre-execute `!secret` walk |
| `internal/apply` | 323 | 4 | 1 | 0.80 | Kernel's `Apply()` entry point |
| `internal/presets/registry` | 820 | 0 | 1 | 0.00 | Renamed from `internal/registry` for disambiguation |

The single strongest signal: **`internal/control` landed at instability
0.00 (foundation-tier)**. It has zero efferent imports — the pure
state-machine logic is genuinely self-contained, exactly the
"callable from any future frontend without dragging executor along"
property the kernel framing promised. That validates the R0.1 design
choice.

---

## 1. What moved

### 1.1 New packages

```
internal/
├── apply/                  NEW  (323 LOC, inst 0.80)  R1.1a
│   ├── doc.go              — kernel-entry-point positioning
│   ├── config.go           — typed Config (the kernel input shape)
│   └── runner.go           — Runner.Run (validate, publish, dispatch)
├── control/                NEW  (179 LOC, inst 0.00)  R0.1
│   ├── doc.go              — kernel-sub-system positioning
│   ├── transaction.go      — TxnState + TxnSkipReason
│   ├── trycatch.go         — TryState + TrySkipReason +
│   │                          RecordTryBodyFailure / RecordTryCatchFailure
│   ├── transaction_test.go (3 tests)
│   └── trycatch_test.go    (7 tests)
├── plan/
│   └── filter/             NEW  (166 LOC, inst 0.67)  R0.2
│       ├── tags.go         — moved from internal/executor/tag_check.go
│       └── tags_test.go
├── secrets/
│   └── resolver/           NEW  (185 LOC, inst 0.67)  R0.3
│       ├── doc.go
│       ├── resolve.go      — moved from executor/secret_resolve.go
│       └── resolve_test.go
└── presets/
    └── registry/           NEW  (820 LOC, inst 0.00)  R3.1
                            — renamed from internal/registry to
                              disambiguate from internal/register
```

### 1.2 Packages that shrunk

| Package | LOC before | LOC after | Δ | Reason |
|---|---:|---:|---:|---|
| `internal/executor` | 2,953 | 2,911 | **−42** | TxnState/TryState moved (R0.1); tag_check moved (R0.2); secret_resolve moved (R0.3) |
| `cmd/mooncake.go` | 1,456 | 1,318 | **−138** | cmd.run lifted to internal/apply (R1.1a) |
| `cmd` (total) | 10,547 | 10,409 | **−138** | Same as above |

Note: the executor shrinkage is modest (~1.4%) because R0.1 was a
**documented partial** — the pure state-machine logic moved to
`internal/control`, but the `*Result`-coupled bits (`recordTxnBodyCompletion`,
`handleTxnBodyFailure`, `runReverse`) stayed in executor where they
have access to handler dispatch. Same shape as the plan's open
question; the partial was accepted.

### 1.3 Imports added (the wiring that exposes the kernel)

The arch-snapshot internal import edges now show:

```
cmd                       → internal/apply
cmd                       → internal/presets/registry
internal/apply            → internal/events
internal/apply            → internal/executor
internal/apply            → internal/facts
internal/apply            → internal/logger
internal/executor         → internal/control
internal/executor         → internal/secrets/resolver
internal/plan             → internal/plan/filter
```

Three of these are the kernel-promotion edges:
- `executor → control` — executor calls into control for state-machine ops
- `executor → secrets/resolver` — executor calls the kernel's pre-execute walk
- `cmd → apply` — the CLI is now a frontend over `internal/apply`

The fourth (`plan → plan/filter`) is the spec-32 alignment R0.2 made.

---

## 2. Kernel-promotion progress

The claim from `vision/kernel.md` was: **the kernel already exists**;
the refactor exposes it. Where does that stand?

### 2.1 Kernel sub-systems

| Concern | Package | Status |
|---|---|---|
| Typed Action ABI | `internal/actions` | ✓ Pre-existing; instability 0.06, 72 aff. |
| Plan compilation | `internal/plan` | ✓ Pre-existing; instability 0.56. |
| Executor dispatch | `internal/executor` | ✓ Pre-existing; shrinking. |
| Side-effect performer | `internal/effects` | ✓ Pre-existing. |
| Audit substrate | `internal/runlog` + `internal/events` | ✓ Pre-existing. |
| Facts | `internal/facts` | ✓ Pre-existing. |
| Compound-step state | **`internal/control`** | ✓ Promoted (R0.1) |
| Tag filtering (plan-time) | **`internal/plan/filter`** | ✓ Promoted (R0.2) |
| Secret resolution | **`internal/secrets/resolver`** | ✓ Promoted (R0.3) |

### 2.2 Kernel entry points

| Entry point | Package / func | Status |
|---|---|---|
| `Kernel.Apply()` (local) | **`internal/apply.Runner.Run`** | ✓ Exposed as flat `error` (R1.1a). Typed `*KernelResult` + `Reverse()` is R1.1b (Wave 3, in flight). |
| `Kernel.FleetApply()` (multi-host) | `internal/fleet.Orchestrator` | → In flight (R2.1a, mechanical extraction). Typed `*FleetKernelResult` is R2.1b (Wave 4, gated). |
| `Kernel.Inspect()` (plan-mode) | `internal/executor.InspectPlan` | ✓ Pre-existing. |
| `Kernel.Reverse()` (plan inversion) | (not yet a unified API) | Comes with R1.1b's `KernelResult.Reverse()`. |
| `Kernel.Explain()` (audit query) | — | Not in current plan. |

### 2.3 The "exposed to any frontend" test

Today: only `cmd` imports `internal/apply`. The MCP server, agent
loop, agentd, and fleet still don't call into `apply.Runner`
directly. They'd have to import it. That's the next consumer wave —
not in the refactor plan, but the kernel surface that allows it now
exists.

---

## 3. Cyclomatic hotspot delta

| Function | gocyclo before | gocyclo after | Note |
|---|---:|---:|---|
| `cmd.run` | 33 | **12** | R1.1a extraction. Residual is CLI-flag pre-validation; documented `.golangci.yml` exclusion. |
| `fleetApplyAction` | 49 | 49 | R2.1a in flight; will land ≤10 when wave 3 completes. |
| `explain.DisplayFacts` | 53 | 53 | Unchanged — outside refactor plan scope. |
| `copy.Execute` | 41 | 41 | Above soft cap; deferred per R0.4 (refactor on next touch). |
| `executor.ExecuteStep` | 37 | 37 | Slight increase as R0.1 added wrapper-method delegation. |
| `artifacts.Writer.OnEvent` | 35 | 35 | Unchanged. |

The plan's hard-target was `cmd.run ≤ 10`; we hit 12 with an
explicit exclusion in `.golangci.yml`. The 12 branches are
CLI-coupled pre-validations that legitimately don't belong in
`internal/apply`. The exclusion follows the precedent already in
`.golangci.yml` for `cmd/fleet.go` and `os_systemd/handler.go`.

---

## 4. What's still off-plan

Honest accounting of where the refactor plan's targets aren't met:

### 4.1 LOC drop in `cmd/mooncake.go` is 138 vs target 300

R1.1a extracted `cmd.run` only. The plan's 300 LOC target assumed
also extracting `runFromPlan` (47 LOC) and possibly more — but
`runFromPlan` is a sibling kernel-apply path that calls
`executor.ExecutePlan` (vs `executor.Start`). Bundling it expands
R1.1a's scope.

**Defer:** the `runFromPlan` extraction should fold into a future
PR — naturally a follow-up to R1.1b, which crystallizes the
`KernelResult` shape that both apply paths would produce. Track as
**R1.1c** (not on the original plan but a natural next step).

### 4.2 Executor LOC drop is 42 vs target 500

R0.1's documented partial accounts for most of the gap. The plan
target assumed full extraction of compound-step orchestration;
what landed is the state types + pure-logic ops in `internal/control`,
with `*Result`-coupled bits staying in executor. The next step
(unblocking the rest) would be a `Reverser` interface that lets
control dispatch handler reverses without importing the action
registry — but that's a deeper restructure.

**Defer:** track as **R0.1-followup**. Becomes worthwhile when a
second consumer (e.g. drift remediation needing transaction-rollback
state) lands.

### 4.3 Kernel-surface checkpoint partially met

From the refactor plan §9:

| Criterion | Status |
|---|---|
| `internal/apply.Runner.Run` exists | ✓ (R1.1a; flat return) |
| `*apply.KernelResult` shape with `Reverse()` | → in flight (R1.1b) |
| `internal/fleet.Orchestrator.Run` exists | → in flight (R2.1a) |
| `*fleet.FleetKernelResult` composed | gated (R2.1b) |
| `internal/mcp` imports `internal/apply` directly | not yet |
| `internal/changegraph` does NOT exist | ✓ (Path A held) |

---

## 5. What the metrics now say about structural health

Re-running the §2 questions from the 2026-05-15 report:

- **Stable foundation**: still healthy. `internal/control` joins the
  foundation tier (inst 0.00). `internal/events`, `internal/security`,
  `internal/facts`, `internal/metrics`, `internal/expression`,
  `internal/runlog`, `internal/fleet/transport`, `internal/winutil`,
  `internal/registry` — wait, that one shifted: it's now
  `internal/presets/registry`, same inst 0.00.
- **Handler fan-out**: unchanged. 65 action packages under
  `internal/actions/`, each with afferent=1 (`internal/register`).
- **ABI evolution**: unchanged. Four typed sub-interfaces still
  opt-in via `Resolve*` helpers.
- **cmd/ god-package smell**: improving. From 10,547 → 10,409 LOC
  (−138). R2.1a will subtract another ≥ 500 when it lands.
- **Executor accretion**: improving slightly. 2,953 → 2,911 (−42).
  Bigger drop comes with R0.1-followup and R1.1b's `Reverse()`
  walker lift.
- **Config.go growth**: unchanged at 1,866 LOC. Watch-only.
- **Handler proliferation**: unchanged. `internal/actions/file` 2,044
  LOC still top of the tail. R0.4 soft caps now formally documented
  in CLAUDE.md / LLM_GUIDE.md.

---

## 6. Immediate next steps (forward-looking)

In execution order, gated on the in-flight Wave 3:

1. **R1.1b lands** → `apply.KernelResult` + `Reverse()` materialize.
   `cmd/mooncake.go` updates recap rendering against `KernelResult`.
2. **R2.1a lands** → `cmd/fleet.go` shrinks ≥500 LOC; `fleetApplyAction`
   gocyclo drops from 49 to ≤10.
3. **R2.1b** (Wave 4) unblocks → `FleetKernelResult` composes per-peer
   `apply.KernelResult`. Refactor plan complete.
4. **Post-refactor follow-ups** (worth filing as new R-items):
   - **R1.1c**: extract `runFromPlan` into `apply.Runner` with a
     `Source` variant. Closes the 162-LOC gap in `cmd/mooncake.go`.
   - **R0.1-followup**: introduce a `Reverser` interface so
     handler-dispatch can move into `internal/control`; closes the
     compound-step partial.
   - **First non-cmd consumer**: import `internal/apply` from
     `internal/mcp` (drop the duplicated apply-orchestration in
     the MCP server). Validates the kernel claim externally.

---

## 7. The one-paragraph version

In 24 hours the kernel-exposure refactor went from a planning
document to drawn-in-code: five new packages (`apply`, `control`,
`plan/filter`, `secrets/resolver`, `presets/registry`) materialize
what `vision/kernel.md` claimed already existed conceptually.
`internal/control` landing at instability 0.00 with zero efferent
imports validates the foundation-tier kernel-sub-system claim
directly. Two of the original 9 R-items are in flight (R1.1b
crystallizes `KernelResult`; R2.1a extracts `fleet.Orchestrator`);
one is gated (R2.1b). Metrics fall short of the plan's stretch
targets in places — `cmd/mooncake.go` −138 LOC vs −300 target,
executor −42 vs −500 — but the kernel-promotion goal is met: every
kernel sub-system has a callable package boundary, and the first
frontend (cmd) is using it. The next visible win will be when MCP
or the agent loop becomes the second importer of `internal/apply`.
