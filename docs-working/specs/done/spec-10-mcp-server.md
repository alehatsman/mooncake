# Spec 10: MCP Server Mode

**Epic:** E6 Agent-Native Interface (S6.1)  
**Effort:** L (1–2 days)  
**Value:** Very High — makes mooncake directly callable from Claude, Cursor, or any MCP client

---

## Problem

AI agents interact with mooncake as a shell subprocess: they shell out, parse stdout,
and get no structured feedback until the process exits. This means:
- No incremental events during a long run
- Extra prompt-injection surface from stdout parsing
- No way to call a single action and inspect its result

The Model Context Protocol (MCP) solves this: mooncake exposes a set of tools that any
MCP client can call directly, with structured inputs and outputs.

---

## Goal

`mooncake mcp` starts an MCP server (stdio transport by default) that exposes five tools:

| Tool | Description |
|------|-------------|
| `get_facts` | Return system facts as JSON |
| `get_snapshot` | Return compact system snapshot (text or JSON) |
| `fact_query` | Dot-path query into facts tree |
| `check_plan` | Dry-run a config, return would-change list |
| `run_plan` | Run a config file, return structured result |

---

## MCP Transport

Default: **stdio** (stdin/stdout). This is what Claude Desktop and most MCP clients use.
The server speaks JSON-RPC 2.0 framed per the MCP spec.

Optional flag: `--transport sse` for SSE (HTTP) transport for remote agents.

---

## Tool Definitions

### `get_facts`
**Input:** `{}` (no parameters)  
**Output:** Full facts object as JSON (same as `mooncake facts --format json`)

### `get_snapshot`
**Input:** `{"format": "text"|"json", "budget": integer}`  
**Output:** System snapshot string (text) or object (json)

### `fact_query`
**Input:** `{"query": "go.version"}` — dot-path  
**Output:** `{"value": "1.26.3", "found": true}`

### `check_plan`
**Input:** `{"config": "path/to/config.yml"}`  
**Output:**
```json
{
  "steps": [
    {"name": "Install neovim", "action": "package", "would_change": true},
    {"name": "Set shell to zsh", "action": "shell", "would_change": false}
  ],
  "total": 10,
  "would_change": 3,
  "would_skip": 2
}
```

### `run_plan`
**Input:** `{"config": "path/to/config.yml", "dry_run": false}`  
**Output:**
```json
{
  "changed": 3,
  "ok": 58,
  "skipped": 8,
  "failed": 0,
  "duration_ms": 274000,
  "steps": [
    {"name": "Install neovim", "action": "package", "changed": true, "duration_ms": 3200},
    {"name": "Set shell to zsh", "action": "shell", "changed": false, "duration_ms": 45}
  ]
}
```

---

## Implementation

### Dependencies

Add `github.com/mark3labs/mcp-go` — a minimal Go MCP SDK that handles JSON-RPC framing,
tool registration, and stdio transport. Alternatively implement the minimal MCP
handshake and tool dispatch by hand (the protocol is simple enough at v1).

Prefer the hand-rolled approach to keep the dependency footprint small:

MCP stdio transport is:
1. Read JSON-RPC request from stdin (newline-delimited)
2. Dispatch to handler based on `method`
3. Write JSON-RPC response to stdout

For tool calls: `method = "tools/call"`, `params = {name, arguments}`.

### `internal/mcp/server.go` (new package)

```go
type Server struct {
    r io.Reader
    w io.Writer
}

func New(r io.Reader, w io.Writer) *Server

func (s *Server) Serve(ctx context.Context) error  // main loop

// Tool handlers registered via:
func (s *Server) RegisterTool(name string, h ToolHandler)
```

### `cmd/mcp.go` (new file)

```go
func mcpCommand() *cli.Command {
    return &cli.Command{
        Name:  "mcp",
        Usage: "start MCP server (stdio transport)",
        Action: func(ctx *cli.Context) error {
            srv := mcp.New(os.Stdin, os.Stdout)
            srv.RegisterTool("get_facts", mcp.HandleGetFacts)
            srv.RegisterTool("get_snapshot", mcp.HandleGetSnapshot)
            srv.RegisterTool("fact_query", mcp.HandleFactQuery)
            srv.RegisterTool("run_plan", mcp.HandleRunPlan)
            return srv.Serve(ctx.Context)
        },
    }
}
```

### MCP manifest

`mooncake mcp` must respond to `initialize` and `tools/list` requests per the MCP spec.
The `tools/list` response enumerates all five tools with their JSON Schema inputs.

---

## Claude Desktop Integration

Users add to `claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "mooncake": {
      "command": "mooncake",
      "args": ["mcp"]
    }
  }
}
```

Claude can then call `get_snapshot` before any provisioning task to understand the
machine state, and `run_plan` to apply changes with full structured feedback.

---

## Acceptance Criteria

1. `mooncake mcp` starts without error and waits for JSON-RPC input on stdin.
2. Responds to `initialize` with server name/version.
3. Responds to `tools/list` with all five tool definitions + JSON Schema.
4. `get_facts` returns valid JSON facts.
5. `get_snapshot` returns the same output as `mooncake snapshot`.
6. `fact_query` returns the same value as `mooncake facts --query <path>`.
7. `run_plan` runs the given config and returns a structured result.
8. Invalid tool call returns a JSON-RPC error response (not a crash).
9. The server is tested with at least one round-trip test per tool.
