# agent Proposals

Six proposals for the agent stream — MCP server, agent loop, the
safety story for LLM-driven mooncake. Brainstormed after the
2026-05-15 manual audit's MCP rounds (#22, #25, #54).

The agent stream's wedge is "Docker for AI agents". These
proposals make that pitch concrete: agents can discover what
mooncake does (01), watch runs incrementally (02), cancel mid-run
(03), audit changes before applying (04), reason about past runs
(05), and operate under enforced permissions (06).

| # | Proposal | Effort | Value | Why |
|---|---|---|---|---|
| [01](./proposal-01-mcp-tool-inventory.md) | `list_actions` / `describe_action` / `list_presets` MCP tools | XS | High | Agents can't introspect capabilities today |
| [02](./proposal-02-mcp-streaming-events.md) | Stream `run_plan` events incrementally | S | High | Long runs return one big answer; need progress signal |
| [03](./proposal-03-mcp-cancel-plan.md) | `cancel_plan` MCP tool | XS | High | Agent loop's "stop, I changed my mind" primitive |
| [04](./proposal-04-diff-plan-tool.md) | `diff_plan` typed pre-execution diff | S | High | `check_plan` returns structure; agents want typed deltas |
| [05](./proposal-05-mcp-history-and-replay.md) | `list_runs` / `get_run` / `replay_run` | S | High | Loop state across reconnects; reconstruct context |
| [06](./proposal-06-permissions-as-contract.md) | Declared allow_permissions; reject plan if exceeded | M | Highest | The safety pitch made enforceable |

## Recommended order

1. **01 MCP tool inventory** — XS, instant capability surface
2. **05 MCP history / replay** — S, agent loop state recovery
3. **04 diff_plan** — S, gates the human-review or policy gate
4. **03 cancel_plan** — XS, pairs with fleet proposal-02
5. **02 streaming events** — S, pairs with 03
6. **06 permissions as contract** — M, the headline safety
   feature; build on the rest of the surface

## Cross-cutting themes

### Theme A: Self-describing surface

Today the MCP server has 6 tools. None of them tell the agent what
the server can do beyond their own metadata. Proposal 01 fixes
that. Once agents can introspect (`list_actions`,
`describe_action`), the rest of the proposals compose into
verb-based vocabulary an agent can discover.

### Theme B: Loop primitives

A real agent loop needs: stream events (02), cancel mid-stream
(03), inspect plans before running (04), recall past runs (05).
Today: none of these. With them, agent loops become viable for
long-running, stateful workflows.

### Theme C: Permissions as contract, not summary

The current `requires:` field in `run_plan` is a *summary* of
what the plan needs. Proposal 06 makes it a *contract*: the
agent declares allowed permissions; the server rejects plans
that exceed. This converts the safety pitch from "human watches
the output" to "machine enforces the rule".

## Mirror across streams

Several proposals here mirror proposals in other streams:

| Agent | Mirror |
|---|---|
| 01 list_actions | DX proposal-04 (`mooncake actions show`) |
| 03 cancel_plan | Fleet proposal-02 (`fleet kill`) |
| 04 diff_plan | Core proposal-04 (typed plan diff) |
| 05 list_runs | DX's existing `mooncake history` |
| 06 allow_permissions | (no current mirror; future fleet `--allow-permissions`) |

The data and primitives are shared; each stream's surface wraps
the same kernel functionality.

## What's NOT in this batch

Bigger un-specced features mentioned in `agent/README.md`'s open
gaps:
- **Policy DSL** — declarative rules engine for agent-driven flows
- **Plan signing** — cryptographic attestation that a plan came from a trusted source
- **Per-action quotas** — rate limits, cost budgets
- **Sandbox mode** — chroot / namespace isolation for agent runs
- **Deterministic replay** — full record-and-replay for testing
  agent flows

Each is its own spec. Several compose on top of proposal 06's
permission framework.

## Audit receipts

- **#22** (`mooncake step` truncation, fixed MT-22)
- **#25 / #26** (MCP notification protocol nits, fixed MT-25)
- **#54** (run_plan all-zero counters, fixed)
- **agent README's "Open gaps (un-specced)"** — Policy, quota,
  signing, replay items all build on the proposals here

These proposals don't re-litigate fixed issues; they extend the
working foundation toward the agent-safety story the stream
promises.
