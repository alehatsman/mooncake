# Spec 64: `fleet observe` — Cross-peer observation fan-out

**Status:** Draft (depends on spec-59)
**Epic:** Personal Fleet (Phase D — fleet QoL bundle)
**Effort:** S (~200–300 LOC; thin controller-side fan-out)
**Value:** High once spec-59 lands. The personal-fleet runtime is
13/14 PRs but every existing fleet command is *mutation-oriented*
(`fleet apply`, `fleet upgrade`, `fleet bootstrap`) or *log-oriented*
(`fleet logs`, `fleet facts`). The read-side cross-peer story is missing:
*"is port 8080 open on any peer? which peers have GPU memory free?"*

**Design principles:** [`action-design-principles.md`](../../action-design-principles.md) + [`non-goals.md`](../../non-goals.md)

---

## Problem

[`spec-59`](../action-surface/spec-59-typed-observability.md) gives
each peer a typed `observe.*` family. The controller-side fan-out
that turns those into fleet-wide queries is missing:

```bash
# Single-peer (works after spec-59):
mooncake observe.port --host localhost --port 80

# Cross-peer (this spec):
mooncake fleet observe.port :80
mooncake fleet observe.gpu --selector 'role=inference'
```

The shape is the same multiplex-over-peers pattern `fleet apply` /
`fleet logs --all` / `fleet facts --query` already use. This spec
adds the read-side equivalent.

---

## Goals

- **G1** `mooncake fleet observe.<kind> [args]` — fan out a single
  observation across all peers (or a `--peer-filter` subset).
- **G2** Output is a tabular comparison by default, JSON via
  `--format json`. Same shape as `fleet facts --query` so users
  build muscle memory on one surface.
- **G3** Respect spec-50 filters (`tag=`, `name=`, `os=`, `role=`)
  and spec-48 overlays.
- **G4** Synthesize a tiny one-step plan client-side, submit through
  the existing `submit_run` agentd surface. No new daemon endpoint.
- **G5** Compose with `spec-52 fleet exec` — both have the same
  "shape one action, fan out, render" pipeline. Share code where
  natural.

**Out of scope:**

- Stored fleet-wide observation history. Each `fleet observe` call
  is one-shot; if a consumer wants a timeline it can poll. (The
  drift loop in spec-58 already stores observation history for
  drift-comparison purposes; that's a different concern.)
- Aggregate predicates as part of the CLI ("are *all* peers' ports
  open?"). Use `jq` or a follow-up filter pipeline; don't grow a
  CLI predicate DSL (per non-goals.md "no expressive policy DSL").
- New peer-side endpoints. Use existing `submit_run` + the spec-59
  `observe.*` actions.

---

## Design

### Synthesized plan

The controller builds a one-step plan in memory:

```yaml
version: "1.0"
steps:
  - name: observe.port (fleet fan-out)
    observe.port:
      host: "{{ ... }}"
      port: 80
    as: result
```

Submits to each selected peer in parallel, collects the typed
`ObserveResult` from each peer's run, renders.

### CLI shape

```bash
$ mooncake fleet observe.port :80 --peer-filter 'role=web'
PEER          OPEN  LISTENER  PID
web-01        true  nginx     1234
web-02        true  nginx     1234
web-03        false -         -

$ mooncake fleet observe.gpu --peer-filter 'tag=gpu' --format json
[
  {"peer":"infer-01","value":{"count":1,"vendor":"nvidia","aggregate":{"memory_used_bytes":2147483648,"max_utilization_pct":42.0}}},
  ...
]
```

### Renderer

Per-kind column selection — `observe.port` shows OPEN/LISTENER/PID;
`observe.gpu` shows MEMORY-USED / UTIL%; `observe.service` shows
ACTIVE/ENABLED/SUB-STATE. Each observe handler ships a tiny "CLI
columns" hint (or the renderer just picks all primitive-typed
fields from the typed `Value` struct).

---

## Key files

| File | Change |
|---|---|
| `cmd/fleet_observe.go` | New. The fan-out command, dispatches per `observe.<kind>` subcommand. |
| `cmd/fleet_observe_render.go` | New. Per-kind tabular rendering. |
| `internal/fleet/observe.go` | New. Synthesize the one-step plan, parallel submit, collect typed results. |
| `internal/fleet/multiplex.go` | Reuse — already used by `fleet logs --all` and `fleet apply` multi-peer. |
| Add a peer-side test that `submit_run` correctly carries the typed observation result in its event payload. |

---

## Phases

1. Foundation + `fleet observe.port` (smallest renderer, exercises plumbing).
2. `fleet observe.service` + `fleet observe.process` (same shape, different value types).
3. `fleet observe.http` — adds the network-egress permission preflight wrinkle.
4. `fleet observe.gpu` — depends on spec-62 shipping.
5. Docs + an integration test against 2 in-process agentds.

---

## Acceptance criteria

- `mooncake fleet observe.port :80 --peer-filter 'tag=web'` returns a
  tabular comparison across selected peers.
- `--format json` produces a JSON array consumable by `jq`.
- `fleet observe.<kind>` errors cleanly when `<kind>` isn't a
  registered observe handler.
- Build / vet / lint / test green; integration test against in-process
  agentd cluster passes.

---

## Open questions

1. **Per-handler arg parsing on the CLI.** `observe.port :80` is
   handy shorthand; `observe.http https://x/health` is too. Do we
   write per-kind CLI arg parsers, or require users to pass YAML
   inline (`--args '{port: 80}'`)? Lean: per-kind shorthand for the
   common 3–4 handlers; YAML fallback for the rest.
2. **Permissions preflight at fan-out time.** If 2 of 10 peers don't
   have `nvidia-smi`, do we fail the whole command or surface
   `Found: false` for those peers and continue? Latter — partial
   answers are still useful, matches `fleet facts --query` behavior.
3. **Timeout per peer.** Default to 10s; user-overridable. A slow
   peer shouldn't block the whole fleet read.
4. **Output ordering.** Default to peer-name alphabetical; consider
   `--sort-by usage` once aggregations are common.

---

## Cross-references

- [`../action-surface/spec-59-typed-observability.md`](../action-surface/spec-59-typed-observability.md) — parent. Each observe action this command fans out is defined there.
- [`../action-surface/spec-60-observe-system-resources.md`](../action-surface/spec-60-observe-system-resources.md) — adds `observe.cpu/memory/disk` to the fan-out set.
- [`../action-surface/spec-62-observe-gpu.md`](../action-surface/spec-62-observe-gpu.md) — adds `observe.gpu`.
- [`spec-52-fleet-exec.md`](./spec-52-fleet-exec.md) — sibling spec. `fleet exec` is the *raw mutation* fan-out; `fleet observe` is the *typed read* fan-out. Share the multiplex/render pipeline.
- [`spec-58-fleet-drift.md`](./spec-58-fleet-drift.md) — the autonomous cousin. Drift loop runs observers on a cadence; `fleet observe` runs them on demand.
