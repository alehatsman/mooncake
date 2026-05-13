# Notes: `mooncaked` HTTP + MCP Surface (Deferred Reference)

**Status:** Deferred reference — not an active spec.
**Epic:** E7 Mooncake Fleet
**Relates to:** Spec 18 (Mooncake Agent Daemon walking skeleton), Spec 10 (MCP
stdio server), and future Specs 19–25.

---

## What this doc is

A concrete sketch of the HTTP + MCP surface that `mooncaked` should eventually
present. It pins down endpoints, tool shapes, error codes, and the SSE event
shape so that:

- Spec 18 implementation has a target to converge on (not a contract to
  satisfy).
- Specs 19–25 can extend the surface without breaking earlier callers.
- LLM clients and the `mooncake fleet` CLI can be designed against a known
  shape.

## What this doc is *not*

- Not a spec. Not part of Spec 18's acceptance criteria.
- Not a stable API contract. The surface here may evolve during Spec 18
  implementation; this doc should be updated to match what ships, not the
  other way around.
- Not exhaustive — auth lifecycle, capability negotiation, and resource
  subscriptions are sketched only where they affect the v1 surface.

If during Spec 18 implementation a different shape proves better, change this
doc.

---

## HTTP surface (whole thing)

```
POST /mcp                    JSON-RPC 2.0 envelope in/out          auth required
GET  /mcp/events?id=<eid>    SSE stream for execution events       auth required (501 in v1)
GET  /healthz                Liveness: {"ok":true}                 no auth
GET  /version                Build info                            auth required
```

All tool calls flow through `POST /mcp`. SSE endpoint URL shape is reserved
in v1 (returns `501`) so Spec 19 can ship without breaking clients.

## Auth

```
Authorization: Bearer 0123abcd...      # constant-time compare against agent.toml token
```

- Source IP outside `[server].allow` CIDRs → `403`, log source IP. Checked
  before token (don't leak token-validity to off-LAN scanners).
- Missing / wrong token → `401`, generic body.
- `/healthz` skips both checks — returns `{"ok":true}` only. No version, no
  hostname.

## MCP lifecycle

Protocol revision **`2024-11-05`** (matching Spec 10's stdio dispatch so the
two transports share code).

```jsonc
// → POST /mcp
{"jsonrpc":"2.0","id":1,"method":"initialize",
 "params":{"protocolVersion":"2024-11-05","capabilities":{},
           "clientInfo":{"name":"mooncake","version":"0.1.0"}}}

// ← response
{"jsonrpc":"2.0","id":1,"result":{
  "protocolVersion":"2024-11-05",
  "capabilities":{"tools":{},"logging":{}},
  "serverInfo":{"name":"mooncaked","version":"0.1.0"}
}}
```

Then `notifications/initialized` (one-way), then `tools/list`, then
`tools/call`. JSON-RPC dispatch is reused verbatim from Spec 10; only the
transport changes.

## Tool catalog

Eight tools. Five reused from Spec 10 unchanged; three new for the daemon.

| Tool | Reads / Writes | Source |
|---|---|---|
| `get_facts` | read | Spec 10 |
| `get_snapshot` | read | Spec 10 |
| `fact_query` | read | Spec 10 |
| `check_plan` | read | Spec 10 (path-based; only useful via local stdio) |
| `run_plan` | write | Spec 10 (path-based; only useful via local stdio) |
| `check_yaml` | read | **new** |
| `apply_yaml` | write | **new** |
| `applied_plans` | read | **new** |

### `apply_yaml` — the load-bearing one

```jsonc
// Input schema
{
  "type": "object",
  "required": ["yaml"],
  "properties": {
    "yaml":      {"type": "string",  "description": "Full mooncake config YAML"},
    "vars":      {"type": "object",  "additionalProperties": true, "default": {}},
    "tags":      {"type": "array",   "items": {"type": "string"}, "default": []},
    "dry_run":   {"type": "boolean", "default": false}
  }
}

// Request
{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{
  "name":"apply_yaml",
  "arguments":{
    "yaml":"steps:\n  - print: hello\n  - package:\n      manager: pacman\n      names: [ripgrep]\n",
    "vars":{},
    "dry_run":false
  }
}}

// Response (success)
{"jsonrpc":"2.0","id":42,"result":{
  "content":[{"type":"text","text":"<human-readable summary>"}],
  "structuredContent": {
    "execution_id":"01HW8XQ7K3M9YBVT0WJ4VZE2RP",
    "plan_hash":"sha256:e3b0c442...",
    "started_at":"2026-05-12T19:14:02Z",
    "finished_at":"2026-05-12T19:14:07Z",
    "duration_ms":5012,
    "dry_run":false,
    "changed":1, "ok":2, "skipped":0, "failed":0,
    "steps":[
      {"index":0,"name":"hello","action":"print","status":"ok","duration_ms":1},
      {"index":1,"name":"package ripgrep","action":"package","status":"changed","duration_ms":4800}
    ]
  }
}}
```

`status` values: `ok` | `changed` | `skipped` | `failed`. For `dry_run:true`:
`ok` | `would_change` | `would_skip`.

### `check_yaml` — sugar for `apply_yaml` with `dry_run:true`

Kept as a separate tool (not just a parameter) for the same reason Spec 10
split `check_plan` from `run_plan`: when an LLM scans `tools/list`,
"read-only / safe" vs "may mutate the system" should be visible at the tool
name level. Implementation forwards to `apply_yaml` internals with
`dry_run=true`.

### `applied_plans`

```jsonc
// Input
{"limit": {"type":"integer","minimum":1,"maximum":1000,"default":50}}

// Output (structuredContent)
{"entries":[
  {"execution_id":"01HW8XQ7K3M9...","plan_hash":"sha256:e3b0...",
   "started_at":"...","finished_at":"...",
   "changed":1,"ok":2,"skipped":0,"failed":0,
   "caller":"192.168.0.42","dry_run":false},
  ...
]}
```

Tail-read the last N lines of `[server].history` — don't full-scan a growing
JSONL.

### Spec 10 tools — unchanged

`get_facts`, `get_snapshot`, `fact_query`, `check_plan`, `run_plan` — see
Spec 10 for shapes. `check_plan` / `run_plan` take filesystem paths and are
only useful via local stdio (or if the caller knows the daemon's local FS
layout). Remote callers should use `check_yaml` / `apply_yaml`.

## Error envelope

Standard JSON-RPC 2.0:

```jsonc
{"jsonrpc":"2.0","id":42,"error":{"code":-32602,"message":"missing required field: yaml"}}
```

Codes used by mooncaked:

| Code | Meaning |
|---|---|
| -32700 | Parse error (malformed JSON in request body) |
| -32600 | Invalid Request envelope |
| -32601 | Method not found / tool not registered |
| -32602 | Invalid params (schema validation failed on tool args) |
| -32603 | Internal error (panic, unexpected condition) |
| **-32001** | Plan compile failed (YAML invalid, preset not found, template error) |
| **-32002** | Plan execute failed — partial result in `data` field |
| **-32003** | Apply unauthorized — reserved for Spec 22 per-peer caps |
| -32004 … | Reserved for Specs 19–25 |

Error `-32002` body shape (for partial failures):

```jsonc
{"jsonrpc":"2.0","id":42,"error":{
  "code":-32002,
  "message":"step 3 failed: package install: exit 1",
  "data":{
    "execution_id":"01HW...","plan_hash":"...",
    "changed":1,"ok":2,"failed":1,"steps":[...]
  }
}}
```

Critical: a partially-applied plan must still return the structured result
so callers (and LLMs) can reason about what landed.

## SSE event stream

```
GET /mcp/events?id=01HW8XQ7K3M9YBVT0WJ4VZE2RP HTTP/1.1
Authorization: Bearer ...
Accept: text/event-stream
```

**v1 behavior** (Spec 18): returns `501 Not Implemented` with body
`{"reason":"streaming events deferred to spec 19"}`. URL shape reserved so
Spec 19 can ship without breaking clients.

**Target shape** (Spec 19, for reference):

```
event: step
data: {"index":3,"name":"Install neovim","action":"package","status":"running"}

event: step
data: {"index":3,"name":"Install neovim","action":"package","status":"changed","duration_ms":3200}

event: done
data: {"execution_id":"01HW...","changed":3,"ok":12,"failed":0}
```

## Worked example: `mooncake fleet apply hello.yml --to arch`

```
$ cat hello.yml
steps:
  - print: hello from {{ hostname }}

$ mooncake fleet apply hello.yml --to arch
```

Wire trace:

```
1. CLI loads ~/.mooncake/agent.toml → finds peer "arch" at arch.local:8765,
   token "0123abcd..."
2. CLI reads hello.yml from local disk → 47 bytes of YAML

3. POST http://arch.local:8765/mcp
   Authorization: Bearer 0123abcd...
   Content-Type: application/json
   {"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}

4. ← {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05",...}}

5. POST /mcp  {"jsonrpc":"2.0","method":"notifications/initialized","params":{}}

6. POST /mcp
   {"jsonrpc":"2.0","id":2,"method":"tools/call","params":{
     "name":"apply_yaml",
     "arguments":{"yaml":"steps:\n  - print: hello from {{ hostname }}\n",
                  "vars":{},"dry_run":false}
   }}

7. (daemon side)
   - auth middleware: 192.168.0.42 ∈ allow ✓, token matches ✓
   - config.ReadConfigBytesWithValidation(yaml, "<inline>") → ParsedConfig
   - plan.BuildPlanFromYAML(yaml, vars={}, baseDir="/var/lib/mooncake/work") → Plan
   - executor.Execute(plan) → result
   - history.Append({execution_id, plan_hash, caller="192.168.0.42", ...})

8. ← {"jsonrpc":"2.0","id":2,"result":{"structuredContent":{
       "execution_id":"01HW...","changed":0,"ok":1,"skipped":0,"failed":0,
       "steps":[{"index":0,"name":"print","action":"print","status":"ok","duration_ms":1}]
     }}}

9. CLI prints:
   arch  ok=1  changed=0  skipped=0  failed=0  (1.2s)
```

## Open questions (resolve when picking this up)

- **Connection model**: keep MCP session per `mooncake fleet …` invocation
  (initialize → call → close) or pool? Per-invocation is simpler for v1;
  pooling matters when fleet count > ~5.
- **`Mcp-Session-Id`** header: MCP HTTP transport spec defines optional
  session correlation. Probably not needed in v1 (one tool call per session).
- **Content negotiation**: only `application/json` (JSON-RPC) on `POST /mcp`
  in v1. Streamable HTTP transport's bidirectional SSE-on-POST mode is
  Spec 19 territory.
- **Schema version**: pin `2024-11-05` or follow upstream? Pin for v1 to
  match Spec 10; revisit when MCP spec next revs.
- **Tool naming**: stuck with Spec 10's snake_case flat namespace
  (`get_facts`) rather than dotted (`host.facts`). Worth revisiting only if
  the catalog grows past ~15 tools.
