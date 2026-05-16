# Proposal 02: Stream `run_plan` events incrementally — let agents watch instead of poll

**Status:** Draft proposal
**Effort:** S (~3 days)
**Value:** High — `run_plan` today is synchronous (return on
completion). Agents wanting to react to mid-run events have to poll
or wait. Streaming brings parity with the existing fleet exec UX.

---

## Problem

Today the agent loop with `run_plan` looks like:

```
agent → mcp.tools/call run_plan {config: "/work/cfg.yml"}
... (wait, possibly minutes) ...
mcp → agent: result with full summary
```

For long-running playbooks (a multi-step deploy, an apt-upgrade, a
fleet exec orchestrating across peers), the agent gets a single
result at the end. It can't:

- Show progress to a user ("step 3 of 8 in progress: pkg install
  nginx")
- React to a step failure mid-run (e.g., re-plan with a different
  strategy)
- Surface mid-run prompts (sudo password requests, confirmations)
- Cancel based on intermediate signals

Meanwhile `mooncake apply --output-format json` already streams
events to stdout. `fleet exec` multiplexes per-peer events live.
The infrastructure exists; just isn't exposed via MCP.

MCP 2024-11-05+ supports **incremental tool results** (server can
push interim updates while the call is in progress). This is the
right primitive.

## Proposal

`run_plan` (and `check_plan`) emit incremental updates as the run
progresses:

```jsonc
// Agent call
{"method": "tools/call", "params": {"name": "run_plan", "arguments": {
   "config": "/work/cfg.yml",
   "stream": true   // opt-in
}}}

// MCP server sends a series of incremental responses (notification-style):

// First, the plan is loaded:
{"method": "notifications/progress", "params": {
   "progressToken": "<run-ulid>",
   "type": "plan.loaded",
   "data": {"total_steps": 8, "input_files_hash": "..."}
}}

// Then each step:
{"method": "notifications/progress", "params": {
   "progressToken": "<run-ulid>",
   "type": "step.started",
   "data": {"step_id": "step-0001", "name": "install jq", "action": "pkg"}
}}

{"method": "notifications/progress", "params": {
   "progressToken": "<run-ulid>",
   "type": "step.stdout",
   "data": {"step_id": "step-0001", "stream": "stdout", "line": "..."}
}}

{"method": "notifications/progress", "params": {
   "progressToken": "<run-ulid>",
   "type": "step.completed",
   "data": {"step_id": "step-0001", "changed": true, "operation": "create", ...}
}}

// Finally, the synchronous call returns with the full summary:
{"id": <call-id>, "result": {"content": [{"text": "{...final summary...}"}]}}
```

When `stream: false` (default for back-compat), behaves exactly as
today: one terminal result at the end.

## Why MCP progress, not SSE

MCP has a built-in notifications channel. Agents already handle
`notifications/progress`. No need to invent a new transport. The
agent's MCP client gets per-step updates routed to the same place
where it handles other notifications.

(Alternative: expose an SSE-style endpoint on agentd directly. But
that's a different transport; we want to keep the agent
integration uniform on MCP.)

## Cancellation hook

While streaming, the agent can cancel by sending:

```jsonc
{"method": "tools/call", "params": {"name": "cancel_plan", "arguments": {
   "run_id": "<run-ulid>"
}}}
```

(See agent proposal-03 for `cancel_plan` details — same
primitive as fleet proposal-02.)

## Format consistency

The streamed events match the existing JSONL channel emitted by
`mooncake apply --output-format json`:

```
{"type": "run.started", ...}
{"type": "plan.loaded", ...}
{"type": "step.started", ...}
{"type": "step.stdout", ...}
{"type": "step.completed", ...}
{"type": "run.completed", ...}
```

The agent processes one shape regardless of whether the events come
from MCP or stdout-JSONL.

## Implementation

The mooncake binary's MCP server already runs the plan via the same
executor as `apply`. It just collects results before responding.
Implementing streaming is:

1. Set up an event subscriber on `run.events.Subscribe()`
2. On each event, encode and emit as MCP `notifications/progress`
3. After the run completes, emit the final result map (same as today)

The synchronous response shape stays exactly as today — meaning
agents that ignore `notifications/progress` still get a useful
answer.

## Receipts

From the audit:
- The JSON stream from `mooncake apply --output-format json`
  is the richest signal mooncake produces. It has step.stdout,
  per-step duration, per-step typed result. The MCP layer wraps
  the same data but only exposes the final summary.
- For a long-running run (e.g., apt-upgrade), 30+ seconds with
  no signal feels broken even when it's working.

## Edge cases

- **Server doesn't support streaming**: client passes `stream:
  true`, server doesn't recognize it. Today MCP servers ignore
  unknown args; result is one big response (back to today's
  behavior). No breakage.
- **Client doesn't support `notifications/progress`**: events are
  emitted but client ignores them. Final result still arrives.
  Same as if client didn't ask for streaming.
- **Connection drops mid-run**: agent reconnects, calls
  `get_run <run-id>` (see agent proposal-05) to get history.

## What this doesn't address

- **Multi-peer fan-out via MCP** — `fleet exec` over MCP would
  need its own tool; defer.
- **Resumable streaming** — if the client disconnects, the run
  continues but its in-flight events are lost. Defer to
  proposal-05's `get_run` for replay.
- **Throttling** — a 5000-step playbook produces 15000+ events
  per audit's perf test. Don't flood the client. Add an
  `update_throttle_ms` arg (default 100ms, batch within window).

## Pairs with

- **Proposal 03** (cancel_plan) — completes the controller-side
  flow: stream events, decide cancel, send cancel
- **Proposal 05** (history + replay) — fallback for dropped
  connections
- **Core proposal-01** (result schema) — events' `data` payloads
  reflect the standardized envelope
