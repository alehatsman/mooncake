# Spec 03: Agent JSONL Output Mode

## Problem

An AI agent invoking mooncake has to parse ANSI terminal output to understand
what happened. This is fragile and token-expensive. The event system already
emits rich structured events internally — we just need a subscriber that writes
them to stdout as JSONL.

## Goal

`mooncake run --output agent` streams one JSON object per line to stdout.
Zero ANSI codes. Errors go to stderr.

```jsonl
{"event":"run.started","ts":"2026-05-12T10:30:00Z","total_steps":79,"dry_run":false}
{"event":"step.started","ts":"...","step_id":"step-1","name":"Install neovim","action":"package"}
{"event":"step.completed","ts":"...","step_id":"step-1","name":"Install neovim","changed":true,"duration_ms":3240}
{"event":"step.skipped","ts":"...","step_id":"step-4","name":"Install OpenJDK (Debian)","reason":"when: apt_available"}
{"event":"step.failed","ts":"...","step_id":"step-7","name":"Install fzf","error":"exit code 1","duration_ms":120}
{"event":"run.completed","ts":"...","ok":61,"changed":12,"skipped":8,"failed":1,"duration_ms":274000,"success":false}
```

## Event schema

All events share: `event` (string), `ts` (ISO8601).

| event | extra fields |
|-------|-------------|
| `run.started` | `total_steps`, `dry_run`, `tags` |
| `step.started` | `step_id`, `name`, `action`, `level` |
| `step.completed` | `step_id`, `name`, `changed`, `duration_ms` |
| `step.skipped` | `step_id`, `name`, `reason` |
| `step.failed` | `step_id`, `name`, `error`, `duration_ms` |
| `run.completed` | `ok`, `changed`, `skipped`, `failed`, `duration_ms`, `success`, `error` |

Package-specific result data (installed/already_present lists) emitted as part
of `step.completed.result` when available — not required for v1.

## CLI interface

```
mooncake run --config main.yml --output agent
mooncake run --config main.yml --output agent --sudo-pass $PASS
```

`--output` flag (new name, replaces `--output-format` for the run command).
Values: `text` (default), `agent`. The existing `json` per-log-line format
can be deprecated — `agent` supersedes it.

## Implementation

New `AgentSubscriber` in `internal/logger/` implementing the same
`events.Subscriber` interface as `ConsoleSubscriber`.

- Receives events from the publisher
- Marshals each to JSON and writes to stdout with `\n`
- Uses `json.Marshal` on a flat struct per event type (not `event.Data` directly
  to control the schema)
- No buffering — flush immediately so agents see events in real time

Wire up in `cmd/mooncake.go` `run()` function: when `--output agent`,
instantiate `AgentSubscriber` instead of `ConsoleSubscriber`.

## Out of scope

- Streaming stdout/stderr lines of shell steps (can add later as `step.stdout` events)
- Binary/protobuf framing
- WebSocket or HTTP streaming
- MCP integration (that's spec-06)
