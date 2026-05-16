# Proposal 07: `mcp_tool` action — invoke any MCP tool through the kernel

**Status:** Draft proposal (brainstorm-stage)
**Effort:** M (~1 week — depends on a small MCP client in `internal/`)
**Value:** Highest of the agent-stream backlog — turns every MCP
server in the ecosystem into a mooncake-managed action, inheriting
diff / perms / risk / replay for free.

---

## Problem

Today the agent integration is one-directional: mooncake exposes an
MCP server so agents can call mooncake. The reverse — **mooncake
calling an MCP tool as a step in a plan** — is a missing primitive,
and it's the one that converts mooncake from "tool agents use" into
"runtime agents live inside".

When an agent wants to send a Slack message, open a Linear issue,
read a Google Drive file, or call any other MCP-exposed verb, today
it does so outside mooncake's safety surface. That means:

- No diff preview (`plan --diff` shows nothing for the call).
- No permission gate — the agent makes the call directly; mooncake
  has no chance to require `allow: send_slack_message`.
- No risk classification — destructive calls (delete-issue,
  send-email) look the same as read-only calls.
- No replay — the call doesn't land in `mooncake history`, so the
  "what did the agent do?" audit trail has holes.
- No idempotency — re-running the plan re-sends the message.

The "Docker for AI agents" pitch leaks at exactly this boundary.

## Proposal

A new core action: `mcp_tool`. Step shape:

```yaml
- mcp_tool:
    server: slack         # named MCP server from mooncake config
    tool: send_message
    args:
      channel: "#ops"
      text: "deploy of {{ vars.service }} complete"
    cache_key: "deploy-{{ vars.service }}-{{ vars.build_sha }}"
    expect:
      ok: true
    risk: high            # explicit; default inferred from server policy
```

### Semantics

- **Idempotency** comes from `cache_key`. If a previous run with the
  same key succeeded, the action is a no-op. The cache lives in
  `~/.mooncake/mcp-cache/` and is part of the history record.
- **Diff** is structural — the planner shows `would call
  slack.send_message(channel="#ops", text="...")` and, for tools that
  declare a `dry_run` capability via their schema, can call the
  server's dry-run path.
- **Perms** flow through the existing kernel: `mcp_tool` declares
  `permissions: [mcp:slack/send_message]`. The agent-stream
  permission contract (proposal-06) gates the whole plan against
  declared allows.
- **Risk** is per-server, configurable: `slack.send_message` is
  `high` by default; `slack.list_channels` is `low`. Defaults live
  in the server registration block, overridable per step.

### Server registration

```yaml
# ~/.mooncake/mcp-servers.yml
servers:
  slack:
    transport: stdio
    command: ["mcp-slack", "--auth-from-keychain"]
    default_risk:
      send_message: high
      delete_message: high
      list_channels: low
  linear:
    transport: http
    url: https://mcp.linear.app
    auth: !secret linear_token
```

### Result envelope

Same shape as other actions (proposal 01 in core):

```json
{
  "operation": "create",
  "target": "slack.send_message",
  "value": {"channel": "#ops", "ts": "1717..."},
  "skipped": false,
  "error": null
}
```

A cache hit returns `operation: noop`.

## Why this is the kernel moat working as designed

Every existing mooncake action follows the same recipe: declare what
it touches, what it changes, how to check, how to diff, how to
reverse. `mcp_tool` is *that recipe wrapped around an opaque tool
call*. The kernel doesn't need to know what Slack is — it just needs
the cache key, the risk tag, and the result. The MCP server provides
the rest.

That's why this is the highest-value action to add: it multiplies the
kernel surface by the size of the MCP ecosystem without growing the
kernel itself.

## What this doesn't address

- **Streaming MCP responses** — `mcp_tool` is request/response. Long
  tool calls block. A future `mcp_tool_stream` could lift this.
- **MCP resource subscription** — different shape (long-lived,
  push-based); pair with the `watch` action (core proposal-14).
- **Dynamic tool discovery within a plan** — plans are static today;
  if an agent wants to *decide which MCP tool to call* based on a
  result, that's the planner-loop story (out of scope).

## Field-budget impact

Zero universal fields. Everything is in the `mcp_tool:` block:
`server`, `tool`, `args`, `cache_key`, `expect`. This is the
template for "new actions go in `args`, not on `Step`" — see
CLAUDE.md soft cap §2.

## Pairs with

- **Agent proposal-06** (permissions as contract) — declared allows
  must include `mcp:<server>/<tool>` for the plan to run.
- **Core proposal-01** (result schema) — `mcp_tool` follows the
  standard envelope.
- **Core proposal-04** (typed plan diff) — `mcp_tool` contributes a
  typed diff renderer (tool name + redacted-or-shown args).
- **Core proposal-14** (`watch` action) — for MCP resources that
  push instead of being polled.
