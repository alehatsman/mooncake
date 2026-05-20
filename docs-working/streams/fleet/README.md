# Stream: fleet

Multi-machine. agentd, peer transport, the multiplexer, fleet
subcommands. Everything that turns `mooncake apply` from a one-box
operation into a peer-to-peer fleet workflow.

If you can `ssh` to it or hit `/v1/version`, it lives here.

## Scope

| In | Out |
|---|---|
| agentd (TCP listener, bearer auth, SSE event hub, sandboxed file sync) | Local-only `mooncake apply` (see [core](../core/README.md)) |
| Peer transport (`peers.toml`, native SSH driver, mDNS discovery) | Enterprise control-plane hub (intentionally deferred — see goals §4) |
| `fleet apply` / `bootstrap` / `discover` / `init` / `status` / `logs` / `facts` / `exec` / `watch` / `ps` / `upgrade` / `doctor` / `observe` | |
| Per-host overlays + tag-based peer selection | |
| `fleet apply <machine>` ordered multi-peer phasing | |
| Windows agentd | |

## State

**Personal-fleet v1 is shipped end-to-end.** Original 14-PR plan is
14/14 closed. Three drafted post-v1 specs remain.

Recent shipped specs (see commit history for the full receipts):

- spec-43 / 44 — fleet transport + SSH bootstrap
- spec-45 — fleet discovery (simple + mDNS + interactive `fleet init`)
- spec-46 — `fleet status` + `fleet logs`
- spec-47 — bootstrap UX (8-step orchestration)
- spec-48 — per-host overlays + tags
- spec-49 — agentd on Windows
- spec-50 — extended filter keys (`os=`, `name=`, `role=`)
- spec-51 — local-apply overlay parity
- spec-52 / 53 / 54 — `fleet exec` / `watch` / `ps`
- spec-56 — Windows fleet bootstrap
- spec-57 — `windows.firewall_rule` + `windows.scheduled_task`
- spec-64 — cross-peer `fleet observe`
- R2.1c phase 1 — `apply.KernelResult` round-trips over the agentd wire; `FleetKernelResult.Reverse()` now composes against typed Steps from each peer (was `ErrPerPeerKernelResultNotWired`).
- R2.1c phase 2 — `Result.ReverseData` round-trips over the agentd wire via a discriminator envelope + per-handler `executor.RegisterReverseDataType`; 16 handler `init()` registrations alongside `actions.Register`. Closes the per-peer `Reverse()` gap end-to-end (`5dd81b95`, 2026-05-17).
- spec-70 — `agentd bootstrap` subcommand; extract install primitives + user-mode (`--user`) no-SSH path (phases 1–3, `0d41e058`–`bbb80b74`).
- spec-72 — Unified escalation policy: one `EscalationReport` + `ProbeEscalation`, one `BecomeRunner`, one decision point. Phases α/β/γ + phase 4 + cleanup all shipped.

Plus three operational features delivered outside the original plan:
`fleet apply <machine>` (ordered phases), `fleet upgrade` (Linux +
Windows), and the `fleet doctor` per-peer probe ladder.

**Verified against a real WSL + Windows two-peer testbed** including
`running` / `failed` / `unreachable` health states. Not slideware.

## Active specs

| Spec | Topic | Why drafted |
|---|---|---|
| [spec-55](./specs/spec-55-fleet-doctor.md) | Fleet doctor fan-out | Draft, not started. Single-host probe ladder shipped; the multi-peer wrapper that aggregates ~16 checks across the fleet is still drafted. |
| [spec-58](./specs/spec-58-fleet-drift.md) | Fleet drift | Draft, not started. Periodic `InspectPlan` loop + `/v1/drift` + per-machine `drift:` policy. The single feature that would turn Mooncake from "config management tool" into "fleet operating system." |
| [spec-71](./specs/spec-71-fleet-init-auto-pair.md) | `fleet init` auto-pair via SSH | Draft, not started. Eliminates the manual token-paste step — SSH-fetch the bearer token and optionally bootstrap the daemon in one command. |

## Open gaps

- **Enterprise hub.** Intentionally deferred. No active spec until a
  paying user asks for inventory / RBAC / approval gates / audit
  export. The personal-fleet stream proves the wire protocol; the hub
  is a separate epic built on top.
- **macOS agentd.** Functional, but preset coverage is uneven vs
  Linux. Surfaces as a [dx](../dx/README.md) follow-up more than a
  fleet gap.

## Cross-stream dependencies

- [core](../core/README.md) — Fleet runs Core plans on agentd. Every
  new action automatically works fleet-wide; no fleet-specific code
  per action.
- [agent](../agent/README.md) — `fleet apply` events flow through the
  MCP event surface; agents can drive multi-peer plans through the
  same typed ABI.
- [dx](../dx/README.md) — `mooncake fleet init` shares the onboarding
  pattern with `mooncake init`; the doctor command's single-host
  probes (Stream Dx) are what spec-55 will fan out across peers.
