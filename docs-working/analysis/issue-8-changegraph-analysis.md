# Issue #8 — ChangeGraph as Mooncake's core primitive (analysis)

**Source:** [#8 Explore ChangeGraph as Mooncake's core primitive for autonomous agents](https://github.com/alehatsman/mooncake/issues/8)
**Analyzed against:** master @ `c6f6838` (2026-05-15)
**Author:** alehatsman, 2026-05-15

Issue #8 is a vision-shaped brainstorm: reframe Mooncake's primitive
from `action → execute` to `intent → compile → ChangeGraph → validate
→ simulate → execute → observe → record`. Ten themed sections, one
suggested first slice, one set of non-goals.

**Headline finding:** about **60% of the proposal is already shipped
or already drafted under `docs-working/`**, mostly under a different
name. The "ChangeGraph" framing is largely a *narrative rebrand* of
the typed-mutation ABI that spec-22 phases 1–6 already landed; the
graph-with-typed-edges piece is genuinely new but mostly cosmetic
until a consumer needs `depends_on` / `conflicts_with` as first-class
metadata. The genuinely-new mechanisms in the issue are **typed
observability** (`observe.*`), **explainability / provenance**,
**discover → generated config**, and **rewind**. The first three of
those are already named as strategic bets in
[`clustermanagement/agentic-interface-brainstorm.md`](../clustermanagement/agentic-interface-brainstorm.md).

Net: the issue is **worth keeping as a positioning document** more
than as a re-architecture program. Adopt the ChangeGraph *vocabulary*
where it sharpens the agent-runtime pitch; cherry-pick the four
genuinely-new items as standalone specs in the order their
prerequisites already imply. Do not rebuild the kernel around a graph
abstraction that current plans don't yet need.

---

## At a glance

| § | Capability                                | State              | Existing pointer                                                                 |
|---|-------------------------------------------|--------------------|----------------------------------------------------------------------------------|
| 1 | ChangeGraph as core primitive             | 🟡 partial         | spec-22 typed-step ABI; graph-edges absent                                       |
| 2 | State as initial + sum of changes         | 🟡 partial         | run-history (spec-08); replay not implemented                                    |
| 3 | Time travel / rewind                      | 🟡 partial         | per-step `Reverse` + spec-30 transactions; whole-run rewind absent               |
| 4 | Explainability engine (`explain <res>`)   | ❌ not started     | named `fleet why` in [agentic doc §4](../clustermanagement/agentic-interface-brainstorm.md) |
| 5 | Discover → generated config               | ❌ not started     | spec-04 snapshot is the read-side foundation                                     |
| 6 | Risk engine (`mooncake risk <plan>`)      | 🟡 partial         | spec-22 Phase 6 `Cost.Risk` (1–10) shipped per-step; plan-level renderer absent  |
| 7 | `observe.*` action family                 | ❌ not started     | named **strategic bet #1** in [agentic doc §1](../clustermanagement/agentic-interface-brainstorm.md) |
| 8 | Rehearsal environments                    | 🟡 partial         | `mooncake plan` / `inspect` (spec-15/16) is logical rehearsal; sandbox VM not    |
| 9 | Agent negotiation MCP layer               | 🟡 partial         | spec-10 MCP shipped; spec-22 Phase 7 wires Diff/Cost/Permissions into MCP        |
| 10| Autonomous maintenance / invariants       | 📝 drafted         | [spec-58 fleet-drift](../specs/personal-fleet/spec-58-fleet-drift.md)            |

Legend: ✅ shipped · 🟡 partial / shipped-but-narrow · ❌ not started · 📝 drafted (spec exists, not implemented).

---

## Per-section detail

### §1. ChangeGraph as core primitive  🟡 partial

**Issue asks:** typed graph of mutations with nodes (install /
deploy / firewall / restart / write / assert) and edges (`depends_on`,
`conflicts_with`, `reverses`, `causes`, `requires`). All higher-level
features (transactions, rollback, fleet orchestration, policy, risk,
simulation, audit) derive from this graph.

**Today:**

- `internal/plan` already produces a *list* of typed Steps that an
  executor consumes. Each Step carries a typed payload and routes to
  a handler implementing the `Handler` interface
  (`internal/actions/handler.go`).
- spec-22 phases 1–6 (✅ shipped) bolt the four ABI methods the
  issue is implicitly demanding onto each handler:
  `Differ` (typed before/after), `Reverser` (inverse Step), `Coster`
  (Risk 1–10 + Resources + Bytes + Reversible), `Permitter`
  (Sudo / Network / RequiredBinaries / FilesystemWrite).
  See `internal/actions/handler_abi.go:122` for `CostEstimate` and
  `:181`/`:207`/`:213`/`:218` for the four sub-interfaces.
- spec-30 transactions (✅ shipped) gives LIFO reverse-on-failure
  across a Step sequence —
  `internal/executor/executor.go` + `executor_transaction_test.go`.

**Gap:** what's missing vs the issue's framing is the **graph
shape itself** — `depends_on` / `conflicts_with` / `causes` /
`reverses` as first-class typed edges on the plan. Plans are
sequential today; `Reverse()` is a per-step method, not an edge to
another node. A `mooncake plan --graph` JSON emitter is cheap
(~150 LOC) once a consumer needs it.

**Verdict:** the *primitives* the issue asks for are already there
under different names. The graph-of-typed-edges framing is a useful
**vocabulary upgrade** for the agent-runtime pitch and an obvious
fit for `mooncake plan --format graph`, but it doesn't unlock new
mechanism. Don't rewrite the kernel around it; expose what's already
there in graph shape when a consumer (UI, MCP client, policy DSL)
materializes.

### §2. Machine state as initial state + sum of changes  🟡 partial

**Issue asks:** `machine_state(t) = initial_state + replay(changes)`
— event-sourced system state.

**Today:**

- spec-08 (✅ shipped) appends every run to `~/.mooncake/runs.jsonl`.
- spec-03 (✅ shipped) emits per-step JSONL events.
- spec-04 (✅ shipped) takes facts snapshots; spec-14 (✅ shipped)
  diffs snapshots over time.
- `internal/runlog/` is the local-mode record; `internal/agentd/`
  has the per-run store (`store.go` / `jsonl_sink.go`).

**Gap:** the **replay** half. We record what happened; we don't
re-derive state from `initial + Σ(changes)`. Doing so cleanly
requires every `Reverser` to be perfectly inverse and every action
to be deterministic against captured facts — both are partial today
(spec-22 phase 5 ships Reverse on 9/11 priority handlers;
`os.service` and `file.unarchive` explicitly refuse).

**Verdict:** the *audit trail* this section wants is shipped. The
*deterministic replay* it implies is overstated as a foundational
primitive; the on-demand `mooncake inspect` path (spec-16) is the
realistic version. Worth keeping as a long-arc goal — not as a
load-bearing claim.

### §3. Time travel / rewind  🟡 partial

**Issue asks:** `mooncake rewind --to "2026-05-15 13:44"` —
semantic reverse of every step taken since timestamp T, in
graph-derived order.

**Today:**

- spec-22 phase 5 (✅ shipped) — per-step `Reverse()` on
  file/text/copy/template/download/pkg handlers. `os.service` +
  `file.unarchive` declare `Reverser` as explicit refusals (tracked
  follow-ups).
- spec-30 transactions (✅ shipped) — automatic LIFO rollback of
  the steps **inside a single transaction**.

**Gap:** rewinding **across runs** (not within one transaction).
Needs:

- Persist `Result.ReverseData` per applied step in `runs.jsonl` (or
  agentd's per-run store), not just in-process for the duration of
  one transaction.
- A `mooncake rewind` planner that walks the history backward,
  rebuilds the reverse Step list, and applies it as a synthetic
  forward plan.
- The two refusal cases (`file.unarchive`, `os.service`) need real
  reverse implementations before rewind is sound; otherwise rewind
  fails irrecoverably on any history containing them.

**Verdict:** real new feature. Useful demo, but **gated on
finishing spec-22 phase 5's refusal cases** + a persistent
reverse-data store. Not a near-term bet.

### §4. Explainability engine (`mooncake explain <resource>`)  ❌ not started

**Issue asks:** `mooncake explain nginx` → introduced-by, reason,
approved-by, dependencies.

**Today:** nothing. Run history is indexed by run, not by `(host,
resource_name)`. spec-22's `Diff` shape gives us the typed
before/after material, but nothing indexes it by resource.

**Pointer:** same primitive surfaces in
[`issue-11-analysis.md` §11](../clustermanagement/issue-11-analysis.md)
("Explain node/resource state") and in
[`agentic-interface-brainstorm.md` §4](../clustermanagement/agentic-interface-brainstorm.md)
("`fleet why` — git-blame for system state"). The data substrate is
the same: `runs.jsonl` + per-step Diff payloads + an index keyed by
resource.

**Verdict:** real new feature, well-scoped, **moderate effort**
(~200–300 LOC: indexer + renderer). High operator value
("why is /etc/wsl.conf shaped like this?"). Probably the
single highest-leverage spec from issue #8 *that doesn't already
have a draft*.

### §5. Discover → generated config  ❌ not started

**Issue asks:** `mooncake discover` — turn an unmanaged machine
into a generated YAML config (packages, services, ports, users,
containers, GPU/CUDA, AI models, dotfiles, cron / systemd timers).

**Today:**

- spec-04 (✅ shipped) `mooncake snapshot` emits compact YAML/JSON
  of facts + tool inventory + service state. Read-only.
- spec-14 (✅ shipped) `snapshot --diff` for drift tracking.

**Gap:** `snapshot` describes; `discover` would *generate executable
config* from that description. The translation step is the real
work — each discovered fact has to compile back into the right
typed action (installed packages → `pkg.install`; running services
→ `os.service`; non-default users → `os.user`; cron entries →
`os.cron`; etc.). 13+ action handlers × per-handler "emit YAML"
shape.

**Verdict:** big standalone spec. **Strategic value: high** —
it's the canonical adoption path ("point Mooncake at my existing
box, get a starting config"). Worth a dedicated spec when adoption
is the goal; not a prerequisite for any other item.

### §6. Risk engine (`mooncake risk <plan>`)  🟡 partial

**Issue asks:** `risk = permissions × blast_radius × reversibility
× dependency_depth` — a single command outputs a risk score with
touches / requires / rollback availability.

**Today:**

- spec-22 phase 6 (✅ shipped) — `CostEstimate{Risk:1..10, Resources,
  Bytes, Reversible}` on every priority handler. Risk bands already
  match the issue's intent: `1–3 safe`, `4–6 routine`, `7–9 high
  impact`, `10 destructive` (see `internal/actions/handler_abi.go:122`).
- `mooncake plan` text output already prints a per-step
  `cost: risk N (band) • reversible • R resources • B bytes` line
  under each would-change step, plus a plan summary
  `max-risk=N (band)` aggregate.
- spec-22 phase 3 (✅ shipped) — `PermissionSet` on every priority
  handler (Sudo / Network / RequiredBinaries / FilesystemWrite).

**Gap:** there's no dedicated `mooncake risk plan.yml` subcommand
that renders a one-screen scorecard for non-plan-reading humans
(the issue's example output). Mechanically: a CLI shim over the
existing plan-time evaluator. ~50–80 LOC.

**Verdict:** ~90% shipped. The remaining sliver is a CLI renderer
and one demo. Trivially picked up alongside any spec-22 phase-7
work (MCP wiring) so the same risk shape lands in both surfaces.

### §7. `observe.*` action family  ❌ not started

**Issue asks:** typed observation primitives — `observe.cpu`,
`observe.memory`, `observe.logs`, `observe.process`, `observe.http`,
`observe.gpu`, `observe.diff`, `observe.port`, `observe.disk`,
`observe.service`. Closes the `observe → reason → propose change →
validate → execute → observe result` loop.

**Today:** nothing under this name. Adjacent primitives exist
piecemeal:

- `internal/facts/` — *static* descriptors (os, cpu_cores, etc.).
- `internal/metrics/` — *live* utilization (cpu_usage_pct,
  load_avg, gpu_metrics) with TTL caching.
- `assert` action — pass/fail without typed output.
- `wait.*` family (spec-29 ✅ shipped) — polling for state to
  reach a target (closest cousin to `observe`).
- spec-37 + spec-38 (drafted) — step output capture +
  `read.json` / `read.yaml`. These close part of the gap (read
  side of file state) but not the full typed-observation surface.

**Pointer:** named **strategic bet #1** in
[`agentic-interface-brainstorm.md` §1](../clustermanagement/agentic-interface-brainstorm.md):
*"It's the missing half of the ABI — mutation is solved,
observation isn't. It unblocks almost every other ambitious idea on
this list."* Issue #8 reinforces the same call from a different
angle.

**Verdict:** **the load-bearing new primitive in issue #8**.
Implementable the same way the action surface was built: one
`observe.X` handler at a time, each ~100–200 LOC, each independently
useful. Closes the agent loop. Should be the next stream-1 spec
**after** spec-37 + spec-38 land (which is already the recommended
order in `streams.md`).

### §8. Rehearsal environments (`mooncake rehearse`)  🟡 partial

**Issue asks:** `mooncake rehearse deploy.yml` — apply in a
temporary namespace / container / VM / Lima / WSL sandbox; run
assertions; report predicted effects.

**Today:**

- spec-15 + spec-16 (✅ shipped) `mooncake plan` / `inspect`
  produce a state-aware preview without side effects. This is the
  *logical* rehearsal: "given the current state, what would change?"
- The handler ABI already exposes structured Diff per step, so
  the rehearsal output is already typed.

**Gap:** rehearsal in a **separate sandbox host** (Lima VM,
container, ephemeral VM) is a different thing — apply for real
against a throwaway environment. Not currently supported. Real
effort: harnessing the existing apply path against a pluggable
target host, plus the orchestration to spin up + tear down the
sandbox.

**Verdict:** 80% of the operator value is already in `plan` /
`inspect`. The sandbox-VM version is a big project with narrow
incremental value (most state machines aren't reproducible enough
in a throwaway VM to be informative anyway). **Defer indefinitely**
unless a concrete consumer (CI integration?) shows up.

### §9. Agent negotiation MCP layer  🟡 partial

**Issue asks:** MCP tools `mooncake.plan`, `mooncake.diff`,
`mooncake.risk`, `mooncake.permissions`, `mooncake.apply_approved`,
`mooncake.rollback`. The agent proposes, Mooncake decides, human
approves.

**Today:**

- spec-10 (✅ shipped) `mooncake mcp` server exposes `run_step`,
  `get_facts`, `get_snapshot`, `check_plan`, `run_plan`.
- spec-22 **phase 7 not yet started** is exactly "MCP server
  exposes Diff/Cost/Permissions in plan tool output."

**Gap:** the per-step Diff/Cost/Permissions surface is shipped in
the JSON plan output but not yet wired through the MCP `plan` tool
return shape. `apply_approved` (two-phase apply: agent gets a plan
hash, human signs, agent submits hash to apply) and `rollback`
(driving spec-30 transaction rollback via MCP) aren't there.

**Verdict:** spec-22 phase 7 already names half of it. Closing
the loop is small in mechanism but **strategic in framing** — the
"LLM proposes, Mooncake decides, human approves" tightening is the
clearest agent-runtime pitch in the repo. Worth its own follow-on
spec once phase 7 lands.

### §10. Autonomous maintenance / invariants  📝 drafted

**Issue asks:** declare invariants (`service: nginx running`,
`disk free > 10GB`, `endpoint 200`); on violation,
`observe → repair plan → simulate → execute → record`.

**Today:**

- **[spec-58 fleet-drift](../specs/personal-fleet/spec-58-fleet-drift.md)**
  (📝 drafted) — periodic `InspectPlan` loop on agentd,
  `/v1/drift` endpoint, `mooncake fleet drift` renderer, per-machine
  `drift: { policy: notify | reapply | revert | none }`. Three-PR
  rollout with the autonomous policies behind PR C.
- Same idea is also brainstormed in
  [`clustermanagement/agentic-interface-brainstorm.md` §3](../clustermanagement/agentic-interface-brainstorm.md)
  and called out in
  [`issue-11-analysis.md` §10](../clustermanagement/issue-11-analysis.md)
  as the single highest-strategic-value item from issue #11.

**Verdict:** the closest 1:1 match in the repo. Issue #8's
"invariants" framing is just a friendlier YAML shape on top of the
plan-conformance loop that spec-58 designs. **No new design pass
needed; ship spec-58 and revisit naming/UX if invariants make for a
clearer surface than `drift:`.**

---

## What's actually new in issue #8

After stripping out the already-shipped and already-drafted items,
the genuine new mechanisms are:

| Item                                   | Effort     | Strategic value | Prerequisite                              |
|----------------------------------------|------------|-----------------|-------------------------------------------|
| §7 `observe.*` family                  | M (per-handler, ~100–200 LOC each) | **highest** — closes the agent loop | nothing — independently implementable |
| §4 `mooncake explain <resource>`       | M (~200–300 LOC indexer + renderer) | high — best operator UX win | spec-22 phase 4 (Diff) ✅ |
| §5 `mooncake discover` → YAML emitter  | L (per-action emit-YAML shape × 13+) | high (adoption funnel) | nothing — sits on facts + tool inventory |
| §3 `mooncake rewind --to <ts>` (cross-run) | L | medium (demo-friendly, brittle) | finish spec-22 phase 5 refusals + persist `ReverseData` |
| Graph-edges on plan output (§1)        | S (~150 LOC) | low until consumer shows up | nothing |

Everything else — typed-ABI primitives, transactions, run-history,
risk scoring, plan-time inspection, MCP server, invariant loop —
is either shipped or actively drafted.

---

## The author's own "first slice" — what's already covered

The issue ends with a proposed minimal first slice:

| Issue's first slice                                          | Status in repo                                                       |
|--------------------------------------------------------------|----------------------------------------------------------------------|
| ChangeGraph v0 (wrap planned steps)                          | ✅ Plans already typed; `--format graph` is a small renderer follow-up |
| `observe.*` minimal family                                   | ❌ Not started — pick 2–3 (`observe.process`, `observe.port`, `observe.http`) |
| `risk.score`                                                  | ✅ Shipped per-step (Cost.Risk); ❌ no dedicated CLI scorecard         |
| `explain <resource>`                                         | ❌ Not started                                                       |
| MCP plan / diff / risk / apply tools                         | 🟡 spec-10 shipped; spec-22 phase 7 not done; `apply_approved` shape new |
| One demo (AI proposes unsafe change, Mooncake rejects/rolls back) | 🟡 The pieces (spec-22 Permissions, spec-30 transactions) exist; demo plan doesn't |

Trimmed to what's **actually new work**, the first slice collapses
to: a 2–3 handler `observe.*` seed + `mooncake explain` + spec-22
phase 7 MCP wiring + one demo plan. Two-to-three weeks of focused
work, not a re-architecture.

---

## Recommended order

Given streams.md's current recommendation (37 → 38 → 22 phase 7 →
…) and what's specced under personal-fleet, the cleanest spec
sequence to absorb issue #8 is:

1. **Finish what's already drafted that issue #8 reinforces:**
   spec-37 + spec-38 (read-side observation gap; prereq for
   `observe.*` to compose well), spec-58 (invariant loop), spec-22
   phase 7 (MCP wiring of Diff/Cost/Permissions).
2. **New: `observe.*` family spec** under
   `docs-working/specs/action-surface/`. Pick 3–4 handlers as the
   seed (process / port / http / gpu) and a shared `ObserveResult`
   shape. **This is the load-bearing new bet.**
3. **New: `mooncake explain <resource>` spec** under
   `docs-working/specs/developer-experience/`. Builds on shipped
   run-history + Diff payloads; index keyed by `(host, resource)`.
4. **New: `mooncake risk plan.yml` CLI renderer** — tiny, lands
   alongside (2) or (3) as a quality-of-life PR. Not its own spec.
5. **New: `mooncake plan --format graph` JSON emitter** — same
   shape, same triviality. Lands when an MCP/UI consumer needs it.
6. **Stretch: `mooncake discover`** — its own spec when adoption
   is the focus. Sits on facts + tool inventory; per-action
   "emit YAML" shape per handler.
7. **Stretch: `mooncake rewind --to <ts>`** — gated on finishing
   spec-22 phase 5's two refusal cases (`os.service`,
   `file.unarchive`) and persisting `Result.ReverseData` beyond
   transaction lifetime.

Items in the issue *not* on this list — graph-edges as first-class
plan metadata, sandbox-VM rehearsal, ChangeGraph-as-event-source
formal model — are deferred until a consumer needs them. The
vocabulary upgrade ("ChangeGraph", "intent → compile → graph") is
useful for positioning docs (VISION.md, README, agent-runtime
pitch) regardless of whether the engineering reorganizes around it.

---

## Where this fits the existing vision

The 5-stream model in
[`streams.md`](../streams.md) already absorbs every concrete piece
of issue #8 without reshaping:

- **Stream 1 (Action Surface)** — `observe.*` family is the
  natural mirror of the typed-mutation ABI. spec-22 phase 7 + a
  new `observe.*` spec close the action-surface story.
- **Stream 2 (Safe Agent Runtime)** — agent negotiation MCP
  (§9), risk scorecard (§6), policy gates (already in streams.md
  as unwritten future work) all reinforce this wedge. Issue #8's
  framing — *LLM proposes, Mooncake decides, human approves* — is
  the tightest single-sentence statement of this stream's pitch
  yet. Adopt the wording.
- **Stream 3 (Fleet & Cluster)** — invariant loop (§10) maps
  directly to spec-58. `mooncake explain` (§4) wants the
  fleet-side `fleet why` shape from the agentic brainstorm.
- **Stream 4 (Developer Experience)** — `mooncake discover` (§5)
  is the adoption funnel for solo devs; sits naturally next to
  `mooncake init`, `mooncake doctor`, the personal-fleet onboarding
  flow.
- **Stream 5 (Ecosystem)** — no direct impact.

The issue does **not** require new streams, new epics, or a kernel
rewrite. It does usefully sharpen the framing of streams 1 + 2 and
gives concrete UX targets (`observe`, `explain`, `risk`, `discover`,
`rewind`) for command surfaces that the existing roadmap had only
sketched.

---

## Cross-references

- [spec-22 extended-handler-abi](../specs/action-surface/spec-22-extended-handler-abi.md)
  — the typed-ABI spec that ships most of the per-step
  ChangeGraph claims (Diff / Reverse / Cost / Permissions).
  Phases 1–6 shipped; phase 7 (MCP wiring) and phase 8 (docs)
  outstanding.
- [spec-30 transactions](../specs/done/spec-30-transactions.md)
  — LIFO reverse-on-failure; issue #8's "rollback" primitive.
- [spec-58 fleet-drift](../specs/personal-fleet/spec-58-fleet-drift.md)
  — issue #8's "invariants / autonomous maintenance" by another
  name; drafted.
- [spec-37 step-output-capture](../specs/action-surface/spec-37-step-output-capture.md)
  + [spec-38 read-json-yaml](../specs/action-surface/spec-38-read-json-yaml.md)
  — the read-side primitives that any `observe.*` family wants to
  compose with. Drafted; recommended next-up in `streams.md`.
- [`clustermanagement/agentic-interface-brainstorm.md`](../clustermanagement/agentic-interface-brainstorm.md)
  — §1 (typed observability), §3 (reconciliation loops), §4
  (`fleet why` / replay), §5 (capability-scoped trust + cost
  budgets), §6 (federated MCP). The strategic-bet half of issue #8,
  already written down.
- [`clustermanagement/issue-11-analysis.md`](../clustermanagement/issue-11-analysis.md)
  — companion analysis of issue #11. Many overlap points
  (observability, explain, drift, secrets / policy gates as
  capability-scoped trust).
- [`streams.md`](../streams.md) — the absorbing structure.
  Issue #8's items map cleanly into streams 1–4 without
  reorganization.
- [`epics/epic-agent-efficiency.md`](../epics/epic-agent-efficiency.md)
  — the already-realized "system sense organ for AI agents"
  framing. Issue #8 extends the same vision; doesn't replace it.
