# Proposal 14: `watch` action — reactive triggers for the maintain loop

**Status:** Draft proposal (brainstorm-stage)
**Effort:** M (~1 week — needs an event-source abstraction in the
executor)
**Value:** Medium-High — pairs with `assert`+`heal` (proposal-11)
to make maintain-mode event-driven instead of poll-only. Pilot
agents and self-healing systems care about latency more than
periodic sweeps.

---

## Problem

`wait.*` actions today block a step until a condition becomes true,
then proceed. That's the **synchronous wait** primitive.

The **asynchronous react** primitive — "when X happens, run Y" — is
missing. Today users compose it from cron + `mooncake apply`, which
means:

- High latency (cron granularity, typically a minute floor).
- High wasted work (re-evaluate every step every tick to find one
  that needs a change).
- No event provenance (the heal doesn't know *why* it was triggered).

For event-driven systems the kernel naturally extends into —

- a file watcher that re-runs a config sync when something edits a
  source file;
- an MCP resource subscription that wakes the maintain loop when a
  remote resource changes;
- a fact-change trigger ("when fact `cpu.load_1m > 5`, run heal X");
- a process-exit trigger ("when `ollama` dies, restart it");
- a periodic timer ("every 30s, evaluate this assert") —

the missing primitive is the same: **declare an event source, attach
a handler step**.

## Proposal

A new core action: `watch`. Two roles depending on usage:

### As a step inside an `apply` (start watching)

```yaml
- watch:
    name: config_changed
    source:
      file_changed: ~/.config/myapp/config.yaml
    on_event:
      - plan: ./apply-config.yml      # see proposal-15 (plan recurse)
    scope: plan                       # plan | session | persistent
```

This starts a watcher and lets the plan continue. The watcher lives
for the configured `scope`:

- **plan** — dies when this apply finishes.
- **session** — survives until a future plan stops it.
- **persistent** — registered with the user's `agentd` (fleet
  stream) for cross-restart persistence.

### As the driver of `mooncake maintain` (run loop)

```bash
mooncake maintain plan.yml
```

Reads all `watch:` steps from the plan, wires them up, and blocks.
Each event fires the attached handler. Stop with SIGINT; the watchers
unregister cleanly.

### Event source vocabulary

A small typed grammar — same shape as `assert.check:` predicates
(proposal-11), but in the reactive direction:

- `file_changed: <path>` — inotify / fsevents / ReadDirectoryChangesW.
- `interval: <duration>` — timer (the cron replacement).
- `fact_changed: <fact_query>` — re-evaluate a fact; fire on diff.
- `process_exit: <process_name>` — pid from proposal-13's process
  registry.
- `mcp_resource: <server>/<uri>` — MCP resource subscription, when
  the server supports it (proposal-07 `mcp_tool` registry knows).
- `port_state: <port>` — listening / not listening transition.
- `http_status: <url>` — long-poll a health endpoint.

Each source maps to one underlying event mechanism; new sources are
additive over time without changing the action shape.

### Handler contract

`on_event:` is a list of steps. They run in their own apply context,
with `vars.event` bound to a typed description of what fired
(source kind, identifying detail, before/after value for changes).

The handler is just a plan. Failures land in history. The watcher
keeps running unless the handler itself calls `watch.stop`.

## Why this is a kernel primitive

The composition is the value:

- `watch` + `assert`+`heal` = event-driven self-healing.
- `watch` + `kv` (proposal-12) = stateful reaction (record the last
  event, debounce, batch).
- `watch` + `mcp_tool` (proposal-07) = "when MCP resource changes,
  call an MCP tool" — agent reflexes.
- `watch` + `plan` recurse (proposal-15) = "when X happens, run this
  named plan" — composable triggers.

Without `watch`, every one of these patterns is built outside
mooncake with cron + glue, losing the kernel's diff/perms/risk
accounting.

## Use modes

- **Dotfile sync** — watch source files, re-apply on edit.
- **Pilot agents** — watch facts (cpu, memory, network), have the
  agent decide what to do.
- **Fleet self-healing** — each LAN node watches its own services,
  heals locally, no control plane required.
- **MCP-driven agents** — watch remote resources, react with mooncake
  steps that have full diff/perms/risk.

## What this doesn't address

- **Cross-node event aggregation** — fleet stream territory. Each
  node watches its own.
- **Backpressure on event storms** — needs `debounce:` and
  `coalesce:` fields. Sketch them in; ship minimal first.
- **Persistent reliable delivery** — events that fire while the
  watcher is down are lost. "Exactly-once" is not a goal at this
  layer; persistent reliable streams are someone else's problem.

## Field-budget impact

Zero universal fields. `watch:` is a new step type, fully
self-contained. The handler reuses existing step shapes.

## Pairs with

- **Core proposal-11** (`assert` + `heal`) — `watch` is the event
  driver; `assert`+`heal` is the typed reaction.
- **Core proposal-12** (`kv`) — watchers want state (last event,
  debounce window, event count).
- **Core proposal-13** (`process`) — `watch` can react to process
  exits; the watcher itself runs under the executor's lifecycle.
- **Core proposal-15** (`plan` recurse) — `on_event` handlers are
  often "run this named plan".
- **Agent proposal-07** (`mcp_tool`) — MCP resource subscriptions
  are first-class `watch` sources.
