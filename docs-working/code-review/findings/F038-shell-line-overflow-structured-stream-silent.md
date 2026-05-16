---
id: F038
title: shell.streamOutput surfaces ErrTooLong only via the human logger — structured event stream stays silent above the 1 MB cap
severity: bug
package: internal/actions/shell
file: internal/actions/shell/handler.go
lines: 454-465
status: open
verified: 2026-05-16 on master @ 49930fd (post-F018 merge)
---

## What

F018 raised the per-line scanner cap from 64 KB to 1 MB and added a
`scanner.Err()` check so a too-long line is logged before the
goroutine returns:

```go
if err := scanner.Err(); err != nil {
    if log := ctx.GetLogger(); log != nil {
        log.Errorf("  shell %s stream stopped early (output truncated): %v", stream, err)
    }
    _, _ = io.Copy(io.Discard, pipe)
}
```

That fix is correct for the **64 KB to 1 MB band** — lines under
1 MB now capture cleanly (verified: 70 KB and 500 KB captures both
round-trip in full).

But for lines **over 1 MB**, the truncation is surfaced **only via
the human-facing logger**. Programmatic consumers see no signal:

- `result.Stdout` is empty (or short).
- `result.Stderr` is unchanged — the truncation message goes to
  the logger, not appended here.
- `result.Failed == false`.
- `result.Rc == 0`.
- The `step.completed` event payload carries the same silent
  values — no `truncated: true` flag, no marker line in
  `stdout`, no error-message field set.

## Verified repro (post-F018, master @ 49930fd)

```yaml
# /tmp/f018-overflow.yml
steps:
  - name: 500KB line under 1MB cap should fit
    shell:
      cmd: 'python3 -c "import sys; sys.stdout.write(\"a\" * 500000 + chr(10))"'
    as: medium_out
  - name: 1.5MB line over 1MB cap should overflow
    shell:
      cmd: 'python3 -c "import sys; sys.stdout.write(\"b\" * 1500000 + chr(10))"'
    as: huge_out
  - name: report
    shell:
      cmd: "echo medium_len={{ medium_out.stdout | length }} huge_len={{ huge_out.stdout | length }}"
```

```
$ mooncake apply -c /tmp/f018-overflow.yml --output-format json | jq -c 'select(.type=="step.completed") | {name: .data.name, rc: .data.result.rc, stdout_len: (.data.result.stdout | length), stderr_len: (.data.result.stderr | length)}'
{"name":"500KB line under 1MB cap should fit","rc":0,"stdout_len":500001,"stderr_len":0}
{"name":"1.5MB line over 1MB cap should overflow","rc":0,"stdout_len":0,"stderr_len":0}     <- silent
{"name":"report","rc":0,"stdout_len":29,"stderr_len":0}
```

```
$ mooncake apply -c /tmp/f018-overflow.yml 2>&1 | grep truncated
  shell stdout stream stopped early (output truncated): bufio.Scanner: token too long
```

The text-mode user sees the warning. The `--output-format json`
consumer sees a successful step with empty stdout — same data
loss as pre-F018, just with a higher threshold.

## Why it's a bug

Same shape as F023 (package handler structured/string drift):
two output channels carry contradictory information about the
same step. The fix moved the human channel's signal from
"nothing" to "warning logged"; the structured channel still has
the pre-F018 problem.

The F018 finding itself called this out indirectly — the
"Suggested fix" snippet wrote:

> Don't fail the command for a line-too-long; surface it via
> **stderr-on-result** so the user sees the truncation.

The implementation surfaced via *logger*, not *stderr-on-result*.
For programmatic consumers (agentd → SSE → controller, MCP tool
results, fleet exec aggregator), only stderr-on-result is
visible.

Concrete consumer breakage:

- **Fleet apply** aggregator (`internal/fleet/exec`) reads
  `step.failed.exit_code` and the `result` payload. A 2 MB
  diagnostic line from `kubectl logs --since=1h` would show as
  empty stdout, rc:0 across the cluster — no operator signal.
- **MCP** `run_config` tool returns the run summary verbatim.
  An agent calling mooncake to apply a config that emits a
  large JSON document would receive empty captured stdout and
  re-issue the call thinking the previous one didn't run.

## Fix sketch

Smallest delta — surface the truncation in `result.Stderr` so the
structured channel matches the human channel:

```go
if err := scanner.Err(); err != nil {
    msg := fmt.Sprintf("mooncake: %s stream truncated (limit %d bytes per line): %v\n",
        stream, shellStreamMaxLineBytes, err)
    if log := ctx.GetLogger(); log != nil {
        log.Errorf("  shell %s stream stopped early (output truncated): %v", stream, err)
    }
    // Surface to programmatic consumers via the buffer that
    // becomes result.Stderr / result.Stdout. The trailing
    // newline keeps it grep-friendly for `... in stderr` guards.
    if capture {
        buf.WriteString(msg)
    }
    _, _ = io.Copy(io.Discard, pipe)
}
```

Belt-and-suspenders option: also publish a synthetic
`events.EventStepStderr` line via `publisher.Publish`, so SSE
subscribers see the message live (without it, they only learn
on `step.completed`).

Open question: should `result.Failed` flip to `true` on
truncation? Tradeoffs:

- **Yes** — strict; treats truncation as a real error, surfaces
  loudly. But it changes existing behavior (a previously-passing
  step now fails) and could break playbooks that emit huge
  diagnostic blobs as a harmless side effect.
- **No, but flag it** — add a `Truncated bool` field to
  `executor.Result` and a `truncated` field on
  `events.StepOutputData` / `step.completed`. Consumers opt in.

Recommendation: **No-failure + stderr-on-result** (this finding's
fix sketch above) covers most consumers without behavior change.
The `Truncated bool` field is a separate spec-level discussion.

## Regression test

```go
// internal/actions/shell/handler_test.go
func TestStreamOutput_OverflowAppearsInCapturedStderr(t *testing.T) {
    // command that emits a 1.5 MB line on stdout
    // assert: result.Stdout is empty (or truncated)
    // assert: result.Stderr contains "stream truncated" substring
    // assert: result.Rc == 0
}
```

Plus a JSON-output test in `cmd/cmd_test.go` covering the same
repro — verify the `step.completed` payload carries the
truncation message.

## References

- `internal/actions/shell/handler.go:413` —
  `shellStreamMaxLineBytes = 1024 * 1024`.
- `internal/actions/shell/handler.go:454-465` — the overflow
  branch.
- F018 — landed in `21a71f5`; this is the structured-stream
  follow-up the finding's "Suggested fix" actually asked for.
- F019/F025 — same "right fix, wrong call site" shape: the human
  surface gets the signal, the programmatic surface doesn't.
- F023 — same "structured/string drift" theme already tracked in
  TODO.md cross-cutting section.
