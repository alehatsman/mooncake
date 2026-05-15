# Cluster Management — Agentic Interface Brainstorm

If mooncake is **the typed interface an LLM uses to drive 1–10
machines reliably**, what does that surface look like at full bloom?

Less a roadmap, more a creative pass on the long arcs and ambitious
bets. Some of these are incremental; some are speculative. None are
specced yet. Captured 2026-05-15 from a single brainstorming pass.

---

## 1. The agent's senses — typed *observability*

Mooncake today has a typed *mutation* surface — `Permissions` /
`Diff` / `Reverse` / `Cost`. The mirror image — typed *observation* —
is mostly missing. Every action that can be applied should have a
`Sense()` half:

- **`observe.*`** — read state without mutating. `observe.file`,
  `observe.process`, `observe.port`, `observe.service`. Returns typed
  results, not free-form text the LLM has to parse.
- **`probe.*`** — structured tests. `probe.reachable`,
  `probe.listening`, `probe.disk_free > 10GB`. Pass/fail with
  reasoned details.
- **`measure.*`** — time-series collection. Latency, error rate,
  queue depth. Built on the existing `/v1/metrics` but fleet-wide and
  queryable historically.

This unlocks the agent loop *"check state → decide → mutate →
verify"* as a single typed conversation instead of "ssh in, parse
logs, guess."

**Strategic argument**: this is what the README's typed-ABI claim is
*actually* missing.

## 2. The fleet as one queryable graph

Today `fleet facts` fans out per-peer. The ambitious version: a
**federated inventory graph** the agent queries like a tiny knowledge
base.

- *"which peers have nvim installed and >16 GB RAM?"*
- *"diff installed packages between main_pc and laptop"*
- *"show me services running on Windows hosts but not WSL"*

Each agentd already collects facts. What's missing is the
controller-side cache + typed query surface. MCP tool
`fleet.query(<dsl>)` returns typed rows.

## 3. Self-healing reconciliation loops

Today mooncake is **pull-by-human**: you run `apply`, it applies,
done. The Kubernetes-flavored version: declare invariants, agentd
inspects periodically (spec-16 already does this on-demand), emits
drift events, optionally auto-reapplies.

```yaml
invariants:
  - name: nvim installed
    plan: machines/main_pc/dev.yml
    inspect_interval: 1h
    on_drift: notify   # or: apply, with cost-budget gate
```

This is the single feature that turns "config management tool" into
"fleet operating system." Pairs naturally with `fleet schedule`.
Stream-2 adjacent; needs spec-30's transaction infra under it.

## 4. Conversation as the unit of work

An agent doesn't *make a plan*; it has a *conversation* across many
small typed actions. Mooncake should treat that as a first-class
concept.

- **Conversation-scoped state** — vars/facts captured during a
  multi-step exchange persist for the convo, get garbage-collected
  after.
- **Conversation transcripts as runbooks** — when the agent fixes
  something, the typed trace becomes a saveable preset. *"You did
  this last time main_pc broke; want to re-run it?"*
- **`fleet why`** — git-blame for system state. *"Why does
  /etc/wsl.conf look like this?"* → traces back through run history
  with attributions.
- **Replay** — re-run a past plan against the captured facts of that
  moment. The deterministic-replay pillar from VISION §13.10.

## 5. Capability-scoped trust

The current bearer token is all-or-nothing. Real agentic ops need
finer-grained:

- **Scoped tokens**: *"this agent can read state but only apply with
  --dry-run"*, *"can apply but not touch anything tagged
  destructive"*.
- **Cost/risk budgets**: `Cost()` lets the agent and the operator
  agree on caps. *"Don't spend more than $0.20 of cloud-time this
  hour"*, *"no plans with >5 file deletes"*.
- **Approval gates between transaction phases** — for high-cost
  phases, post a structured "about to do X, here's the diff" to
  slack/web/CLI; block on ack.
- **Plan signing + provenance** — every applied plan has signed
  by-who. Agent runs require signed-by-{agent,human}. Closes the
  audit-trail story.

## 6. Federated MCP — the agent interface

The killer integration is **MCP federation**. Each agentd already has
an MCP server inside. Controller-side MCP aggregates them, so an
agent connects to *one* MCP endpoint and can:

- target individual peers (`fleet.apply --peer main_pc <plan>`)
- query state across the fleet (`fleet.query`)
- subscribe to live events (`fleet.watch`)
- compose multi-peer transactions (the ordered-phase apply, exposed
  as a tool)

This is the answer to "what *is* mooncake-the-agent-interface."
Right now you have it per-peer; making it *fleet-shaped* is the
unlock.

## 7. Coordination primitives — fleet as a distributed system

If two agents (or a human and an agent) drive simultaneously, you
need:

- **Fleet-wide leases** — *"I'm applying to main_pc, hands off"*.
  Lightweight mutex via agentd state.
- **Quorum gates** — for risky ops *"don't reboot all 3 web servers
  at once"*. Built-in N-of-M ack.
- **Time-windowed maintenance** — *"apply at the next 1am–3am
  window"*. Plan held by agentd until the window opens.
- **Cost-aware scheduling** — `Cost()`-tagged steps deferred to
  off-peak.

## 8. Per-peer micro-operators

The speculative one. Each agentd hosts a tiny local LLM "operator"
that takes high-level intents in plain English and produces typed
mooncake plans *for its peer specifically* — leveraging local facts
the controller would need a round-trip to learn. The controller's
agent becomes a coordinator-of-operators rather than a
driver-of-actions.

Why this is interesting: a Windows operator knows about registry
quirks, a macOS operator knows about launchd vs systemd.
Specialization at the edge.

Risk: very speculative; would need a much smaller bundled model.
Probably wait for sub-1B local-capable models to mature.

## 9. The fleet REPL

`mooncake repl` — typed prompt with autocomplete that talks to the
fleet. Live `fleet.query` results scroll. The human-facing equivalent
of the agent's MCP surface. Probably the single most demo-able
feature on this list.

## 10. The audit / replay story as a product

Every run is a typed event stream. Stream it off-box to an audit
sink → search, replay, attribute. *"What did the agent do last week?
Show me the 3 highest-cost actions."* Especially load-bearing if
mooncake ever ships into regulated work.

---

## Where to plant the flag

If forced to pick *one* of these as the next strategic bet:

### **Typed observability primitives** (item 1)

- It's the missing half of the ABI — mutation is solved, observation
  isn't.
- It unblocks almost every other ambitious idea on this list
  (reconciliation, self-healing, conversation-scoped state,
  fleet-graph queries, replay).
- It's the answer to "why pick mooncake over ansible + prometheus +
  ssh." The typed-mutation story is *necessary* but not *sufficient*;
  the typed-observation story is what makes the agent loop close.
- It's incremental — each `observe.X` action ships independently
  like the action surface did.

### Honourable mention: **Federated MCP** (item 6)

It's the marketing surface — *"connect Claude/Cursor/whatever to
mooncake, get a typed fleet of personal computers."* Visceral demo,
less foundational than typed observability but the more immediate
adoption hook.

---

## Cross-references

- `epics/done/epic-personal-fleet.md` — Stream-3 framing this builds on.
- `streams.md` — the unwritten future-spec list mentions policy DSL,
  plan signing, per-action quotas, egress policy, sandbox mode, cost
  classifier, deterministic replay; several map onto items above.
- `PROGRESS.md` (rev12) — current state. The "strategic constraint
  has shifted from code to users" line is the context for why a
  brainstorm like this is timely.
- `requests/request-apply-machine-multi-peer.md` — the
  most-recently-closed request, useful as a template for what a
  "real user-driven spec" looks like; several items above could be
  filed as requests in a similar shape.
