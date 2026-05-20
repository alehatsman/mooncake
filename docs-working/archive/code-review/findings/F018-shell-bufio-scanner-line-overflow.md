---
id: F018
title: shell.streamOutput uses default bufio.Scanner — commands emitting lines > 64 KB silently truncate
severity: bug
package: internal/actions/shell
file: internal/actions/shell/handler.go
lines: 396-435
status: done
fixed: 2026-05-16 — original fix raises the per-line cap via `scanner.Buffer(make([]byte, 64*1024), shellStreamMaxLineBytes)` at handler.go:377; subsequent commit `6565252d fix(shell): F038 — surface line-overflow truncation in result.Stdout/Stderr + a synthetic step.stderr event` made the truncation visible (was previously human-logger-only). F018 markers in-code at handler.go:358 + 419.
verified: 2026-05-17 — confirmed fixed on master @ 099ee336. Dedicated test file `internal/actions/shell/f018_long_line_test.go`; `TestShellHandler_LongLineWithinCap` passes (line longer than the default 64KB but within the new 1MB cap survives without truncation). The F038 follow-on tests cover the surfacing path for over-cap lines.
---

## ✅ Fixed

Three changes in `streamOutput`:

1. **Raised the scanner's max-token cap from 64 KB to 1 MB** via
   `scanner.Buffer(make([]byte, 64*1024), 1024*1024)`. A 1 MB cap is
   generous for human-readable output and small enough that a
   runaway command can't OOM the daemon. Binary blobs > 1 MB should
   be redirected to a file by the playbook.
2. **Check `scanner.Err()` after the loop**. On `ErrTooLong` (or any
   other non-EOF pipe error) the logger receives an `Errorf` so the
   operator sees the truncation signal — pre-fix the error was
   swallowed entirely and a `capture: true` step looked successful
   with empty/short stdout.
3. **Drain the rest of the pipe after Scanner gave up** —
   `io.Copy(io.Discard, pipe)`. Discovered during regression-test
   authoring: without this, the child process blocks on its write
   end when the kernel pipe buffer fills (PIPE_BUF is small) and
   `command.Wait()` hangs forever. Silent truncation would have
   become a process-leak in the fix. The 1-MB line-over-cap test
   timed out at 60 s until the drain was added.

### Regression tests

`internal/actions/shell/f018_long_line_test.go`:

- `TestShellHandler_LongLineWithinCap` — a 100 KB single line
  (`awk` BEGIN block) round-trips through `result.Stdout` intact.
  Pre-fix: captured stdout is the empty string (the first
  `Scan()` errors out on a 100 KB line, the rest of the stream is
  discarded). Stashing the fix and re-running reproduces this exactly.
- `TestShellHandler_LineOverCapLogsTruncation` — a 2 MB line
  exceeds the new cap; the test asserts the logger receives an
  `Errorf` about `stdout stream stopped early` and the process
  doesn't hang.

Both skip on Windows (test uses `awk`).

### Adjacent — total-output cap NOT addressed

The finding's "Adjacent: total-output cap" section (a 64 MB
`bytes.Buffer` ceiling for streams that emit GB of small lines) is
a separate concern; not bundled here. Worth a follow-up finding
since `yes | head -c 10G` still OOMs the daemon today.

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
