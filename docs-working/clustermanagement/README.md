# Cluster Management — brainstorm

Working notes on where the personal-fleet stream could go next, assuming
agentd is running and reachable on every machine. Two framings inside:

- **[qol-features.md](./qol-features.md)** — near-term quality-of-life
  features for daily fleet operations. Concrete, ranked by leverage and
  implementation cost. Each item builds directly on the existing agentd
  surface (TCP + bearer + SSE + run-submit + multiplexer + per-host
  overlays). Tier-1 items are ~100–200 LOC apiece.
- **[agentic-interface-brainstorm.md](./agentic-interface-brainstorm.md)**
  — wider, more speculative. Frames mooncake as **the typed interface
  an LLM uses to drive 1–10 machines reliably**, and asks what the
  surface looks like at full bloom. 10 themes from typed observability
  (the missing half of the ABI) through federated MCP to per-peer
  micro-operators.

Both lists were generated from a single brainstorming pass on
2026-05-15; treat them as a creative starting point, not a roadmap.
None of these are specced yet — when one gets picked up, draft a
proper spec under `docs-working/specs/personal-fleet/` (or wherever
the surface area lives) and reference back here.

---

## Strategic context

`PROGRESS.md` rev12 captures the current state: 13/14 personal-fleet
PRs shipped, only the interactive `fleet init` flow left in Phase C
polish. The cluster-management gap *isn't* missing primitives any more
— the kernel, transport, and orchestration all work end-to-end against
a real WSL+Windows testbed. The gap is in operator (and agent) **UX
around the running fleet**: live introspection, observation, drift
detection, cross-peer queries, conversation-scoped state.

The agentic-interface brainstorm argues the strategic plant-the-flag
move is **typed observability primitives** — the mirror image of the
typed-mutation surface that already exists. It's the single
foundation that unblocks the most other ambitious ideas (reconciliation
loops, fleet-graph queries, replay, conversation-scoped state). The
honourable mention is **federated MCP** — the marketing surface that
makes "connect your editor's agent to a typed fleet of personal
computers" demo-able.
