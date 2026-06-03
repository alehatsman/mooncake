---
id: mcp-server
status: draft
owners: [aleh]
covers:
  - internal/mcp/**
  - cmd/mcp/**
---

# MCP Server

## Intent

Mooncake exposes its kernel operations to LLM tool-call clients as a Model
Context Protocol server — a minimal JSON-RPC 2.0 implementation over stdio
(`mooncake mcp`) that also backs the daemon's `/v1/mcp` endpoint via one shared
tool registry. Agents introspect the machine, preview and apply typed plans,
and read the action ABI without shelling out, every mutation flowing through
the same typed funnel as the CLI.

## Behavior

- WHEN started, the server speaks JSON-RPC 2.0: it answers `initialize`,
  `tools/list`, `tools/call`, and `ping`; an unknown method returns -32601 and
  a malformed line returns -32700.
- WHERE a request is a JSON-RPC notification (absent/null `id`, `initialized`,
  or any `notifications/*` method), the server emits no response.
- WHEN `tools/call` names a tool, the server dispatches to its handler and
  wraps the handler's string return in an MCP `content` text block; an unknown
  tool returns -32601 and a handler error returns -32000.
- WHERE the tool surface is defined, `RegisterAllTools` registers one canonical
  set so the stdio command and the daemon's HTTP transport expose identical
  tools.
- WHEN an agent inspects the host, `get_facts`, `get_snapshot`, `fact_query`,
  `get_metrics`, and `query_file` return read-only JSON.
- WHEN an agent introspects the action ABI, `list_actions`, `describe_action`,
  and `list_presets` return the typed contract and capability matrix
  (check/diff/cost/reverse/permissions) without shelling out.
- WHEN an agent previews a config, `check_plan` returns per-step inspections
  (would-change, aggregated permissions, cost summary) with no side effects.
- WHEN an agent applies a config, `run_plan` executes it through the kernel
  (per-step diff + cost + apply outcome) and refuses any step exceeding the
  optional `policy` permissions-as-contract gate (#11) before its side effect.
- WHILE `run_plan` runs, it MUST stream run events to the caller incrementally
  rather than returning only a final summary (#7 — drift, not built).
- WHEN a long `run_plan` is in flight, an agent MUST be able to cancel it
  mid-run via a `cancel_plan` tool (#8 — drift, not built).
- WHEN an agent reasons about a config before approval, a `diff_plan` tool MUST
  return a typed pre-execution diff (the intended state delta, distinct from
  `check_plan`'s structural inspection) (#9 — drift, not built).
- WHEN an agent reconstructs context across turns, `list_runs`, `get_run`, and
  `replay_run` tools MUST expose the persisted run history (#10 — drift, not
  built).
- WHEN a plan needs to call another MCP server's tool, an `mcp_tool` action
  MUST let that call run as a kernel step inheriting diff/perms/risk/replay
  (#12 — drift, not built).

## Non-goals

- The agent loop that drives an LLM to produce plans — `internal/agent/**`.
- The agent daemon and its run store/HTTP lifecycle — `internal/agentd/**`.
- The executor, action handlers, and transaction semantics the tools invoke —
  owned by the kernel specs; tools are a thin typed surface over them.

## Checklist

- [x] JSON-RPC 2.0 stdio transport: initialize / tools/list / tools/call / ping
- [x] Notification suppression + parse/method/tool error codes
- [x] `DispatchBytes` shared with the daemon `/v1/mcp` HTTP transport
- [x] Read-only tools: get_facts, get_snapshot, fact_query, get_metrics, query_file
- [x] ABI introspection: list_actions, describe_action, list_presets
- [x] `check_plan` structural preview (per-step inspections, permissions, cost)
- [x] `run_plan` apply with optional `policy` gate (#11)
- [ ] `run_plan` incremental event streaming (#7 — todo, not built)
- [ ] `cancel_plan` tool (#8 — todo, not built)
- [ ] `diff_plan` typed pre-execution diff tool (#9 — todo, not built)
- [ ] `list_runs` / `get_run` / `replay_run` history tools (#10 — todo, not built)
- [ ] `mcp_tool` action — call MCP tools through the kernel (#12 — todo, not built)
