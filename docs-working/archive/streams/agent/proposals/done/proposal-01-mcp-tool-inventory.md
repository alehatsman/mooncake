# Proposal 01: MCP discovery tools — `list_actions`, `describe_action`, `list_presets`

**Status:** Shipped 2026-05-17 (commit `d5eef65c`)
**Effort:** XS (~1 day)
**Value:** High — agents today can't introspect mooncake's
capabilities without out-of-band knowledge. They have to be told
"call run_step with `pkg: ...`" rather than asking the server "what
actions can I call?"

---

## Problem

Today the MCP server exposes 6 tools:
- `get_facts` / `get_snapshot` / `fact_query` / `get_metrics`
  (read-only system state)
- `run_plan` / `check_plan` (apply / dry-run a plan file)

What's missing is **capability discovery**. An agent connecting fresh
to a mooncake MCP server should be able to ask:

- "What actions does this mooncake support?"
- "What parameters does `pkg` take?"
- "What presets are available?"

Today the agent either has to ship with hard-coded knowledge of the
action surface, or shell out to `mooncake actions list` /
`mooncake schema generate`. Both are brittle:

- Hard-coded knowledge drifts (the validator alone covered the
  drift problem at finding #27; same shape across agent integrations).
- Shelling out works on the same host but not across MCP transports
  (claude desktop can't shell out to your mooncake binary).

For the "Docker for AI agents" pitch to hold, the **agent
integration surface must be self-describing**.

## Proposal

Three new MCP tools:

### `list_actions`

```jsonc
// Request
{"method": "tools/call", "params": {"name": "list_actions", "arguments": {}}}

// Response (content[0].text contains JSON-stringified):
{
  "actions": [
    {"name": "file.write", "category": "file", "platforms": ["linux", "darwin", "windows"]},
    {"name": "file.copy", "category": "file", ...},
    {"name": "pkg", "category": "system", ...},
    ...
  ],
  "total": 44
}
```

Optional filter:
```jsonc
{"method": "tools/call", "params": {"name": "list_actions", "arguments": {"category": "file"}}}
```

### `describe_action`

```jsonc
// Request
{"method": "tools/call", "params": {"name": "describe_action", "arguments": {"name": "pkg"}}}

// Response
{
  "name": "pkg",
  "description": "Install/remove packages via the OS package manager",
  "category": "system",
  "platforms": ["linux", "darwin", "windows"],
  "requires_sudo": true,
  "capabilities": {
    "check": "yes",
    "diff": "yes",
    "cost": "yes",
    "reverse": "partial"
  },
  "parameters": {
    "required": [{"name": "name", "type": "string"}],
    "optional": [
      {"name": "state", "type": "string", "enum": ["present", "absent"], "default": "present"},
      {"name": "version", "type": "string"},
      {"name": "manager", "type": "string", "enum": ["apt", "dnf", "pacman", "brew", "choco"]}
    ]
  },
  "result_schema": {
    "operation": "create|update|delete|noop",
    "target": "string (package name)",
    "value": {"version": "string", "manager": "string"}
  },
  "example_minimum": {
    "pkg": {"name": "jq"}
  }
}
```

Maps directly to the data DX proposal-04 (`mooncake actions show`)
surfaces; same source.

### `list_presets`

```jsonc
// Request
{"method": "tools/call", "params": {"name": "list_presets", "arguments": {}}}

// Response
{
  "presets": [
    {"name": "jq", "version": "1.0.0", "description": "Install jq..."},
    {"name": "docker", "version": "1.0.0", "description": "Install Docker..."},
    ...
  ],
  "total": 20
}
```

Optional `describe_preset` (defer to v1.1 if scope is tight):
```jsonc
{"method": "tools/call", "params": {"name": "describe_preset", "arguments": {"name": "jq"}}}
// Returns preset.yml's parameters block + steps overview
```

## Receipts

From the audit:
- I had to `mooncake schema generate` ~50 times to look up action
  fields. An agent doing the same workflow has worse access (no
  shell).
- `mooncake actions list` and `mooncake presets list` both exist —
  the data is on the controller side, just not exposed via MCP.
- The MCP server tooling story is otherwise nailed (round-trippable
  initialize, clean errors). This proposal completes it.

## API

Same MCP server, three new tools added to `tools/list` output. No
breaking changes.

The discovery tools should advertise themselves prominently. After
`initialize`, a polite MCP client would call `tools/list` and see:

```
get_facts, get_snapshot, fact_query, get_metrics, run_plan, check_plan,
list_actions, describe_action, list_presets
```

…and learn what the server can do. Self-describing tools are the
point of MCP.

## Implementation

Each new tool maps to existing internal data:

```go
// internal/mcp/tools/list_actions.go
func (s *Server) listActions(args ListActionsArgs) (*ListActionsResult, error) {
    actions := registry.AllActions()
    if args.Category != "" {
        actions = filterByCategory(actions, args.Category)
    }
    return &ListActionsResult{
        Actions: actions,
        Total:   len(actions),
    }, nil
}
```

`registry.AllActions()` already exists for `actions list`. The MCP
binding is a thin wrapper.

## What this doesn't address

- **Action result streaming** — see proposal-02 (MCP streaming
  events). Knowing what an action does is separate from watching
  it execute.
- **Action capability search** ("which actions can install
  packages?") — could be a future `find_actions {capability:
  install_packages}` tool. Defer until there's user demand.
- **Preset parameter introspection** — `describe_preset` is part of
  the proposal; the deeper "what does running this preset
  actually do?" would call into the planner and is a separate
  feature.

## Pairs with

- **DX proposal-04** (`mooncake actions show`) — same data, two
  surfaces (CLI + MCP)
- **Core proposal-05** (capability flags) — provides the
  `Permissions/Diff/Cost/Reverse` columns that `describe_action`
  returns
- **Core proposal-01** (result schema conventions) —
  `describe_action.result_schema` reflects the standardized
  envelope
