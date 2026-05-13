# Spec 09: Structured Error Messages

**Epic:** E6 Agent-Native Interface (S6.5)  
**Effort:** S–M (3–5h)  
**Value:** High — agents can act on structured errors without parsing prose

---

## Problem

When a step fails today, mooncake emits an unstructured error string to the console.
An AI agent reading stdout/stderr has to parse prose to understand:
- Which step failed
- What the exit code was
- What was in stderr
- What to try next

This is fragile and burns tokens.

---

## Goal

Failed steps emit a structured JSON error object to **stderr**. Human display on stdout
is unchanged. Agents consuming the JSONL stream (`--output-format agent`) already get
`event: "step_failed"` — this spec extends that event with `stdout`/`stderr` capture and
adds a `hint` field inferred from common failure patterns.

---

## Structured Error Object

Emitted to stderr (as a single JSON line) on every step failure, regardless of
`--output-format`:

```json
{
  "event": "step_error",
  "ts": "2026-05-12T10:31:42Z",
  "step": "Install pyenv",
  "action": "shell",
  "exit_code": 1,
  "stdout": "",
  "stderr": "curl: command not found\n",
  "hint": "curl is not installed; try: package: {name: curl, state: present}",
  "suggested_step": "package:\n  name: curl\n  state: present"
}
```

Fields:
- `event` always `"step_error"`
- `step` — step name
- `action` — action type (`shell`, `package`, `file`, etc.)
- `exit_code` — integer; -1 if the process could not start
- `stdout`, `stderr` — captured output (truncated to 2000 chars each)
- `hint` — optional inferred hint (see hint rules below)
- `suggested_step` — optional inline YAML step that would fix the issue

---

## Hint Inference Rules

A small lookup table in `internal/errors/hints.go`:

| stderr pattern | hint | suggested_step |
|----------------|------|----------------|
| `command not found: <name>` | `<name> is not installed` | `package: {name: <name>, state: present}` |
| `curl: command not found` | curl not installed | `package: {name: curl, state: present}` |
| `permission denied` | insufficient permissions; try running with sudo | — |
| `No such file or directory: <path>` | path does not exist | — |
| `address already in use` | port already bound; check running processes | — |
| `EACCES` | permission denied on filesystem path | — |

Rules are checked in order, first match wins. No match → `hint` and `suggested_step`
are omitted.

---

## Implementation

### Stdout/stderr capture in shell action

Currently `shell` action may not capture output separately. Extend to always capture
both and attach to the failure event.

### `events.EventStepFailed` extension

```go
type EventStepFailed struct {
    StepID     string
    Name       string
    Action     string
    ExitCode   int
    Stdout     string
    Stderr     string
    DurationMs int64
    Err        error
}
```

### `internal/errors/hints.go` (new)

```go
type Hint struct {
    Text          string
    SuggestedStep string
}

func InferHint(stderr string) Hint
```

### `internal/logger/stderr_error_subscriber.go` (new)

Subscribes to `EventStepFailed`. On every failure, writes one `step_error` JSON line
to `os.Stderr`. Always active, regardless of output format.

```go
type StderrErrorSubscriber struct {
    w io.Writer // os.Stderr by default; injectable for tests
}
```

### `AgentSubscriber` update

Include `exit_code`, `stdout`, `stderr`, `hint`, `suggested_step` in the existing
`step_failed` event line (merges with new `EventStepFailed` fields).

---

## Acceptance Criteria

1. When a step fails, a `step_error` JSON object is written to stderr.
2. The object includes step name, action, exit code, stdout, stderr.
3. For known patterns, `hint` is populated with a human-readable suggestion.
4. For `command not found: <name>`, `suggested_step` contains the package YAML.
5. Stderr capture works for shell actions; package actions include package manager output.
6. Human display on stdout is unchanged.
7. `--output-format agent` includes `stdout`/`stderr`/`hint` fields in `step_failed` events.
8. Stderr output is valid JSON (parseable by `jq`).
