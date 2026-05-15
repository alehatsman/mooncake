# Stream: agent

AI agent safety. The MCP server. The agent loop. The typed surface an
LLM uses to drive Mooncake without escape hatches.

If an autonomous agent (Claude / Cursor / Codex / a script your CI
runs) calls Mooncake to mutate a system, the safety guarantees come
from here.

This stream owns the "Docker for AI agents" wedge.

## Scope

| In | Out |
|---|---|
| MCP server (`run_step`, `get_facts`, `get_snapshot`, `check_plan`, `run_plan`, `query_file`) | Action handlers themselves (see [core](../core/README.md)) |
| Agent loop (iterate-until-done) | The handler ABI methods being declared (Core) |
| `transaction:` blocks with LIFO rollback (spec-30) | Multi-machine agent workflows (see [fleet](../fleet/README.md)) |
| `!secret` typed refs + 3 providers + redaction (spec-23 §3) | Human-shaped CLI UX (see [dx](../dx/README.md)) |
| `on_change:` reactive triggers (spec-23 §1) | |
| `try/catch/finally` compound steps (spec-23 §2) | |
| MCP wiring of Diff/Cost/Permissions (spec-22 phase 7) | |
| Plugin model and tier-2 plugin loader (spec-31, drafted) | |
| Policy DSL, plan signing, per-action quotas, egress policy, sandbox mode, deterministic replay — un-specced | |

## State

**Load-bearing primitives are shipped.** The agent-safety pitch on the
README is backed by runnable code (`examples/transactions/
rollback-demo.yml`). What remains is the policy / quota / signing /
replay layer — un-specced, incremental on top of the working
foundation.

Recent shipped specs (see commit history for the full receipts):

- spec-10 — MCP server
- spec-18 — agentd as the daemon surface
- spec-22 (phases 3–7) — four-method ABI declared + MCP-wired
- spec-23 — framework primitives (all three sections)
- spec-30 — `transaction:` + LIFO rollback

## Active specs

| Spec | Topic | Why drafted |
|---|---|---|
| [spec-31](./specs/spec-31-tier2-plugin-model.md) | Tier-2 plugin model | Tier-1 caps at ~30 actions deliberately. The next tier (`notify.*`, `container.compose`, `k8s.apply`, `db.postgres.*`) needs an extension shape that doesn't grow the main binary. Proof-of-concept domain: `notify.slack`/`webhook`/`email`/`pagerduty`. |

## Open gaps (un-specced)

The four-method ABI (Permissions / Diff / Reverse / Cost) is the
ground floor of agent safety. The next layer — risk-scoring, policy,
quota — is incremental on top. Each item below sits in the un-specced
backlog:

- **Policy DSL.** `deny: agent.touches("/etc/passwd")`-style patterns
  over `Permissions` + `Diff`. Hooks exist; the language doesn't.
- **Plan signing.** Sigstore-style signed plans; daemon refuses
  unsigned ones in prod mode.
- **Per-action quotas.** "This agent may make at most 10 file edits
  this run."
- **Egress policy.** "This agent may only download from npm / pypi /
  github."
- **Sandbox mode.** Agent loses shell entirely; only the typed ABI
  surface is reachable. Filesystem and network mediated by Mooncake.
- **Deterministic replay.** `mooncake replay <run-id>` — re-execute an
  agent's exact plan deterministically. The audit trail already has
  every input; the replay command doesn't exist yet.
- **Cost / risk classifier.** `Cost()` per handler provides the input;
  an aggregation/risk-scoring layer on top is the next piece.

## Why this stream is the defensible wedge

> "plan + snapshot + reverse + deterministic replay, all typed
> end-to-end."

Three of four are in master and demoable. Deterministic replay is the
last open piece. An Ansible+OPA+AWX combo can audit (AWX) and gate
(OPA) but cannot automatically revert a half-applied transaction
byte-identically to pre-state — because no handler in that stack
declares a `Reverse()` method.

Existing agent sandboxes (Daytona, E2B, Modal) sandbox the
*environment*. Mooncake sandboxes the *intent*. They're complementary;
Mooncake fits *inside* their VMs.

LLM vendors will not build this — it's orthogonal to their model
business.

## Cross-stream dependencies

- [core](../core/README.md) — every Core action is automatically an
  Agent surface via MCP. Agent's work is to expose / mediate / police,
  not to add actions.
- [fleet](../fleet/README.md) — agentd carries MCP-ready event
  streams; agents can drive multi-peer plans through the same
  primitives.
- [dx](../dx/README.md) — human-shaped UX is intentionally separate.
  The DX team optimizes for "what does a person see?"; Agent
  optimizes for "what JSON does an LLM consume?"
