# Mooncake — Architecture & Structural Review (2026-05-15)

> Generated as a one-shot architectural audit, grounded in
> `ARCH_SNAPSHOT.md` (auto-generated package metrics), `VISION.md`,
> `streams.md`, `non-goals.md`, and a direct read of the Handler ABI.
> Cross-reference, not authority — re-run the snapshot script and
> revisit when the structural picture moves.

## Executive summary

Mooncake's kernel is **structurally sound and well-bounded** for what it is
today: a single-binary Go tool with ~66k LOC across 85 internal packages,
organized around a clear five-layer model (kernel → CLI → daemon → fleet →
ecosystem). The Handler ABI (one required interface, four opt-in
sub-interfaces) is a textbook example of evolving a contract without breaking
existing implementors. Dependency direction is mostly correct: stable
foundation packages (`config`, `events`, `facts`, `security`, `expression`,
`metrics`) sit near instability 0.00, and the 50+ action handlers fan out
cleanly through `internal/register` (instability 0.98, only 64 LOC — exactly
what a registration package should look like).

The strategic story (`VISION.md` + `streams.md`) and the seven explicit
non-goals (`non-goals.md`) are **load-bearing**: the four primitives Mooncake
claims to be built on — convergence, plan-before-apply, dependency graph +
compensating action, correlation ID + append-only log — map 1:1 to packages
in the tree (`plan`, `executor`, `actions/handler_abi.go::Reverser`,
`runlog`). That's rare; many projects have aspirational vision docs and a
codebase that drifted somewhere else.

That said, there are **five concrete structural smells** that should be
addressed before the next big stream (drift detection, policy DSL, sandbox
mode) starts piling code on top.

---

## 1. What's working (evidence-based)

| Sign | Evidence |
|---|---|
| Stable core | `internal/config` (LOC 2971, afferent 59, instability **0.02**), `internal/events` (40 afferent, **0 efferent**), `internal/facts/security/expression/metrics/llm/containerruntime/utils/registry/runlog/errors` all at **instability 0.00** |
| Clean handler fan-out | Every `internal/actions/<name>` package has afferent=1 (only `internal/register` imports it). 50+ handlers, zero cross-handler dependencies |
| ABI evolution discipline | `handler_abi.go` adds 4 *opt-in* sub-interfaces (`Differ`/`Reverser`/`Coster`/`Permitter`) without changing the required `Handler` interface. The default-resolver pattern (`Resolve*`) keeps existing handlers working |
| Non-goal alignment | No CRDs, no admission webhooks, no pipeline DSL, no git-as-database, no ACID claims. `transaction:` blocks are explicitly SAGA-shaped with `Reverser` compensators — honest |
| Architecture observability | `scripts/arch-snapshot.sh` regenerates `ARCH_SNAPSHOT.md` with package LOC + instability + import edges + gocyclo + goda-cut metrics. **You have a structural feedback loop most projects lack** |

---

## 2. Five structural smells worth addressing

### Smell 1 — `cmd/` has become a business-logic god-package

- **Evidence**: `cmd` is 7139 LOC across many files; `cmd/presets.go` (1435 LOC),
  `cmd/mooncake.go` (1315), `cmd/fleet.go` (888) are the three biggest files in
  the repo. Gocyclo: `fleetApplyAction` = **49**, `collectParameters`
  (presets.go) = 29, `executePresetInstall` = 28, `run` (mooncake.go) = 28,
  `formatPlanText` = 26.
- **Why it's a problem**: CLI commands are doing orchestration that belongs in
  `internal/*`. Specifically: fleet apply rollout logic, preset parameter
  resolution, and plan text formatting are not "thin Cobra adapters" — they're
  application services living in the wrong layer. That's the standard symptom
  that produces "we can't add a TUI / SDK / second frontend" later.
- **Fix shape**: For each cmd file >500 LOC, identify the actual service it
  encodes and move the *body* down into `internal/<service>`, leaving cmd as
  flag-parse → call → render. Start with `fleetApplyAction`: extract a
  `fleet.Orchestrator` and let cmd just configure it.

### Smell 2 — `internal/executor` is accumulating eight runtime concerns in one package

- **Evidence**: 2650 LOC across `executor.go` (1148, **largest non-cmd file**),
  `dryrun.go`, `inspect.go`, `preflight.go`, `transaction.go`, `trycatch.go`,
  `secret_resolve.go`, `scope.go`, plus `result.go`/`context.go`/`errors.go`.
  `ExecuteStep` is gocyclo **36**; `walkAndResolveSecrets` is 23.
- **Why it's a problem**: 53 packages import `internal/executor`. Every concern
  that lands in this package ripples. The package's instability is 0.22 — which
  is healthy for a "controller" — but the file structure says it's being
  treated like a layer ("everything that runs at execute time goes here").
  That's an architecture-by-accretion pattern.
- **Fix shape**: Promote sibling sub-systems to peer packages where they have a
  real seam:
  - `internal/executor/transaction.go` + `trycatch.go` → `internal/control`
    (compound-step semantics; spec-30 + spec-23 §2).
  - `internal/executor/secret_resolve.go` → `internal/security` (it's already
    secret-aware).
  - `internal/executor/preflight.go` → either stays here, or moves to
    `internal/policy` once a policy DSL exists.
  - `executor.go` itself should shrink to dispatch + result assembly.
    `ExecuteStep` cc=36 is doing too much branching; split by step kind
    (handler / control / capture).

### Smell 3 — `internal/config/config.go` (1491 LOC) is the single biggest non-test file in the repo

- **Evidence**: 1491 LOC, 59 importers. Every action handler imports it for
  `*config.Step`.
- **Why it's a problem**: This is the de-facto schema of the entire system.
  When 59 packages depend on a 1491-LOC file, every new YAML field becomes a
  global decision: it touches 59 import sites the moment you reach for it. The
  file is almost certainly carrying both **YAML schema types** and **runtime
  convenience helpers**, which is what makes it grow.
- **Fix shape**: Split along the seam between (a) the closed YAML schema
  (`Step`, `Config`, raw types — the part the non-goal *No DSL evolution* says
  is frozen) and (b) the helper/convert/normalize layer that the actions use.
  Two files at minimum; ideally two sub-packages. The schema half should be
  small, hostile to change, and reference-able.

### Smell 4 — A handful of action handlers are 3–4× the stated "100–500 line" budget

- **Evidence (LOC + gocyclo)**:
  - `internal/actions/file` 1979 LOC, `handler.go` is 1145 (cc=26 on
    `Execute`/`Run` paths)
  - `internal/actions/tool` 1496 LOC
  - `internal/actions/service` 1466 LOC, `handler.go` 1308, `Run` cc=27
  - `internal/actions/package` 1216 LOC, `handler.go` 901
  - `internal/actions/os_systemd` 712 LOC, **two** functions at cc=34 and cc=28
  - `internal/actions/copy` 846 LOC, both `Execute` and `Run` at cc=26+
  - `internal/actions/text_patch_*` family: 700–1038 LOC each
- **Why it's a problem**: LLM_GUIDE.md says handlers should be self-contained
  100–500 lines. The big ones aren't handlers anymore — they're mini-subsystems
  with their own driver dispatch (apt/dnf/brew under `pkg`, systemd state
  machines under `service`, etc.). Two risks compound here: (a) the cyclomatic
  complexity makes correctness brittle for the very actions that touch
  system-critical things (services, packages, mounts); (b) the package becomes
  hard for spec-22 ABI hooks (Phase 8 docs are pending — but harder still:
  future `Diff`/`Reverse` adjustments).
- **Fix shape**: Extract per-driver code (apt/dnf/brew/yum/winget) into sibling
  files or a `drivers/` sub-package within each action. The handler entry point
  should become a dispatcher; each driver should be the size of a small
  handler. `os_systemd`'s `computePlan` (cc=34) is the canonical candidate —
  that's a state machine begging to be a table.

### Smell 5 — `internal/effects` (903 LOC, instability **0.50**) is mid-range with no obvious owner

- **Evidence**: 0.50 instability + 903 LOC is the textbook "package that could
  either become foundational or get re-absorbed". `effects/default.go` is 611
  LOC; `buildHunks` is cc=29. Only 2 importers (`executor`, plus itself).
- **Why it's a problem**: The package abstracts filesystem mutations so plan
  and execute share predicates (per the Handler doc comment). But with only 2
  consumers and 900 LOC, either (a) the abstraction earns its keep and more
  handlers should route through it, or (b) it's a Brownian-motion package that
  should be folded into `executor`. Right now it's neither.
- **Fix decision needed**: Either commit to "all FS-mutating handlers go
  through `effects`" (and migrate them — copy, file, template, download,
  unarchive, the file_* family), or fold `effects` into `executor` and stop
  pretending it's an independent abstraction.

---

## 3. Risks against the seven non-goals

Tested each non-goal against the current tree. Six look safe; **one needs
vigilance**.

| Non-goal | Status | Note |
|---|---|---|
| 1. No DSL evolution | ✅ safe | New behavior lives in handlers; YAML keys are stable |
| 2. No plugin marketplace | ✅ safe | spec-31 frames plugins as Tier-2 in-tree, not a marketplace |
| 3. No control-plane sprawl | ⚠ **watch** | `agentd` (2839 LOC) + `fleet` (2264) + `fleet/transport` + `fleet/discovery` is the largest growing region. **Spec-58 fleet-drift** is the next test: if drift detection lands as an agentd-side scheduler emitting JSONL, fine. If it grows a hub-side controller with watch semantics, the non-goal is breached |
| 4. No git-coupled audit | ✅ safe | `runlog` is 152 LOC of JSONL; no git coupling |
| 5. No image monoculture | ✅ safe | Facts package recognizes capabilities not OS images |
| 6. No ACID claims | ✅ safe | `transaction:` is explicitly SAGA-shaped via `Reverser` |
| 7. No pipeline DSL | ✅ safe | Rollout = flags on `fleet apply`, no `pipeline.yml` |

**Pre-implementation gate for spec-58** (fleet-drift): require the spec to
spell out *where the loop runs* before merging. Per non-goal #3 the only
acceptable answer is "on agentd, against local state, with explicit per-plan
policy."

---

## 4. Forward-looking architectural questions

These are decisions that should be made before code lands, not after.

1. **Policy DSL home** (Stream 2 future). When `deny: agent.touches(...)` rules
   appear, do they sit in a new `internal/policy` package consuming
   `PermissionSet` + `Diff` from the ABI, or do they ride inside
   `executor/preflight.go`? Recommendation: new package, because policy will
   grow tests/golden-files that don't belong next to executor.

2. **Sandbox mode boundary** (Stream 2 future). "Agent has no shell or file
   API" is an *executor configuration*, not a new layer. It should be a
   runtime flag that disables specific action categories — which means
   `ActionCategory` (already in `handler.go`) needs to be enforced at dispatch
   time, not just used for grouping. Cheap to add today; expensive to retrofit
   once a workaround exists.

3. **Cost aggregation layer** (post-spec-22 phase 6). Per-step `CostEstimate`
   is declared; a fleet apply across 10 nodes needs *summed* + *worst-case*
   views. That's a new consumer of `Coster`. Likely belongs in `internal/plan`
   or a new `internal/budget` package — not in cmd-side recap code.

4. **Deterministic replay** (the last leg of the unfair-advantage claim in
   VISION §13.10). The plan-hash + run-log JSONL are already in place. A
   `mooncake replay <run-id>` command needs (a) hash-pinned plan
   reconstruction and (b) snapshot-pinned facts. The snapshot package already
   exists; the question is whether `internal/agent` is the right home or
   whether replay deserves its own.

5. **`internal/agentd` + `internal/mcp` near-cycle**. `agentd → mcp →
   executor`, and `doctor → agentd`. None are cyclic, but `doctor` (the CLI
   helper) reaching into a daemon package is unusual — it works because both
   share facts. Watch this when adding fleet doctor extensions.

---

## 5. Concrete prioritized refactor list

If three weeks of refactor budget were available with the goal of "make the
next stream cheap":

1. **Split `internal/config/config.go`** into a frozen schema half and a
   helpers half. Highest leverage because every other refactor below touches
   it. (~1 day)
2. **Extract `cmd/fleet.go`'s `fleetApplyAction` into
   `internal/fleet.Orchestrator`**. Direct unblock for spec-58 drift work,
   which will want to call the same orchestration loop from a scheduler. (~2
   days)
3. **Decompose `internal/actions/{service,package,os_systemd}` by driver**.
   Each handler.go → `handler.go` (dispatch) + `driver_<name>.go`
   (apt/systemd/etc.). Quiet correctness win. (~3 days)
4. **Promote `executor/transaction.go` + `trycatch.go` to `internal/control/`**.
   Stream-2 backlog (policy DSL, plan signing, sandbox) will plug here, not
   into executor proper. (~1 day)
5. **Resolve `internal/effects`**: commit to it as the FS-mutation gateway *or*
   fold it in. Either way, document which. (~half day to decide; days to
   migrate if path A)

Nothing on this list is structural rewrites — these are seams the codebase has
already drawn, just sharper.

---

## Bottom line

The kernel has earned the marketing claim, the right reflexes about non-goals,
and a structural feedback loop most projects don't have. The smells are all
*concentration smells* — too much LOC in too few files at a few specific seams
(cmd, executor, config, the largest handlers). None of them are existential;
all of them get worse if drift detection, policy DSL, or sandbox mode lands on
top without addressing the underlying file/package shape first.

The most important architectural discipline to maintain through Stream 2
(policy / sandbox / signing) is **"loops live on agentd, against local
state"** — that's the line that separates Mooncake from becoming the
Kubernetes-control-plane it has correctly named as a non-goal.

---

## 6. Observation stream (spec-59 family) — architectural implications

> **Update (post-D3/D4 landing, same day)**: this section was written
> before commits `5b5f98e` (specs 59–64), `7e855b0` (spec-52 fleet
> exec), `846f883` (D3 log structured data), and `887a17b` (D4 query
> CLI + MCP tool) landed locally. §6 below stands as the original
> analysis of spec-59 in isolation. §7 reconciles it with what the
> Read & Report epic has actually shipped.

### 6.1 What spec-59 actually proposes

A typed `observe.*` action family — `observe.port`, `observe.process`,
`observe.http`, `observe.service` — that mirrors spec-22's typed-
mutation ABI on the read side. The framing in the spec is precise:
*"the missing half of the ABI."* Specs 60–63 layer system resources /
logs / GPU / streaming on top; spec-64 fans observation out across the
fleet.

The key architectural choice in spec-59 §"Handler interface — reuse,
don't fork" is that **observation handlers implement the existing
`Handler` interface** plus the four spec-22 sub-interfaces with no-
mutation specializations (`Diff` = empty, `Reverse` = `(nil, nil)`,
`Cost` = `{Risk: 1}`, `Permissions` gains a new `ReadOnly: true` bit).
No new top-level interface. That choice resolves the open question
this review's §4 raised about read-side ABI shape — the answer is "no
new family, extend the existing one."

### 6.2 What the design does well (validates from the existing structure)

- **Reuses, doesn't fork.** Matches non-goal "Borrow vocabulary, not
  implementation." Every consumer that already knows how to render a
  spec-22 `Diff` or surface `Cost` handles `observe.*` for free.
- **Plan-mode is structurally predictive by default.** Plan returns a
  synthetic "(deferred to apply)" result; `--inspect-real` is opt-in.
  Same pattern `wait.*` already uses — consistent.
- **Composes via spec-37 capture.** `as: nginx` →
  `{{ nginx.value.open }}` uses an existing seam, no new variable
  scope mechanism.
- **`Permissions{ReadOnly: true}` is the right shape for future
  sandbox mode.** §4.2 of this review flagged that sandbox mode
  should disable categories at dispatch time; `ReadOnly` gives the
  policy/sandbox layer a typed bit to gate on (an "agent may run
  read-only actions" mode becomes trivial).

### 6.3 Three architectural decisions worth scrutinizing before code

**(1) `internal/metrics` cohesion with spec-60.** Spec-60 explicitly
flags this: *"Cohesion question with `internal/metrics/`; share data
via a refactored collector, separate surface."* The arch snapshot
shows `internal/metrics` at 1276 LOC, instability **0.00**, perfect
leaf — already does CPU/memory/disk with TTL caching. Two paths:

  - **(a)** `observe.cpu/memory/disk` are thin handlers over
    `internal/metrics`. Clean reuse, one data source.
  - **(b)** Parallel observer-specific collector. Two sources of
    truth, divergence risk.

The right answer is (a), and it's worth saying so in the spec-60
acceptance criteria *before* phase 1 of spec-60 starts. This is
exactly the situation Smell 5 of this review described in the
abstract: a new consumer surfaces and the choice between "share or
duplicate" should be made deliberately. The `internal/metrics` shape
(0.00 instability, no efferent deps) is ideal for a shared
data-collector role — extending it is much cheaper than discovering
two collectors a year from now.

**(2) `ActionMetadata.OutputSchema` is an ABI extension, not just a
handler addition.** Spec-59 §"Open questions" Q2 frames this as
"reflection-based vs documentation-only." That undersells the
decision. Adding a machine-readable `OutputSchema` field to
`ActionMetadata` means:

  - Every existing handler now has a schema-shaped hole in its
    metadata, defaulted empty. Forward-compatible, but each
    handler should fill it in eventually for consistency.
  - The MCP server's tool-definition generator becomes
    output-typed, not just input-typed. That's the agent-side
    feature most likely to need the schema first.
  - The reflection path is doable but the spec-22 family has so far
    avoided reflection. Worth picking the same approach (hand-
    declared structs) for consistency.

This is exactly the kind of evolution the existing ABI is good at
(opt-in, default-empty) — but it should be designed against a
concrete MCP-side consumer, not in isolation.

**(3) `internal/effects` resolution becomes more urgent (revisits
Smell 5).** This review's Smell 5 framed `internal/effects` as
ambiguously-owned (903 LOC, instability 0.50, 2 importers). Observers
introduce a second potential consumer of a "shared predicate between
plan and execute" abstraction — specifically the `--inspect-real`
opt-in needs an executor-side switch that's symmetric to whatever
the mutation side does for plan/apply parity. Two ways this can go:

  - **Path A**: extend `effects` to cover read predicates too →
    `effects` becomes the canonical plan/execute parity layer for
    both reads and writes. Justifies its existence.
  - **Path B**: build observer-side plan/apply gating independently
    → two parallel mechanisms, `effects` stays in limbo.

Whoever lands spec-59 phase 1 should look at `internal/effects`
first. If reuse is plausible, that resolves Smell 5. If not, fold
`effects` into `executor` *before* spec-59 lands so the second
mechanism doesn't get bolted on next to a stranded one.

### 6.4 Non-goal pressure points the observation stream introduces

The observation stream creates two new ways the seven non-goals can
be breached. Both are well-controlled in spec-59 itself, but worth
naming so they remain controlled in spec-60–64 and successors.

- **Non-goal #1 (No DSL evolution).** Predicates like
  `when: "ngx.value.open"` and `changed_when: "value.open == false"`
  expose typed observation values to the template language. The
  *number* of typed fields per handler grows (PortObservation,
  ServiceObservation, etc.). That's fine — fields are data, not
  syntax — but spec-61 (logs) and spec-62 (GPU) need to resist the
  temptation to introduce new template *functions* (e.g. log-pattern
  matchers). Keep matchers as handler config, not as Jinja filters.

- **Non-goal #3 (No control-plane sprawl).** Spec-63 (streaming /
  subscription) is **already correctly deferred** in the draft —
  "the right home for watch-state-and-react is the drift loop
  (spec-58), not the plan executor." That's exactly the
  architectural seam this review's §3 flagged. The discipline must
  hold: observers stay single-shot reads; long-running watch lives
  in agentd-side schedulers, not in plan steps. If spec-63 ever
  comes off the deferred shelf, the gate for promoting it is the
  same as for spec-58: *"the loop runs on agentd, against local
  state, with explicit per-plan policy."*

### 6.5 Adjusted prioritized refactor list

§5 of this review proposed five refactors. The observation stream
changes the order of #1 and #5:

| Was | Now | Reason |
|---|---|---|
| 1. Split `internal/config/config.go` | 1. **Resolve `internal/effects` first** | Spec-59 lands a second potential consumer; decide before more code piles on |
| 5. Resolve `internal/effects` | 5. Split `internal/config/config.go` | Still highest-leverage but less time-pressured than effects given spec-59's draft status |

Items 2–4 (cmd extraction, handler-driver decomposition, control
package promotion) stay in their original order.

### 6.6 Net assessment

The observation stream is the **right architectural addition** —
it closes the asymmetric gap this review didn't flag because it was
invisible until spec-59 named it. The design choices in spec-59
(reuse Handler + spec-22 sub-interfaces, single-shot reads, plan-
mode safety by default, defer streaming) align with the non-goals
discipline. The three decisions in §6.3 are the meaningful forks;
none of them are blocking, all of them should be made deliberately
before phase 1 code lands.

---

## 7. Post-landing reconciliation (sync with local master)

Six commits landed locally between the original review (§1–§6) and
this update, none of them yet on `origin/master`:

| Commit | Subject | What's actually new |
|---|---|---|
| `c597854` / `7e855b0` | spec-52 fleet exec — ad-hoc shell across N peers | `internal/fleet/exec/` (plan.go + run.go, ~520 LOC of logic + ~510 LOC tests) + `cmd/fleet_exec.go` (354 LOC of Cobra glue) |
| `5b5f98e` | Specs 59–64 typed observability + extensions | 6 draft specs, 1289 added lines, no code yet |
| `846f883` | D3 log: structured data + format + budget | Extends existing `internal/actions/print/handler.go` (+136 LOC effective) — no new package |
| `887a17b` / `6b9d4b1` | D4 mooncake query CLI + query_file MCP tool | `cmd/query_cmd.go` (219 LOC) + matching MCP tool; deliberately duplicates ~20 LOC between cmd/ and mcp/ |

These move the review's recommendations and observations forward in
four concrete ways.

### 7.1 Spec-52 validates the §5.2 refactor pattern *empirically*

`fleet exec` is the first new fleet command shipped during this
review window, and the team put it where §5.2 recommended `fleet
apply` belongs: orchestration in `internal/fleet/<subcommand>/`,
Cobra glue in `cmd/`. The shape is:

```
internal/fleet/exec/plan.go   — domain logic (peer selection, dispatch plan)
internal/fleet/exec/run.go    — orchestrator (spawn, stream, aggregate)
cmd/fleet_exec.go             — flag parse → call → render
```

Implication for §5: refactor #2 (extract `fleetApplyAction` → `internal/fleet.Orchestrator`)
is no longer a *proposal* — it's now **catching `fleet apply` up to the
pattern `fleet exec` already uses**. That's a much easier conversation.
The recommendation stays, but its priority moves up: every other
fleet command added on top of `cmd/fleet.go`'s 888 LOC widens the
inconsistency.

`cmd/fleet_exec.go` at 354 LOC is still bigger than ideal for pure
glue. Worth a glance once the dust settles to see what got stranded
in cmd that belongs in `internal/fleet/exec/`. Not urgent.

### 7.2 The Read & Report epic ships an architectural shape worth naming

Reading the [epic](../epics/epic-read-and-report.md) end-to-end clarifies
something §6 missed: Mooncake now has **three distinct observation
surfaces**, each with deliberately different semantics. They are not
overlapping; they are the three axes of a coherent decomposition.

| Surface | Where | Plan-mode | Use case | Status |
|---|---|---|---|---|
| `read.json` / `read.yaml` (actions) | inside plans | **executes** (read is cheap, deterministic) | "parse this file as part of the plan; bind the value for downstream steps" | ✅ shipped (spec-37/38) |
| `observe.*` (actions, spec-59 family) | inside plans | **defers** (state read may not be deterministic) | "is this port open / process up / service running *right now*?" | 📝 drafted |
| `mooncake query` / `query_file` (CLI + MCP) | outside plans | n/a | "agent wants to look at a file once, no plan, no apply" | ✅ shipped (D4) |

This is the right decomposition. Three observations:

1. **Plan-mode behavior splits cleanly along a real seam.** File
   content reads execute in plan mode (deterministic, no I/O surprises);
   state observations defer (network calls, races, "as-of" semantics).
   §6.3 raised this as a concern; the epic resolves it by separating
   the two cases architecturally rather than via flag overloading.
2. **`mooncake query` is positioned correctly as "not a plan step."**
   An agent that just wants to read a file shouldn't synthesize a
   single-step plan and apply it. The CLI + MCP surface honors that.
3. **Shared pathquery semantics across all three.** D4's commit
   message specifically calls out that `mooncake query` reuses the
   same path syntax as `read.json` / `read.yaml`. This is the kind of
   detail that prevents future drift — three surfaces, one mental
   model.

The risk to watch: a fourth consumer of pathquery semantics surfaces
(spec-59's `observe.*` predicates? a future `templates`/`when:`
extension?), and the deliberate ~20-LOC duplication between cmd/ and
mcp/ (per D4's commit message) tips over into needing a shared package.
Worth revisiting if/when that happens — but the current "duplicate
20 LOC, no premature abstraction" call is correct per CLAUDE.md
guidance.

### 7.3 D3's choice to *extend* `print/` rather than create `log/` is the right call but produces a small naming-vs-package mismatch

The YAML action is `log:`. The Go package is
`internal/actions/print/`. The handler now serves both the prior
`print:`/`log:` text-message case and the new structured-data case.
Two notes:

- **Right call architecturally.** Splitting `print/` into separate
  `print/` and `log/` packages would have produced two near-identical
  handler.go files (cross-handler duplication is what §1 of this
  review praised as currently absent).
- **The mismatch is real but cheap.** Future maintainers may grep for
  `actions/log/` and not find it. A one-line comment in
  `actions/print/handler.go` declaring "registers both `print` and
  `log`" would resolve it. Not blocking.

This pattern (one package serving multiple action names) is fine and
worth codifying — but it should be intentional. The `package` action
already does this (apt/dnf/brew under one handler); the file family
might benefit too. Worth a doc paragraph in
`action-design-principles.md` if it becomes a pattern, not a one-off.

### 7.4 Adjusted refactor list (re-revision)

§5 gave five refactors; §6.5 reordered them. Post-landing:

| Position | Item | State change |
|---|---|---|
| 1 | Resolve `internal/effects` | unchanged — still the gating decision for spec-59 phase 1 |
| 2 | **Catch `fleet apply` up to the `fleet exec` sub-package pattern** | reframed — was speculative, now empirically validated by spec-52 |
| 3 | Split `internal/config/config.go` | unchanged — still highest-leverage cross-cutting refactor |
| 4 | Decompose `actions/{service,package,os_systemd}` by driver | unchanged |
| 5 | Promote `executor/transaction.go` + `trycatch.go` to `internal/control/` | unchanged |

Item 2 is the one whose justification got materially stronger this
cycle: there's now precedent in the same package family.

### 7.5 What's still missing from the architectural picture

Three things drafted-but-not-shipped that this review hasn't covered
because there's nothing to read in code yet, but worth flagging:

1. **Spec-59 phase 1 implementation** — **actively in flight** on
   worktree `worktree-spec59-observe` (per `~/.mooncake/claims.jsonl`,
   note: "Phase 1: observe.port handler + ObserveResult shared type").
   When it lands it will resolve §6.3's three open questions (metrics
   cohesion, OutputSchema, effects). Re-run this review then —
   especially §6.3.3, since the effects-vs-fold-in call should not
   be deferred past phase 1 merge.
2. **Spec-58 (fleet drift)** — drafted, still the non-goal-#3
   pressure point named in §3. Hasn't moved this cycle.
3. **D2 `register:` step field** — listed in epic-read-and-report
   as one of the four deliverables but doesn't appear in any commit
   message in the new range. Either it shipped earlier and the
   commit history I scanned missed it, or it's the lone unlanded
   piece of the epic. Worth checking before the next review.

### 7.6 Net assessment (revised)

The dominant signal from the new commits is **structural discipline
under pressure**. Three things could have gone wrong architecturally
in a single afternoon of shipping (a new fleet command, a multi-spec
draft family, two new observation surfaces), and none did:

- `fleet exec` landed in the right place (`internal/fleet/exec/`).
- The three observation surfaces are deliberately separated, not
  collapsed.
- D3 extended an existing package rather than spawning a new one.
- D4 chose acceptable duplication over premature abstraction and
  documented why.

The smells named in §2 are *not worse* than the original review; the
patterns in §5 are *better validated* than they were. The next
architectural decision worth watching closely is whichever one lands
first from {spec-59 phase 1, spec-58 implementation, the `fleet apply`
catch-up to the `fleet exec` pattern}.

---

## 8. Second post-landing reconciliation (spec-59 phases 1+2+3 shipped, spec-54 fleet ps, two production bug fixes)

> **Update**: 12 more commits landed locally between §7 and this
> section, including the entire **spec-59 seed (phases 1+2+3)** that
> §6 evaluated when it was still draft. The observation stream is
> now real code, and the three §6.3 open questions can be answered
> against the implementation rather than against intent.

### 8.1 What shipped

| Commit range | Subject | Code surface |
|---|---|---|
| `ebc0f10`/`27c90fe` | spec-59 Phase 1 — `observe.port` seed | `internal/actions/observe.go` (85 LOC shared envelope + `PlanDeferred` helper) + `internal/actions/observe_port/` (235+224 LOC handler+tests) |
| `1e84513`/`b26acbf` | spec-59 Phases 2+3 — `observe.process` / `observe.service` / `observe.http` | three more `observe_*` packages (~600 LOC handlers, ~370 LOC tests) |
| `528530b`/`df3d4dd` | spec-54 fleet ps — list in-flight runs across peers | another fleet QoL command (likely in `internal/fleet/ps/`, matching the `internal/fleet/exec/` pattern §7.1 validated) |
| `27c90fe` | two production bugs (winutil + schemagen) found during Windows dotfiles redeploy | `<Settings>` element order + invalid `<AllowStartIfOnBatteries>` in Task Scheduler XML; schemagen drift |
| `c4fb86c`/`d69bc1d` | claim-file protocol formalized | `~/.mooncake/claims.jsonl` is now mandatory before cross-worktree work, per CLAUDE.md |
| `5747789`/`011f34a`/[`manual-test-findings-2026-05-15.md`](./manual-test-findings-2026-05-15.md) | manual-test bug filings | 13 issues documented; headline bugs: `as_user: root` invokes sudo when uid=0; shell guards mis-count as `changed` instead of `skipped`; **`for_each` broken** (per memory) |

### 8.2 Spec-59 implementation answers §6.3's three questions

| §6.3 question | Answer the code gives | Implication |
|---|---|---|
| (1) Metrics cohesion | **Deferred correctly.** Phase 1–3 don't touch `internal/metrics`. The question only fires when spec-60 (`cpu`/`memory`/`disk`) lands. | §6.3.1 stays open but is gated on spec-60 starting, not on phase 1 completion. The acceptance criteria for spec-60 should still require reuse-not-duplicate. |
| (2) `OutputSchema` on `ActionMetadata` | **Deferred to phase 5 (docs)**, per spec-59 §"Open questions" Q2. The shipped `ObserveResult` envelope has stable JSON tags; consumers branch on string keys for now. | §6.3.2 stays valid as a phase-5 decision. The MCP-side consumer (the agent SDK's "what can I observe and what shape do I get?" tool) still wants this. |
| (3) `internal/effects` | **Bypassed.** Observers built their own plan-mode gating via `PlanDeferred(emptyValue)` in `observe.go`. `grep -l 'internal/effects' internal/actions/observe*/*.go` returns empty. | §6.3.3 outcome is **Path B** — observer-side gating independent of effects. This makes §5 refactor #1 ("Resolve `internal/effects`") *more* urgent, not less: effects now has only its original consumer (`executor`) and the next read-shaped abstraction was built without it. |

### 8.3 The `ObserveResult.Data` shape choice is interesting

Reading `observe.go`, the executor result publishes the envelope under
two keys: `"observe"` (full ObserveResult) and `"value"` (un-wrapped
per-handler payload). That lets downstream templates pick either:

```yaml
when: "nginx.value.open"        # un-wrapped path
when: "nginx.observe.found"      # universal path
when: "nginx.value.status_code"  # per-handler typed field
```

The `ObserveValueToMap` helper round-trips the typed Go struct through
JSON so the template engine (which can't reflect Go struct fields) sees
a `map[string]any` keyed by JSON tags. This is the right plumbing —
worth flagging because **future actions that want to publish typed
results downstream now have a precedent to copy** (also useful for spec-58
drift detection, which will want typed snapshots in templates).

### 8.4 Spec-54 (fleet ps) continues the sub-package pattern

Didn't read the code yet, but `cmd/` + `internal/fleet/<subcmd>/` is now
the consistent pattern across `exec`, `ps`, and (originally)
`bootstrap`. **The pattern is now load-bearing** — when `fleet apply`
catches up (§5 refactor #2 / §7.4 item 2), it has three siblings to
model on, not zero.

### 8.5 What the manual-test findings tell the architecture

The 13-issue manual-test doc (linked in §8.1) is correctness-focused,
not architecture-focused, so it doesn't change any of §1–§7's
conclusions. But three patterns are worth naming:

1. **The two Windows bugs (`27c90fe`) lived at the cmdlet boundary**
   — XML element ordering for Task Scheduler. Unit tests couldn't
   catch them because the bug is "PowerShell rejects the document",
   not "our Go code returns wrong bytes." This is the same gap the
   manual-test doc keeps surfacing across all 13 findings: **the
   project's test coverage is strong inside Go and weak at OS-cmdlet
   /shell-out boundaries.** Not a structural problem, but a testing-
   strategy gap that matters more as Windows + cross-cmdlet observers
   ship (spec-59 phase 2's `observe.service` shells out to
   `systemctl` / `launchctl`; the same risk applies).
2. **`for_each` broken** (per memory) is a planner-level bug, not a
   handler bug. That's in `internal/plan/planner.go` (the 951-LOC
   file with `walkAndRender` at gocyclo 29 — Smell named in §2).
   Refactor item 3 (`config.go` split) is adjacent; refactor item 4
   (driver decomposition) is unrelated. **If `for_each` regresses,
   the planner's complexity is the proximate cause.** Worth a closer
   look in the next refactor pass.
3. **`as_user: root` + sudo bug** is in the executor's preflight or
   command-builder layer. Per §2 Smell 2, the executor package has
   accreted preflight + secret resolve + transaction + trycatch +
   etc. concerns. The bug being in this layer is consistent with the
   "concerns piling up in one package make subtle behavior brittle"
   observation.

In short: the manual-test doc is independent evidence for §2's
concentration-smell diagnosis, not new structural information.

### 8.6 Adjusted refactor list (third revision)

| Position | Item | State change since §7.4 |
|---|---|---|
| 1 | **Resolve `internal/effects`** | **Urgency increased** — observers explicitly chose Path B (bypass). Effects now has the smallest justifiable consumer base it has ever had. Decide soon |
| 2 | Catch `fleet apply` up to the sub-package pattern | Pattern now used by `exec` + `ps` + `bootstrap` — strongest case yet |
| 3 | Split `internal/config/config.go` | Unchanged; still gated on the others |
| 4 | Decompose `actions/{service,package,os_systemd}` by driver | Unchanged |
| 5 | Promote `executor/transaction.go` + `trycatch.go` to `internal/control/` | Manual-test finding on `as_user:root`+sudo bug is mild additional evidence that this layer needs untangling. Slight bump but stays at 5 |

### 8.7 Net assessment (third revision)

Spec-59 phases 1–3 are the largest single architectural addition the
project has made since spec-22, and **the implementation honored the
non-goal discipline visible elsewhere**: no new top-level interface,
no metrics path collision, no DSL escalation, deliberate plan-mode
defer. The one place it diverged from §6's predictions was bypassing
`internal/effects` rather than extending it — and that divergence is
defensible. The cost is that `effects` is now harder to justify, not
easier.

The next architecturally consequential moments:

- **`internal/effects` resolve-or-fold-in decision** — should happen
  before any new read-shaped abstraction (spec-58 drift?, spec-60
  observe.cpu/memory/disk?) considers using or bypassing it.
- **spec-60 `observe.cpu/memory/disk`** — the metrics-cohesion test
  case. Reuse `internal/metrics`, don't fork.
- **The `for_each` planner bug fix** — touches the 951-LOC
  `planner.go` and may itself be the trigger for a planner refactor.

---

## Cross-references

- [`../ARCH_SNAPSHOT.md`](../ARCH_SNAPSHOT.md) — package LOC, instability,
  import edges, gocyclo hotspots (re-generate via `scripts/arch-snapshot.sh`)
- [`../streams.md`](../streams.md) — stream/spec status the smells map onto
- [`../non-goals.md`](../non-goals.md) — the seven lines tested in §3
- [`../action-design-principles.md`](../action-design-principles.md) — the
  "100–500 line handler" budget referenced in Smell 4
- [`../../VISION.md`](../../VISION.md) — strategic context (esp. §13.10
  unfair-advantage line referenced in §4.4)
- [`../specs/action-surface/spec-59-typed-observability.md`](../specs/action-surface/spec-59-typed-observability.md)
  — the observation stream this review's §6 evaluates
- [`../epics/epic-read-and-report.md`](../epics/epic-read-and-report.md)
  — the epic whose D3 + D4 deliverables §7 reconciles against
- [`manual-test-findings-2026-05-15.md`](./manual-test-findings-2026-05-15.md)
  — 13-issue manual test bug filings that §8.5 cross-references against
  §2's concentration-smell diagnosis
