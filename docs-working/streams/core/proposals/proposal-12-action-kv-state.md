# Proposal 12: `kv` action — typed persistent state that participates in plan/diff

**Status:** Draft proposal (brainstorm-stage)
**Effort:** S (~3 days)
**Value:** High — agent loops and self-healing plans need memory
between runs; today they smuggle it through files and lose the
kernel's diff/perms/risk accounting.

---

## Problem

Mooncake has facts (read-only snapshots of system state) and
templates (read-only interpolation of facts and vars). It does
not have a **mutable, typed, plan-managed store of state**.

When an agent or a multi-run plan needs to remember something —
"last successful build SHA", "the bearer token we got from auth",
"which migrations have run", "the timestamp of last heal" — the
options today are:

- Write to a file with `file:` (untyped; the planner sees text, not
  meaning).
- Write to facts (read-only by design — wrong abstraction).
- Smuggle through environment vars between steps (lost across runs).
- Use a sidecar database (loses the kernel's audit trail).

For the directions this project is growing — pilot agents with local
models, self-healing maintain loops, fleet coordination without a
control plane — the agent's *memory* should be a first-class kernel
concept the same way file state is.

## Proposal

A new core action: `kv`. Backed by a typed JSON store at
`~/.mooncake/kv/<namespace>.json`, with schemas declared per
namespace.

### Schema declaration

```yaml
# preset.yml
kv_schemas:
  build:
    last_sha: string
    last_built_at: timestamp
    attempts: integer
```

### Steps

```yaml
- kv:
    namespace: build
    set:
      last_sha: "{{ vars.git_sha }}"
      last_built_at: "{{ now() }}"

- kv:
    namespace: build
    get: last_sha
    as: previous_sha    # exposes `vars.previous_sha` downstream

- kv:
    namespace: build
    delete: last_sha
```

### Diff semantics

The planner shows kv changes the way it shows file changes:

```
kv build/last_sha: "abc123" → "def456"
kv build/last_built_at: (unset) → "2026-05-17T12:00:00Z"
```

This is the whole point — agent state changes are auditable the
same way file state changes are. `mooncake history show <id>`
includes kv deltas. `mooncake plan --diff` previews them before
they happen.

### Why typed, not freeform

The temptation is "just give me a freeform map". The argument
against:

- Untyped maps drift into garbage over time. The planner can't tell
  a typo from a new key; the user can't tell what keys exist.
- Typed schemas let `mooncake doctor` flag stale or
  schema-mismatched entries.
- Typed entries let the diff show "string → string" cleanly, vs.
  guessing how to render an arbitrary blob.

The schema is declared once per namespace in the preset / plan file
and lives next to the steps that use it.

### Concurrency

The kv store uses an exclusive lock per namespace during apply.
Cross-host coordination (LAN fleet) is out of scope here —
proposal future-work, or a separate `fleet-kv` proposal layered on
top.

## Use modes

- **Agent loop memory** — pilot agent stores its rolling context,
  last decision, last action's effect.
- **Self-healing book-keeping** — assert+heal (proposal-11) records
  how many times it healed in the last hour; quota check can read
  `kv.heal_count_last_hour`.
- **Incremental work tracking** — "which files have I already
  processed?" Without losing it to a kernel restart.
- **Auth token caching** — the token from a `mcp_tool` auth flow
  (proposal-07) cached with a TTL, refreshed by an `assert` that
  checks token expiry and heals by re-auth.

## What this doesn't address

- **Distributed kv across LAN peers** — out of scope; defer to a
  fleet-kv proposal that builds on this.
- **TTL / expiry** — useful, but punt to v1.1 unless agent-stream
  needs surface it sooner. A `ttl:` field can be added without
  breaking v1.
- **Schemas with nested objects** — first cut is flat. JSON-like
  nesting is a slippery slope to "we shipped a database".

## Field-budget impact

Zero universal fields. `kv:` is a new step type, fully self-contained
in its block. The `kv_schemas:` block in preset.yml is a new
top-level key, not a field on `Step`.

## Pairs with

- **Core proposal-11** (`assert` + `heal`) — heal counters, last-heal
  timestamp, rate-limit windows all want kv backing.
- **Agent proposal-07** (`mcp_tool`) — cache_key entries can be kv
  entries; one mechanism for "did we already do this?".
- **Core proposal-15** (`plan` recurse) — sub-plans need a shared
  state surface; kv is the obvious answer.
- **Fleet stream** — future "fleet-kv" for cross-node state lives
  on top of this single-node design.
