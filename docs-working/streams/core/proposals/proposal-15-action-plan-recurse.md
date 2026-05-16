# Proposal 15: `plan` action — recursive sub-plan execution as a first-class step

**Status:** Draft proposal (brainstorm-stage)
**Effort:** M (~1 week — planner + executor need recursion limits
and result aggregation)
**Value:** Medium-High — composition is the unlock. Today plans
include other plans only at file-include time (static). A `plan:`
step makes "run this named plan, possibly with different vars,
possibly conditionally, possibly capture its result" a kernel
primitive.

---

## Problem

Mooncake plans compose by **textual inclusion**: presets include
other presets, plans include presets, all resolved at parse time
into one flat list of steps. That's the right default — it keeps
plans readable and the diff understandable.

What it doesn't give you is **runtime composition**:

- Run a named sub-plan as a single conceptual step, with the
  sub-plan's recap appearing as one row in the parent recap.
- Run a sub-plan conditionally based on a runtime value.
- Run a sub-plan in response to an event (proposal-14 `watch`).
- Run a sub-plan as a heal handler (proposal-11 `assert`+`heal`).
- Run a sub-plan with a different set of permissions / risk profile
  than the parent.
- Run the same sub-plan with a `for_each` over a runtime list.

Today those patterns either don't compose (you can't `watch ... do:
include`), or they flatten into the parent (losing the conceptual
boundary), or they require shelling out (`shell: mooncake apply
sub.yml` — loses kernel accounting).

For the kernel to grow into agent systems where small declarative
flows are composed dynamically into larger ones, composition has
to be a kernel verb, not a parser feature.

## Proposal

A new core action: `plan`. Step shape:

```yaml
- plan:
    file: ./presets/install-llm.yml
    vars:
      model: llama3.1
      gpu: cuda
    when: "{{ fact.gpu.vendor == 'nvidia' }}"
    permissions:
      allow: [pkg.install, file.write, process.start]
    capture:
      as: install_result
```

Or by reference to a labeled plan in the same file:

```yaml
plans:
  install_llm:
    - pkg: { name: ollama }
    - process: { name: ollama, command: [ollama, serve], state: running }

steps:
  - plan:
      name: install_llm
      vars: { model: llama3.1 }
```

### Semantics

- **Single-step accounting** — in the parent recap, the `plan` step
  shows as one row (`changed=1` if the sub-plan changed anything,
  `ok=1` if no-op, `failed=1` if anything failed). The sub-plan's
  detailed recap is nested in `mooncake history show`.
- **Diff** — the parent's `plan --diff` recursively previews the
  sub-plan, indented one level. Big plans collapse cleanly with
  `--diff-depth N`.
- **Permissions** — the parent declares what the sub-plan is allowed
  to do. The sub-plan's actual `permissions:` block must be a subset.
  If it asks for more, the parent rejects (this is the proposal-06
  contract pushed down one level).
- **Vars** — the sub-plan gets a fresh vars scope seeded by the
  `vars:` block. No transparent inheritance — you have to pass
  what you want. Less spooky-action, easier to reason about.
- **Result capture** — `capture: { as: <name> }` exposes a typed
  summary (counts + handler results) as a var for downstream steps.

### Recursion limits

- Hard cap on depth (default 8, configurable per apply). Recursion
  past the cap is a plan-time error.
- Cycle detection — if `a.yml` plan-includes `b.yml` which
  plan-includes `a.yml`, the planner rejects before execution.

### Run modes

- **Inline (default)** — sub-plan runs in the same process,
  in-order with the parent's steps.
- **Async** — `mode: async` returns immediately; the sub-plan runs
  in the background under the same `process:` registry
  (proposal-13). Useful for fire-and-forget reactions.
- **Isolated** — `mode: isolated` runs the sub-plan in a separate
  process with its own working directory and env. Heavier; used for
  permission boundaries you don't want shared.

## Why this is a kernel primitive, not a CLI feature

The composition opportunity is in every other proposal in this
batch:

- `assert` + `heal` (proposal-11) wants `heal:` to be a plan, not
  a single step.
- `watch` (proposal-14) wants `on_event:` to be a plan.
- Fleet stream wants "run this plan on N peers" (the peer driver
  calls into `plan:` semantics, just remote).
- Agent loops want "run this branch of the decision tree as a
  sub-plan with its own permissions".

Each of those is awkward without first-class plan composition. With
it, they're all the same shape: hand a plan reference, get a result.

## Use modes

- **Reusable heals** — one named sub-plan (`restart_api`) is the
  heal for three different asserts.
- **Conditional flows** — `when:` on a `plan:` step branches the
  apply without spreading conditionals across every inner step.
- **for_each composition** — iterate a runtime list, run a sub-plan
  per element.
- **Permission gating** — call a sub-plan with `permissions: { allow:
  [readonly.*] }` to enforce a read-only branch.

## What this doesn't address

- **Static includes** — presets and `include:` continue to work as
  textual composition. `plan:` is the runtime variant; both have
  legitimate uses.
- **Cross-host sub-plans** — that's the fleet stream's "run this
  plan on peer N". `plan:` is the local primitive that fleet wraps.
- **Plan composition as a graph (not a tree)** — out of scope; if
  two parents need to share a sub-plan's *result*, capture-and-pass
  is the seam.

## Field-budget impact

Zero universal fields. `plan:` is a new step type, fully
self-contained. Recursion depth is a runtime config knob, not a
schema field.

## Pairs with

- **Core proposal-11** (`assert` + `heal`) — `heal:` becomes a
  `plan` reference.
- **Core proposal-14** (`watch`) — `on_event:` becomes a `plan`
  reference.
- **Core proposal-12** (`kv`) — sub-plans share state via kv.
- **Agent proposal-06** (permissions as contract) — recursive
  permission narrowing is the headline use of this proposal.
- **Fleet stream** — peer execution becomes "plan: but remote".
