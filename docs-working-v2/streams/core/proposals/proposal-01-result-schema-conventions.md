# Proposal 01: Result schema conventions — make every action's output predictable

**Status:** Draft proposal
**Effort:** M (~5 days; cross-cuts ~30 handlers)
**Value:** High — agents and humans both consume action results.
Today each handler returns its own shape. Conventions exist
informally; codifying them lets agents parse without per-action
knowledge.

---

## Problem

Every action returns a result map, but the shapes diverge:

```json
// os.group
{"changed": true, "name": "testers", "operation": "create"}

// pkg.hold
{"changed": true, "manager": "apt", "holding": ["bash"], "unholding": null, "targets": ["bash"], "state": "held"}

// observe.cpu
{"changed": false, "as_of": "2026-05-15T...", "found": true, "value": {"cores": 32, "usage_pct": 4.34}}

// observe.process — note: `value:` wrapper + `error:` populated when not-found
{"changed": false, "found": false, "value": {"running": false}, "error": "no matching process", "failed": false}

// pkg.list — NO `value:` wrapper; fields at top level
{"changed": false, "count": 92, "manager": "apt", "packages": [...]}

// shell
{"changed": true, "rc": 0, "stdout": "...", "stderr": ""}

// file.write
{"changed": true}    // no path? no size?

// repo.tree
{"changed": false, "total_dirs": 1, "total_files": 0, "tree": {...}}

// text.line
{"changed": true, "operation": "replace", "path": "/work/conf.txt"}

// wait.http (timeout case)
{"changed": false, "failed": false, "success": false, "error": "...timeout...", "url": "...", "iterations": 2}
```

Findings citing the inconsistency:
- **#61** — `failed: false` populated with `error:` (observe.process,
  wait.http, os.mount, os.firewall): four different shapes
- **#70** — read.json wraps content in `.value`; pkg.list doesn't
- **#22** (now fixed) — `mooncake step` only surfaced shell stdout;
  typed result fields were stripped. Same shape disagreement at the
  step interface.

Result: an agent that wants to "use what changed" has to know each
action's shape. A human reading text output has to know which
fields exist per action. Documentation has to cover N shapes
instead of N + one convention.

## Proposal

Codify a **5-field common envelope** every action returns:

| Field | Type | Required | Semantics |
|---|---|---|---|
| `changed` | bool | yes | "Did this action mutate state on disk / on the host?" |
| `failed` | bool | yes | "Should this be counted as a failure?" (see proposal-06) |
| `operation` | enum: `create`/`update`/`delete`/`noop`/`query`/`reverted` | yes | One-word taxonomy of what happened |
| `target` | string | yes (when applicable) | The primary thing acted on (path, package, peer, etc.) |
| `value` | object | yes | Action-specific typed payload; always present, may be empty |

Free fields (`rc`, `stdout`, `stderr`, `duration_ms`, `as_of`, `error`)
remain as standard cross-cutting fields, but action-specific data
lives under `value`.

### Examples after conversion

```json
// shell
{
  "changed": true, "failed": false, "operation": "update",
  "target": "echo hi",
  "value": {"rc": 0, "stdout": "hi\n", "stderr": ""},
  "duration_ms": 1
}

// os.group create
{
  "changed": true, "failed": false, "operation": "create",
  "target": "testers",
  "value": {"name": "testers", "gid": 1001}
}

// os.group noop (already exists)
{
  "changed": false, "failed": false, "operation": "noop",
  "target": "testers",
  "value": {"name": "testers", "gid": 1001}
}

// observe.cpu (read-only)
{
  "changed": false, "failed": false, "operation": "query",
  "target": "<host>",
  "value": {"cores": 32, "usage_pct": 4.34, "load_1m": 3.5},
  "as_of": "2026-05-15T20:00:00Z"
}

// observe.process — "not running" is a valid observation, NOT an error
{
  "changed": false, "failed": false, "operation": "query",
  "target": "nonsense_proc",
  "value": {"running": false},
  "as_of": "2026-05-15T20:00:00Z"
  // NO `error:` field — query that returns "absent" is success
}

// wait.http (timeout)
{
  "changed": false, "failed": true, "operation": "query",
  "target": "http://localhost:9999/never",
  "value": {"last_status": 0, "iterations": 2, "elapsed_ms": 1001},
  "error": "wait.http timeout after 1s for ..."
}

// transaction (success)
{
  "changed": true, "failed": false, "operation": "update",
  "target": "transaction (3 children)",
  "value": {
    "children_committed": 3,
    "children_reverted": 0
  }
}

// transaction (rollback)
{
  "changed": false, "failed": true, "operation": "reverted",
  "target": "transaction (3 children)",
  "value": {
    "children_committed": 0,
    "children_reverted": 2,
    "failure_step": "step-0003",
    "failure_reason": "ENOTDIR /dev/null/cannot-exist"
  }
}
```

## Migration path

This is a contract change. Approach:

1. **v0.3.x**: handlers emit BOTH old fields AND new common envelope.
   Code reading either path keeps working. Docs flip to the new shape.
2. **v0.4.x**: remove the deprecated old fields. Single shape.
3. **v0.5.x**: agents and tooling assume the new shape.

The `step.completed.data.result` JSON keeps growing during v0.3.x but
that's fine — it's machine-consumed.

## Implementation pattern

```go
// internal/actions/handler/result.go
type Result struct {
    Changed   bool                   `json:"changed"`
    Failed    bool                   `json:"failed"`
    Operation Operation              `json:"operation"`
    Target    string                 `json:"target"`
    Value     map[string]interface{} `json:"value"`
    // Cross-cutting (optional):
    DurationMs int64  `json:"duration_ms,omitempty"`
    AsOf       string `json:"as_of,omitempty"`
    Error      string `json:"error,omitempty"`
    Stdout     string `json:"stdout,omitempty"`  // shell-family only
    Stderr     string `json:"stderr,omitempty"`
    RC         int    `json:"rc,omitempty"`
}

type Operation string
const (
    OpCreate   Operation = "create"
    OpUpdate   Operation = "update"
    OpDelete   Operation = "delete"
    OpNoop     Operation = "noop"
    OpQuery    Operation = "query"
    OpReverted Operation = "reverted"
)
```

Handlers fill `Result.Value` with the action's typed fields. The
common envelope is added by a wrapper after the handler returns.

## Receipts

From the audit:
- **#61** is the umbrella issue; this proposal resolves it.
- **#70** (top-level stringify) — `cfg.value.X` access works
  consistently if every action wraps in `value`.
- Every "operation:" we've seen in the wild (os.group, pkg.hold,
  pkg.repo, text.line, os.cron, os.sysctl) overlaps cleanly with
  the 6-value enum above. The work is mostly retrofit to the
  ~15 actions that don't yet emit one.

## What this doesn't address

- **Per-action property schemas** for `value:` — those should be
  generated by `mooncake schema generate` (already partially done).
  Adding documentation for each `value:` shape is the next step
  but separable.
- **Backward compat for external tooling** — anyone shelling out
  to `mooncake step` and parsing the raw result map has to adapt.
  The deprecation window above is the answer.

## Pairs with

- **Proposal 02 (recap counter discipline)** — the recap counters
  derive from `Result.Operation` and `Result.Failed`.
- **Proposal 06 (failed/error distinction)** — formalizes when
  observe-style "absent" is success vs failure.
- **DX proposal-04 (actions show)** — per-action `value:` shape
  belongs in `mooncake actions show <name>`.
- **Agent proposal-04 (typed diff)** — typed diffs read from
  `Result.Operation` + `Result.Target` + `Result.Value`.
