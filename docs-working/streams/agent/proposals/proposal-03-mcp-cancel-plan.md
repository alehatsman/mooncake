# Proposal 03: `cancel_plan` MCP tool — the agent's "stop, I changed my mind" primitive

**Status:** Draft proposal
**Effort:** XS (~half day; mostly composing existing pieces)
**Value:** High — the agent loop's ability to course-correct
requires a clean cancellation path. Without it, mid-run decisions
are aspirational.

---

## Problem

An agent submits `run_plan` and decides — based on a streamed
event, an external signal, or a user prompt — to stop the run
partway through.

Today: no way. The MCP `run_plan` call is one-shot. The agent
either lets it complete or kills the MCP connection (which
doesn't actually cancel the remote run).

This is the same hole the fleet proposal-02 closes for the
controller side. The MCP side has it too — and agents are arguably
worse-positioned than humans: they react to streaming events
(proposal-02) but have no lever to act on the decision.

## Proposal

A new MCP tool `cancel_plan`:

```jsonc
// Request
{"method": "tools/call", "params": {"name": "cancel_plan", "arguments": {
   "run_id": "01KRPK1126M8B63M63SKYQVCM5",
   "grace_seconds": 5,        // optional, default 5s
   "no_rollback": false       // optional, default false (do rollback)
}}}

// Response (sync acknowledgement)
{"id": <call-id>, "result": {"content": [{"text": "{
  \"ack\": true,
  \"run_id\": \"01KRPK1126M8B63M63SKYQVCM5\",
  \"cancellation_started\": \"2026-05-15T20:00:00Z\"
}"}]}}
```

Behavior (same shape as fleet proposal-02):

1. MCP server posts to the local `agentd`'s
   `/v1/runs/<id>/cancel` (or invokes the executor's cancel
   primitive if there's no agentd in this process — same code
   path either way).
2. The runner sends SIGINT to the current step's process group;
   waits `grace_seconds`; SIGKILL.
3. If the cancelled step was inside a `transaction:`, the
   transaction's LIFO rollback runs (unless `no_rollback: true`).
4. The run's record in `runs.jsonl` is updated to `status:
   cancelled`.
5. If streaming (proposal-02), the client gets a final
   `notifications/progress` with `type: "run.completed",
   data.status: "cancelled"`.

## Use cases

| Workflow | Cancel trigger |
|---|---|
| Agent watches `step.stdout` during a long-running `pkg upgrade`; user-facing UI prompts "abort?" | User clicks abort → MCP `cancel_plan` |
| Agent runs `run_plan` for a deploy; mid-deploy a `check_drift` signal indicates a problem on the target | Agent calls `cancel_plan` + `rollback` |
| Agent submits a plan; an upstream service signals an outage; abort | Cancel |
| Sandbox / quota: per-action quotas hit a limit | Cancel + report |

## Why this lives in agent

This is the agent-side mirror of fleet proposal-02. Both call into
the same agentd cancellation primitive. The MCP tool is a thin
wrapper around it.

Reusing the agentd cancel endpoint means:
- One code path for cancellation logic
- One audit trail format
- Cancellation that propagates correctly when an MCP `run_plan`
  is delegated to a fleet peer (run is on a peer, not local —
  the cancel just routes through the peer's agentd)

## Receipts

From the audit:
- Round 49: `mooncake apply` doesn't exit on SIGINT (#87). Same
  cancellation hole, exposed via different surface.
- Agent loop tests: I always submitted short playbooks because
  long ones offered no escape. With cancellation, long-running
  agent-driven flows become viable.

## API

The MCP tool list (after this + proposal-01 + proposal-02) becomes:

```
get_facts, get_snapshot, fact_query, get_metrics,
list_actions, describe_action, list_presets,
run_plan, check_plan, cancel_plan
```

A coherent verb set.

## Implementation

```go
// internal/mcp/tools/cancel_plan.go
func (s *Server) cancelPlan(args CancelPlanArgs) (*CancelPlanResult, error) {
    run := s.runs.Find(args.RunID)
    if run == nil {
        return nil, &MCPError{Code: -32602, Message: "run not found"}
    }
    if run.Terminal() {
        return nil, &MCPError{Code: -32000, Message: "run already terminal", Data: map[string]any{
            "status": run.Status,
        }}
    }
    run.Cancel(args.GraceSeconds, args.NoRollback)
    return &CancelPlanResult{
        Ack:                  true,
        RunID:                args.RunID,
        CancellationStarted:  time.Now().UTC().Format(time.RFC3339),
    }, nil
}
```

The cancellation propagates asynchronously. The MCP client gets the
final outcome via the streaming events of the original `run_plan`
call.

## What this doesn't address

- **Permission to cancel** — should arbitrary MCP clients be able
  to cancel any run? In v1: yes, anyone with MCP access has full
  control. In future versions, scoped tokens (read-only,
  cancel-only, full-control). Defer.
- **Cancelling a run on a different host** (fleet) — fleet
  proposal-02 covers that surface. Same primitive on agentd's
  side.
- **Replay after cancellation** — out of scope; defer to a
  `replay_run` tool.

## Pairs with

- **Proposal 02** (streaming events) — closes the loop:
  stream → decide → cancel
- **Fleet proposal-02** (fleet kill) — same cancellation
  primitive at agentd; this proposal is the MCP wrapper
- **Core proposal-02** (recap counter discipline) — `cancelled=N`
  in the recap reflects this
