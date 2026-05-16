---
id: F018
title: shell.streamOutput uses default bufio.Scanner — commands emitting lines > 64 KB silently truncate
severity: bug
package: internal/actions/shell
file: internal/actions/shell/handler.go
lines: 396-435
status: open
---

## What

```go
func (h *Handler) streamOutput(pipe io.Reader, buf *bytes.Buffer, ctx actions.Context, capture bool, stream string) {
    scanner := bufio.NewScanner(pipe)
    // ...
    for scanner.Scan() {
        line := scanner.Text()
        // ...
    }
}
```

`bufio.NewScanner` has a default max-token size of **64 KB**
(`bufio.MaxScanTokenSize`). A shell command that emits a single
line longer than 64 KB (binary blobs, large JSON without newlines,
some heredocs) will trigger:

- `scanner.Scan()` returns `false` early.
- `scanner.Err()` is `bufio.ErrTooLong` — **never checked here**.
- The remaining output is silently discarded.

The command itself **does not fail**: `executeAndCaptureOutput`
on line 392 calls `command.Wait()` after the WaitGroup, and the
caller sees a "successful" run with truncated stdout/stderr.

Concrete repro:

```yaml
- name: dump-big-blob
  shell:
    cmd: |
      python3 -c 'print("x"*70000)'
  as: out
```

`{{ out.stdout }}` is empty (or whatever fit before the first
discard), but `result.Failed == false` and no error is reported.

## Why it's a bug, not a smell

1. **Silent data loss on a successful exit code.** A user who
   captures shell output and feeds it to a downstream step will
   get unpredictable behavior. The whole point of `capture: true`
   is to keep the output reliable.
2. **The scanner.Err() check is missing entirely.** Even
   ignoring the line-length issue, errors from the pipe (e.g.
   `io.EOF`-not-actually-EOF on a connection reset) are
   swallowed.
3. **`failed_when: '"error" in stdout'`-style guards** can be
   evaluated against truncated output, giving false negatives
   for the very class of long-output commands they're often used
   for.

## Suggested fix

```go
func (h *Handler) streamOutput(pipe io.Reader, buf *bytes.Buffer, ctx actions.Context, capture bool, stream string) {
    scanner := bufio.NewScanner(pipe)
    // Allow up to 1 MB per line. Larger than default (64 KB),
    // small enough that a runaway command can't OOM the daemon.
    scanner.Buffer(make([]byte, 64*1024), 1024*1024)

    lineNum := 0
    // ... existing loop ...

    if err := scanner.Err(); err != nil {
        // Don't fail the command for a line-too-long; surface it
        // via stderr-on-result so the user sees the truncation.
        if ctx.GetLogger() != nil {
            ctx.GetLogger().Warnf("  shell %s stream stopped early: %v", stream, err)
        }
    }
}
```

Two parameters to think about:

- **Max line size.** 1 MB is generous for human-readable output.
  Binary blobs should probably be redirected to a file by the
  user, not captured. If a user is hitting 1 MB lines, they have
  a different problem.
- **Behavior on `ErrTooLong`.** Log-and-continue (above) is the
  least-surprising; alternatives are "fail the step" (loud, may
  break existing users) or "truncate-with-marker" (insert a
  `[...truncated]` line — bigger change).

### Adjacent: total-output cap

`bytes.Buffer` (line 410-411) grows unboundedly. A shell command
that prints GB of small lines (`yes | head -c 10G`) will OOM the
daemon. Reasonable cap: **64 MB** of captured output per stream,
matching the truncate-and-warn shape. After 64 MB, stop appending
to `buf` but keep draining the pipe (so the child doesn't block
on a full pipe).

```go
const maxCapturedBytes = 64 * 1024 * 1024 // 64 MB
// ...
if capture && buf.Len() < maxCapturedBytes {
    buf.WriteString(line)
    buf.WriteString("\n")
}
```

The event publisher path (line 423-432) still emits every line,
so the live SSE stream sees the full output even if the captured
buffer is bounded. That's the right asymmetry: live events are
ephemeral, captured output is materialized into `result.Stdout`
and persisted.

## Verification

- Add `TestShellHandler_LongLine` with a 100 KB output and assert
  the captured stdout contains the full line.
- Add `TestShellHandler_HugeOutput` with `yes` for 10 seconds
  and assert the captured stdout is bounded at 64 MB + the warn
  message fires.
- `go test -timeout 60s ./internal/actions/shell/...`

## References

- `bufio.ErrTooLong` documentation.
- `bufio.Scanner.Buffer()` — public API for raising the cap.
- Adjacent: similar pattern likely in `assert/handler.go` and
  other handlers that scan subprocess output; audit.
