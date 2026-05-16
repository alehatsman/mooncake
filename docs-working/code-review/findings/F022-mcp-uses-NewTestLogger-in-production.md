---
id: F022
title: mcp/tools.go uses logger.NewTestLogger() in production runConfig + HandleCheckPlan
severity: smell
package: internal/mcp
file: internal/mcp/tools.go
lines: 357, 457
status: open
---

## What

`internal/mcp/tools.go` (the MCP server's request handlers, shipped
to LLM clients) builds an `executor.InspectPlan` call with a
`*logger.TestLogger`:

```go
// Line 357 (runConfig, the run_plan tool path)
inspections, _ := executor.InspectPlan(planData, "", logger.NewTestLogger()) // non-fatal

// Line 457 (HandleCheckPlan, the check_plan tool path)
inspections, err := executor.InspectPlan(planData, "", logger.NewTestLogger())
```

`logger.NewTestLogger()` is declared as **"NewTestLogger creates a
new TestLogger for use in tests"** (`internal/logger/test_logger.go:24`).
It stores every log entry in a `Logs []LogEntry` slice under a
mutex — its purpose is so tests can `assert.Contains(logger.Logs, "x")`
on captured output.

In MCP's case:

- The slice grows unbounded every time `runConfig` or
  `HandleCheckPlan` is called.
- The slice is never read — there's no assertion against it; the
  logger is thrown away after `InspectPlan` returns.
- On a long-lived MCP server (the daemon's `/v1/mcp` endpoint),
  each call leaks every log line until the GC eventually
  reclaims the TestLogger.

It's a minor memory leak per call, but the deeper concern is the
**signal**: a production-path function reaches for a test
utility. Either the test helper isn't really test-only, or the
production code shouldn't be using it.

## Why it's a `smell` (not a `bug`)

The leak is bounded by the lifetime of `runConfig` / `HandleCheckPlan`
— the TestLogger goes out of scope when the function returns. The
slice gets GC'd. No persistent leak.

But: if a future refactor decides to surface InspectPlan logs
back to the MCP caller (a natural extension), reaching for
NewTestLogger and reading `.Logs` could become production behavior
— and that's exactly the wrong direction.

## Suggested fix

Either:

**Option A — use the appropriate production logger.**

`InspectPlan` takes a `logger.Logger` interface. Wire in either:

- A `NewQuietLogger()` (if InspectPlan logs are noise for MCP).
  No such constructor exists today; would need to add one.
- A `NewLogger(logger.ErrorLevel)` — production logger gated to
  error-only output. Lossy on debug logs but quiet on the happy
  path.

Recommended: add `logger.NewDiscardLogger()` that implements
`Logger` with `Infof`/`Debugf` no-ops. Drop-in replacement for
the "I don't want logs" use case. Lives next to NewTestLogger /
NewLogger.

**Option B — make `InspectPlan` accept a `nil` logger** and
internally skip logging if nil. Smaller blast radius but adds a
nil-check to every call site inside InspectPlan.

Option A is the cleaner architectural fix: the MCP handler asks
for "no logging," and the logger package has the right primitive.

## Verification

After fix:
- `grep -rn 'NewTestLogger' internal/` outside `*_test.go` → no
  hits.
- MCP tests for `runConfig` / `HandleCheckPlan` still pass.

## Adjacent observation

The `// non-fatal` comment on line 357 implies that errors from
`InspectPlan` are deliberately ignored. The variable assigned is
the `_` for `err`. Together with the discarded logger, the call
becomes "best-effort prediction; eat any error." That's fine, but
worth a one-line comment explaining the philosophy:

> InspectPlan errors are non-fatal because the predicted
> diff/cost is a UX nicety; if it fails we'd rather the apply
> proceed than block on the prediction.

## References

- `internal/logger/test_logger.go:24` — "for use in tests"
  docstring.
- `internal/logger/console_logger.go:20` — `NewLogger(int)` is
  the production constructor.
