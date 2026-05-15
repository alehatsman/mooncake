# Spec 31: Tier-2 Plugin Model — `notify.*` proof of concept

**Status:** Draft
**Epic:** E9 Modern Action Surface — bucket E9.5
**Effort:** L (2 weeks)
**Value:** Strategic. Establishes how Mooncake's action surface
extends *beyond* the Tier-1 kernel without ballooning the main binary
or polluting the schema. Even modest community adoption depends on
having a sensible plugin shape.

**Design principles:** `docs-working/action-design-principles.md`

---

## Problem

Tier-1 caps at ~30 actions; that's deliberate. Everything else —
container.compose, k8s.apply, dns.cloudflare.record, secret.vault,
notify.slack, db.postgres.* — needs to live outside the main binary.
Three reasonable distribution models:

1. **Static-linked Go modules.** Build-time compose; mooncake binary
   ships with whatever plugins were enabled.
2. **Out-of-process subprocesses.** HashiCorp-style; plugins are
   separate binaries the engine talks to over gRPC/stdin-stdout.
3. **WASM.** Sandboxed in-process; cross-language; ecosystem still
   maturing.

We don't have to pick one forever, but we DO need to ship the first
official Tier-2 plugins to prove the pattern. This spec picks
notifications (`notify.slack`, `notify.webhook`, `notify.email`,
`notify.pagerduty`) as the proof-of-concept domain because:

- Low blast radius (sends an HTTP request — can't break the system).
- Multiple drivers needed (Slack, webhook, email, PagerDuty) →
  exercises the driver-dispatch shape.
- High demand from agent-developer users.

---

## Goals

- **G1** Define the Tier-2 plugin loader shape. v1: static-linked Go
  modules, opt-in via build tags. Document the WASM/subprocess paths
  as future work.
- **G2** Ship `notify.slack`, `notify.webhook`, `notify.email`,
  `notify.pagerduty` as the first Tier-2 actions.
- **G3** Establish a plugin's relationship to the schema: plugins
  augment the schema at build/start time, not edit the main
  `schema.json` source.
- **G4** Define how plugins implement spec-22 sub-interfaces.
- **G5** Document the path from "in-tree Go plugin" → "out-of-tree
  community plugin" so the eventual WASM/subprocess transition is
  predictable.

**Out of scope:**

- WASM runtime integration. Detailed sketch only; implementation in a
  follow-up.
- Subprocess plugins.
- Marketplace UI / signing / discovery.
- Paid plugins / revenue share.

---

## Design

### Plugin shape (v1: static Go module)

A plugin lives in `internal/plugins/<name>/` (during the proof-of-
concept phase; later moves to `pkg/plugins/<name>/` so it can be
imported by external consumers).

A plugin registers actions by calling `actions.Register(...)` from an
`init()` function — same as Tier-1. The difference is purely
organizational:

1. Plugin code lives under `internal/plugins/<name>/` (not
   `internal/actions/<name>/`).
2. Plugin packages are conditional on a Go build tag, e.g.
   `//go:build mooncake_notify`.
3. The plugin's schema fragment lives next to the code as
   `schema.json.frag`; `mooncake schema generate` merges enabled
   plugin fragments into the main schema.

Default `make build` enables all in-tree Tier-2 plugins. Stripped
builds (e.g. for embedded use) can disable via build tags.

### Plugin schema fragment

Each plugin ships a self-contained fragment:

```json
{
  "$id": "notify",
  "actions": [
    "notify.slack",
    "notify.webhook",
    "notify.email",
    "notify.pagerduty"
  ],
  "definitions": {
    "notify.slack": { "type": "object", "properties": { ... } },
    "notify.webhook": { ... },
    ...
  }
}
```

`mooncake schema generate` produces the main `schema.json` by
unioning these fragments with the Tier-1 actions' generated schema.

### `notify.*` actions

Common shape:

```yaml
- notify.slack:
    webhook: !secret env:SLACK_WEBHOOK
    message: "Deploy succeeded for {{ service }}"
    channel: "#ops"

- notify.webhook:
    url: https://hooks.example.com/deploy
    method: POST
    headers:
      Content-Type: application/json
    body: '{"event": "deploy", "service": "{{ service }}"}'

- notify.email:
    to: ops@example.com
    subject: "Deploy {{ status }}"
    body: "..."
    smtp:
      host: smtp.example.com
      username: !secret env:SMTP_USER
      password: !secret env:SMTP_PASS

- notify.pagerduty:
    routing_key: !secret env:PD_ROUTING_KEY
    severity: error
    summary: "{{ service }} is down"
```

All four:
- Implement spec-22 hooks. `Diff: noop` (notifications don't change
  declarative state). `Reverse: nil` (can't unsend). `Cost: Risk: 2`.
  `Permissions: Network: true`.
- Accept `!secret` refs for any credential field.
- Document idempotency: notifications always fire (not idempotent in
  the strict sense). For idempotency-aware use, gate with
  `when: <something>.changed`.

### Path to out-of-tree plugins (future)

This spec doesn't implement the out-of-tree path; it commits to
making it possible without redesign:

1. **WASM** — a future spec adds a WASM runtime (e.g. wazero) that
   loads `*.wasm` plugins from `~/.mooncake/plugins/`. The plugin
   exposes the same `Handler` interface via a WIT contract. Schema
   fragment ships inside the .wasm.
2. **Subprocess** — alternative path for languages with weak WASM
   support. Plugin is an executable; engine speaks Protocol Buffers
   over stdin/stdout (HashiCorp's `go-plugin` library is the obvious
   choice).
3. **Marketplace** — much later. `mooncake plugin install <name>`
   fetches signed .wasm or binary from a registry.

The in-tree Go pattern stays — we don't kill it when out-of-tree
arrives. The distinction is: Tier-1 = Mooncake core, Tier-2 in-tree
= official-maintained, Tier-3 out-of-tree = community.

### Plugin discovery + listing

```
mooncake plugins list
> notify              built-in   v1.0   actions: notify.slack, notify.webhook, notify.email, notify.pagerduty
> container           built-in   v1.0   actions: container.run, container.image, ...
```

Tier-1 actions also appear in the list (with `built-in` source) so
users can introspect the full surface.

---

## Key files

| File | Change |
|---|---|
| `internal/plugins/notify/` | New. Four action handlers, schema.json.frag, README. |
| `internal/plugins/registry.go` | New. Build-tag-aware plugin enumeration. |
| `internal/schemagen/plugins.go` | New. Merges plugin schema fragments into main schema. |
| `cmd/plugins.go` | New. `mooncake plugins list / info <name>`. |
| `Makefile` | New `build-stripped` target that disables Tier-2 build tags. |
| `docs-next/plugins/authoring.md` | New doc page. |

---

## Tasks (phased)

1. **Phase 1** — `internal/plugins/notify/webhook/` — the simplest
   driver. Establishes the in-tree shape.
2. **Phase 2** — `notify.slack`, `notify.email`, `notify.pagerduty`
   in parallel.
3. **Phase 3** — Schema fragment loader. `make schema-generate`
   merges fragments.
4. **Phase 4** — Build-tag plumbing. Default-enabled; opt-out builds.
5. **Phase 5** — `mooncake plugins list` / `info`.
6. **Phase 6** — Authoring docs.

---

## Acceptance criteria

- `notify.webhook` posts JSON to a test server end-to-end. Credential
  via `!secret env:` doesn't appear in events or runlog.
- `notify.slack` succeeds against a real Slack webhook (or a recorded
  mock).
- `mooncake plugins list` shows the notify plugin.
- `mooncake plan --format json` includes notify.* actions in the
  oneOf surface.
- `mooncake build-stripped` produces a binary without notify actions
  (schema doesn't include them).
- Build / vet / lint / test green.

---

## Open questions

1. **`notify.email` SMTP — pluggable transports?** Probably defer to
   tier-3; just SMTP for v1.
2. **`notify.pagerduty` v1 vs v2 events API?** Events V2.
3. **What goes in spec-22's `Reverse` for "unsend"?** Decline:
   notifications are explicitly declared `reversible: false`. The
   notification was sent; can't unsay it.
4. **Should `notify.*` be batchable?** ("Send a single Slack message
   per N steps' worth of changes.") Probably no — `for_each` over a
   summary builder is the right shape. Maybe later via on_change
   triggers.
5. **Schema fragment format vs full JSON schema?** Lean: simplified
   fragment (just the `definitions` and action-name list) gets
   spliced into the main schema by `schemagen`.
