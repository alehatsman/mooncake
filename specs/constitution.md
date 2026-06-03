---
id: constitution
status: living
owners: [aleh]
---
# Constitution

## Intent

This is the repo-wide contract every other spec inherits. Mooncake is a typed,
idempotent, audited execution layer between an actor (human, script, or AI) and
a system — a system-call ABI for *intent → physical change*, not a config
manager that happens to take YAML. Every mutation flows through one constrained,
observable funnel so that auditability, idempotency, policy, reversibility, and
agent safety are properties of the engine rather than obligations on the actor.
These principles are the *why* behind the per-subsystem specs; where they bear
on a feature, that spec applies them rather than restating them. They change
rarely and deliberately; a sibling spec that needs to contradict one should say
so explicitly and link here. Keeping them in one place is what lets the rest of
the specs stay short.

## Behavior

- WHILE mooncake runs, it is a single static Go binary: the CLI, MCP server,
  agent loop, planner, executor, facts, and fleet peer all ship in one artifact
  with no runtime dependency to install — a fresh box gets the whole kernel by
  dropping one file.
- WHERE an actor mutates a system, the only sanctioned path is the typed action
  funnel: every change is a typed action with declared inputs, validated against
  a schema before it runs, so an actor (especially an AI agent) cannot reach the
  host except through the constrained ABI.
- WHILE a plan is applied, idempotency is the engine's guarantee, not the
  actor's discipline: a handler observes current state and converges to desired
  state, so re-running a plan that already matches is a no-op and the actor never
  hand-writes "check before change" logic.
- WHEN any step runs, it emits a typed, structured event, so the run log and
  audit trail are a byproduct of execution — observability comes for free rather
  than from instrumentation the actor must remember to add.
- WHERE a change has not been confirmed, dry-run precedes apply: `--dry-run`
  plans and renders a per-action typed Diff with no side effects, so the actor
  (or a reviewer, or an agent's proposer) sees exactly what would change before
  anything does.
- WHERE a grouped change can fail partway, safety is structural: `transaction:`
  blocks reverse already-applied steps in LIFO order on failure, so a half-
  applied mutation rolls back to the prior state rather than leaving the system
  wedged.
- WHERE a secret is referenced, it never materializes where it can leak: typed
  `!secret` refs resolve only at apply time and resolved values are redacted out
  of plans, diffs, events, and logs (see secrets-and-security).
- WHERE the host gives up its permission wall by moving execution into the
  kernel, a per-run policy replaces it: the actor spawning a run declares what it
  may do (action allow/deny, network, risk cap) and the executor refuses any step
  that exceeds it — this is what makes a shell-less agent run *safe*, not merely
  structured.
- WHILE the system grows, scope stays inside three rings — (1) kernel:
  declarative actions, idempotency, planning, facts; (2) runtime: host daemon,
  fleet, audit, policy; (3) economy: preset marketplace, agent SDK, signed plans
  — each ring built on the one below and independently usable, so a solo dev
  never pays for the fleet and an enterprise reuses the same kernel.
- WHILE mooncake is developed, simplicity wins: boring, explicit mechanisms are
  preferred over abstraction and magic (a flat policy struct over a policy DSL,
  one escalation primitive over six hand-rolled sudo wraps), and scope is added
  only when a real need demands it.
- WHERE specs exist, they are the dual of the code: a spec says what the system
  *should* do and the gap to what it *is* is drift — a signal surfaced to humans,
  recorded as an unchecked checklist item, not a merge blocker.

## Non-goals

- **A config-management framework.** Mooncake is an execution ABI, not "Ansible
  for AI"; compatibility with another tool's module ecosystem is not a goal.
- **An expressive policy DSL.** Gating is a flat allow/deny/risk contract; an
  OPA/Rego-style policy language is deliberately out of scope (the sprawl trap).
- **A central scheduler or hub-of-record for the fleet.** The personal fleet is
  peer-to-peer; a mandatory control plane is a ring-3 product, not a kernel
  assumption.
- **Untyped escape hatches as the happy path.** Arbitrary shell exists but is the
  thing policy exists to *remove*; the typed action surface is the intended
  interface.
- **Restating subsystem detail.** This spec holds principles only; the concrete
  behavior of each capability lives in its own spec.

## Checklist

- [x] Single static Go binary carries the whole kernel + CLI + MCP + agent + fleet peer
- [x] Typed action funnel: validated typed actions are the only sanctioned mutation path
- [x] Idempotency guaranteed by the engine (observe → converge), not the actor
- [x] Every step emits a typed structured event; audit/observability for free
- [x] Dry-run precedes apply; per-action typed Diff with no side effects
- [x] `transaction:` blocks auto-revert in LIFO order on failure
- [x] Secrets resolve at apply time only and never leak into plans/diffs/events/logs
- [x] Per-run policy gate replaces the host permission wall for in-kernel execution
- [x] Three-ring scope (kernel / runtime / economy), each independently usable
- [x] Simplicity over abstraction; explicit mechanisms over DSLs and magic
- [x] Specs are the dual of code; drift is a non-blocking signal
