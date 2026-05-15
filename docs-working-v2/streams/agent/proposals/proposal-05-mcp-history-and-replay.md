# Proposal 05: MCP run history — `list_runs`, `get_run`, `replay_run`

**Status:** Draft proposal
**Effort:** S (~2 days)
**Value:** High for agent loops — knowing "what did I do last time?"
is the foundation of stateful behavior. Today the agent has no
canonical way to ask the server.

---

## Problem

Agent loops are stateful. A reasonable workflow:

```
- Run plan A
- Observe outcome
- Decide next action based on the outcome
- Run plan B
- ...
```

Today, mid-loop, the agent can't ask the server "what happened
last time?" without remembering by some external channel. The
mooncake binary keeps `runs.jsonl` (and per-run artifact bundles
when `--artifacts-dir` is set), but these aren't accessible via
MCP.

Three concrete gaps:

1. **No `list_runs`**: the agent can't enumerate past runs.
2. **No `get_run`**: the agent can't fetch one past run's details
   (events, result, duration, recap).
3. **No `replay_run`**: the agent can't say "run that same plan
   again against the saved plan-time facts" (which `mooncake
   apply --from-plan` does locally).

For long-running loops where the agent disconnects and reconnects
(MCP transport stutters, claude desktop restarts, a CI agent goes
back to its queue), reconstructing context requires reading
files via `mooncake apply -c <file>`-style tools — fragile.

## Proposal

Three new MCP tools:

### `list_runs`

```jsonc
// Request
{"method": "tools/call", "params": {"name": "list_runs", "arguments": {
   "limit": 10,
   "status": "success",   // optional filter: running | success | failed | cancelled
   "since": "1h"          // optional: only runs newer than this
}}}

// Response: newest-first array
{
  "runs": [
    {
      "run_id": "01KRPK1126M8B63M63SKYQVCM5",
      "ts": "2026-05-15T20:00:00Z",
      "config": "/work/cfg.yml",
      "status": "success",
      "summary": {
        "ok": 2, "changed": 5, "skipped": 0, "failed": 0,
        "duration_ms": 1200
      }
    },
    ...
  ],
  "total": 47   // total in history; we returned `limit`
}
```

Same data as `mooncake history list --format json`. Exposed via MCP.

### `get_run`

```jsonc
// Request
{"method": "tools/call", "params": {"name": "get_run", "arguments": {
   "run_id": "01KRPK1126M8B63M63SKYQVCM5",
   "include_events": false,    // default false; events can be large
   "include_facts": false      // default false; facts snapshot is verbose
}}}

// Response: full run record
{
  "run_id": "...",
  "ts": "...",
  "config": "/work/cfg.yml",
  "input_files_hash": "abc...",
  "status": "success",
  "summary": {...},
  "steps": [
    {
      "step_id": "step-0001",
      "name": "...",
      "action": "...",
      "changed": true,
      "operation": "create",
      "target": "/etc/nginx/conf.d/site.conf",
      "duration_ms": 12,
      "value": {...}     // per core proposal-01 result schema
    },
    ...
  ],
  "events": null      // populated only if include_events: true
}
```

### `replay_run`

```jsonc
// Request
{"method": "tools/call", "params": {"name": "replay_run", "arguments": {
   "run_id": "01KRPK...",
   "allow_stale": false,
   "dry_run": false
}}}

// Behavior:
// 1. Looks up the run's saved plan (if --artifacts-dir was set on the original)
// 2. Or recompiles from config + input_files_hash check (#10 stale-plan rejection)
// 3. Submits replay; streams events back per proposal-02
```

Mirrors `mooncake apply --from-plan <plan.json>` at the MCP layer.

## Use cases

| Workflow | Tool(s) |
|---|---|
| Agent reconnects after disconnect, wants to see what happened | `list_runs` → `get_run` of the latest |
| Agent committed N changes, wants to recap them for a human | `list_runs --since 1h` |
| Agent diagnoses a failure: "what step failed in run 01KRPK...?" | `get_run --run-id 01KRPK...` |
| User: "redo what you did 30 min ago, same plan, same hosts" | `list_runs --since 30m`, pick the one, `replay_run` |
| Agent demonstrates idempotency: "show me that this re-applies cleanly" | `replay_run` of the latest |

## Data sources

`list_runs` and `get_run`:
- `~/.mooncake/runs.jsonl` for the summary records
- `~/.mooncake/runs/<run-id>/` for full event bundles (if
  `--artifacts-dir` was set at the time)

`replay_run`:
- If artifact bundle has `plan.json`: use it directly (audit-grade
  replay)
- Else: recompile from `config` + check `input_files_hash` — if
  changed, error per #10's stale-plan rejection (the agent must
  pass `allow_stale: true` to override)

## Receipts

- `mooncake history list --format json` exists. Surfaces the
  records cleanly. Just isn't exposed via MCP.
- `mooncake apply --from-plan` exists. Same mechanism.
- The agent can shell out via `run_plan {"config": <path>,
  "dry_run": ...}` today, but that doesn't pick up the saved
  plan.json with its plan-time facts.

## API

After this proposal + proposals 01–04, the MCP tool list:

```
get_facts, get_snapshot, fact_query, get_metrics,
list_actions, describe_action, list_presets,
check_plan, diff_plan, run_plan, cancel_plan,
list_runs, get_run, replay_run
```

Cleanly grouped: state, discovery, plan/apply, history.

## Implementation

`list_runs`:
```go
func (s *Server) listRuns(args ListRunsArgs) (*ListRunsResult, error) {
    runs, err := history.List(args.Since, args.Status, args.Limit)
    if err != nil { return nil, err }
    return &ListRunsResult{Runs: runs, Total: history.Count()}, nil
}
```

`get_run`:
```go
func (s *Server) getRun(args GetRunArgs) (*GetRunResult, error) {
    run, err := history.Find(args.RunID)
    if err != nil { return nil, err }
    if args.IncludeEvents {
        run.Events = history.LoadEvents(args.RunID)
    }
    return &GetRunResult{Run: run}, nil
}
```

`replay_run`: composes existing `--from-plan` flow.

## What this doesn't address

- **Cross-host run history** — `mooncake history` is per-host.
  For fleet replay, the controller-side history is incomplete (it
  only sees runs it dispatched). Defer; fleet stream might want a
  `fleet history --all` proposal.
- **Run search by tag/content** — "find runs that touched
  /etc/foo.conf" — useful but expensive (full-text search on
  events). Defer.
- **Run pinning / saving** — agents might want to bookmark a
  run as "the known-good baseline". Defer.

## Pairs with

- **Proposal 02** (streaming events) — `replay_run` emits
  events the same way
- **Core proposal-01** (result schema) — `get_run.steps[].result`
  follows the standardized envelope
- **DX proposal-02** (output middle ground) — `get_run` output
  can be rendered by the same formatter
