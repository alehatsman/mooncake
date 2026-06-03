---
id: agent-framework
status: draft
owners: [aleh]
covers:
  - internal/agent/**
---

# Agent Framework

## Intent

The agent framework drives an LLM to write mooncake config plans and feeds
those plans through the kernel's typed action funnel — never letting model
output reach the system as raw shell. It owns the iterate-plan-apply loop, the
provider-agnostic LLM client abstraction, and the turn/approval/cancel
lifecycle a driver (e.g. moongit, model id `mooncake-agent`, `mooncake agent
run`) controls over stdio.

## Behavior

- WHEN a run is given a goal, `RunLoop` iterates up to `MaxIterations`
  (default 5): build prompt from goal + machine snapshot + last-iteration
  feedback, ask the LLM for a plan, apply it, then feed the outcome forward.
- WHEN a run is given a `--plan`/`--stdin` file instead of a goal, `Run`
  executes that one plan through the same funnel without calling any LLM.
- WHERE model output enters the kernel, it passes through the typed funnel in
  order — strip markdown fences, `SanitizePlan`, `NormalizePlanBytes`,
  `WrapInTransaction`, `ReadConfigWithValidation` — and only a validated plan
  reaches `executor.Start`.
- WHILE selecting a provider, the client resolves first match of: explicit
  `--provider`, `MOONCAKE_AGENT_PROVIDER`, `claude` on `$PATH`,
  `CLAUDE_API_KEY`, `MOONCAKE_AGENT_ENDPOINT` (OpenAI-shape); an unknown
  provider is a hard error.
- IF a provider implements `StreamingClient`, the loop streams `planner.delta`
  events live; otherwise it falls back to buffered `GeneratePlan` and emits one
  synthetic delta, so the event contract is uniform across providers.
- WHEN not `--auto-apply`, each generated plan stops at a confirm gate before
  execution; an injected `Approver` (driver-channel approval) replaces the
  built-in stdin gate, and a missing TTY on the built-in path is a terminal
  error.
- WHILE a programmatic run reads NDJSON control messages on stdin,
  `stop`/`abort` cancel the run context (stopping at the next safe point with
  `StopCanceled`, rolling back an in-flight transaction), and
  `approve`/`reject`/`edit` answer a parked `plan.awaiting_approval` gate.
- WHEN the planner re-emits a byte-identical plan, fails the same step twice
  (`#71`), or makes no agent-attributable change (`#77`/`#87`), the loop stops
  early with `StopNoProgress` rather than burning iterations.
- WHEN `--style step`, an empty plan is the goal-reached signal
  (`StopStepDone`) and a plan with more than one step is a contract violation
  fed back to the next prompt.
- WHEN a run is custom-wired, an injected `Registry` extends the planner's
  action vocabulary and the executor's resolvable handlers with the consumer's
  own typed actions, and an injected `LLMClient` bypasses provider resolution.
- WHERE a run completes in JSON output mode, it terminates with a single
  `agent.completed` event whose `status` is `TerminalStatus()` — the worst
  outcome across all iterations, so a late no-op cannot mask an earlier
  failure.

## Non-goals

- The agent daemon (run lifecycle/store/HTTP) — `internal/agentd/**`.
- Mooncake's MCP server surface — `internal/mcp/**`.
- The executor, action handlers, and transaction rollback mechanics — the
  framework consumes those typed contracts, it does not define them.

## Checklist

- [x] `RunLoop` iterate-plan-apply loop with snapshot + feedback prompting
- [x] Single-shot `Run` (plan from file/stdin, no LLM call)
- [x] Typed funnel: sanitize → normalize → wrap-in-transaction → validate → execute
- [x] `Client` / `StreamingClient` abstraction + provider resolution chain
      (anthropic-cli, anthropic-http, openai-shape)
- [x] Streaming `planner.delta` with buffered fallback to one synthetic delta
- [x] Confirm gate: stdin, injected `Approver`, and `--auto-apply` bypass
- [x] Control channel: stop/abort cancel; approve/reject/edit answer the gate
- [x] No-progress guards: identical-replan, repeated-failure (#71), no-change
      streak (#77), inherited-dirt baseline (#87)
- [x] `--style step` empty-plan-done / multi-step-violation contract
- [x] Per-run `Policy` (#11) and injected `Registry` threaded to prompt + executor
- [x] Terminal `agent.completed` event with worst-outcome `status` (#64/#79)
