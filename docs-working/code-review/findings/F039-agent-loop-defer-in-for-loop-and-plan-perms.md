---
id: F039
title: agent.RunLoop defers tmpfile cleanup inside the for-body (resource leak per iteration); SavePlan writes 0644
severity: smell
package: internal/pilot
file: internal/pilot/loop.go
lines: 109-115, 214-223
status: partial
verified: 2026-05-16 — confirmed real on master @ b48a11e. pilot/loop.go:109-115 (renamed from agent/loop.go since the finding was filed) creates tmpfile inside for-body with defer-in-loop (cleanup pushed onto goroutine defer stack, tempfiles stay on disk through RunLoop). pilot/loop.go:218 SavePlan writes 0644 — world-readable for files that may contain resolved secret values
resolved: 2026-05-16 — F039(c) + F039(d) shipped: `SavePlan` now `os.MkdirAll(dir, 0o700)` + `os.WriteFile(filename, body, 0o600)` and returns `(string, error)` instead of swallowing failures into an empty string. Caller in `RunLoop` logs the error via `log.Errorf` so failed-to-persist iterations are no longer silent. The 0700/0600 perms match the rest of mooncake's state-dir convention (`internal/agentd/store.go`); plan files can contain resolved `!secret` values (post-F037 the planner expands markers BEFORE serialization), so 0644 was a real leak on shared hosts. Regression tests in `loop_test.go`: `TestSavePlan_FilePerms` (0600/0700), `TestSavePlan_CreatesIterationsDir` (the dir is now actually MkdirAll'd — pre-fix this was a latent bug: `os.WriteFile` would fail silently if the dir didn't exist), `TestSavePlan_ReturnsErrorOnFailure` (occupied-path triggers an error mentioning "create iterations dir"). Out of scope (left for separate fix): F039(a) defer-in-loop extraction — needs an iteration-body helper function refactor; the resource accumulation is bounded by MaxIterations today so this is non-urgent. F039(b) variable-capture is a non-issue under Go 1.22+ semantics.
---

## What

### (a) Defer inside the for loop body

`RunLoop` creates a per-iteration tempfile for the LLM-generated
plan, then defers cleanup INSIDE the loop body:

```go
for i := 1; i <= opts.MaxIterations; i++ {
    // ... build prompt, call LLM, sanitize ...

    tmpFile, err := os.CreateTemp("", "mooncake-plan-*.yml")
    if err != nil {
        return nil, fmt.Errorf("failed to create temp file: %w", err)
    }
    defer func() {
        _ = os.Remove(tmpFile.Name())
    }()

    // ... write, validate, executor.Start ...
}
```

Two consequences:

1. **Deferred calls accumulate.** Each iteration's deferred
   `os.Remove` is appended to the goroutine's defer stack and
   runs only when `RunLoop` returns. For `MaxIterations=5`
   that's 5 deferred functions; if a future use case raises
   the limit (e.g. background agent with 1000 iterations),
   that's 1000 deferred functions consuming memory until exit.
2. **Tempfiles stay on disk through the run.** Iteration 1
   creates `/tmp/mooncake-plan-12345.yml`; even after
   `executor.Start` returns and we move on to iteration 2's
   plan, the iteration-1 file remains on disk until `RunLoop`
   returns. For a long-running agent loop (or a crash), that's
   N orphan plan files in `/tmp`.

### (b) Variable capture

The deferred closure references `tmpFile` — which is the
**inner** loop variable that gets reassigned next iteration.
With Go 1.22+ loop-variable-per-iteration semantics, this is
actually safe; pre-1.22 it would have been a classic captured-
variable bug. Either way, the read-on-defer happens at function
exit which works correctly with the new semantics.

Worth a comment if the intent is "rely on the per-iteration
variable" rather than the silent-fix from the language change.

### (c) SavePlan writes 0644

```go
// SavePlan, line 207:
if err := os.WriteFile(filename, planBytes, 0644); err != nil { // #nosec G306 -- standard file permissions
    return ""
}
```

The `# nosec` claims "standard file permissions" but `.mooncake/
iterations/00001.plan.yml` may contain:

- The operator's `goal` (the prompt to the LLM; may contain
  task-specific context, paths, names).
- The LLM's generated plan (may reference secrets / hosts /
  tokens that the operator considered transient).

`0644` makes plan files world-readable. On a single-user laptop
this is fine; on a shared system (CI runners, dev servers with
multiple users, container hosts), the plan history is exposed
to every user on the box.

The state-dir convention elsewhere in mooncake is 0700/0600:

- `internal/agentd/store.go:88,151,170,328` — runs dir 0700,
  result.json 0600.
- `internal/agentd/files_handler.go:188` — synced files dir
  0700.

Plan iterations should match: 0600 for the file, 0700 for the
parent dir.

### (d) SavePlan returns "" on error without logging

```go
if err := os.WriteFile(filename, planBytes, 0644); err != nil {
    return ""
}
return filename
```

The error is dropped. The caller (loop.go:144) checks `if
planPath != ""` and skips adding the artifact entry. Operator
sees: iteration log lists no plan artifact, no error message.
A subtle "I tried to save your plan and failed" → silent loss.

## Why it's a smell (not a bug)

(a) and (b): correct under Go 1.22+ semantics; just non-idiomatic
and accumulates resource handles. Pre-Go-1.22 it would have been
a correctness bug.

(c): security hygiene, not exploitation. Until someone runs
mooncake-agent on a shared box.

(d): UX failure, not data corruption — the plan still ran (the
file was tempfile-written earlier in line 99-110 and was
written/closed successfully). Only the persisted artifact is
missing.

## Suggested fix

### (a) Move the cleanup INTO the iteration body

Replace the deferred Remove with a per-iteration helper function
that owns the tempfile lifecycle:

```go
for i := 1; i <= opts.MaxIterations; i++ {
    // ... build prompt, call LLM, sanitize ...

    if err := runOneIteration(...); err != nil {
        // ... handle errors ...
    }
}

// In runOneIteration (separate function):
func runOneIteration(...) error {
    tmpFile, err := os.CreateTemp("", "mooncake-plan-*.yml")
    if err != nil { ... }
    defer os.Remove(tmpFile.Name())   // ← scoped to this function
    // ... rest of per-iteration work ...
}
```

The defer now scopes to the helper function and fires at the
end of each iteration. Plus the function extraction makes the
loop body readable (~100 lines down to a single call site).

### (c) SavePlan + parent dir use 0600/0700

```go
func SavePlan(repoRoot string, iterNum int, planBytes []byte) string {
    dir := filepath.Join(repoRoot, ".mooncake", "iterations")
    if err := os.MkdirAll(dir, 0o700); err != nil {
        return ""
    }
    filename := filepath.Join(dir, fmt.Sprintf("%05d.plan.yml", iterNum))
    if err := os.WriteFile(filename, planBytes, 0o600); err != nil {
        return ""
    }
    return filename
}
```

### (d) Surface SavePlan errors

```go
func SavePlan(repoRoot string, iterNum int, planBytes []byte) (string, error) {
    // ... build dir + filename ...
    if err := os.WriteFile(filename, planBytes, 0o600); err != nil {
        return "", fmt.Errorf("save plan: %w", err)
    }
    return filename, nil
}

// Caller (loop.go:144):
planPath, savErr := savePlan(opts.RepoRoot, iterNum, planBytes)
if savErr != nil {
    ctx.GetLogger().Warnf("failed to save plan %d: %v", iterNum, savErr)
}
```

The plan being unsaveable shouldn't fail the iteration (the
plan already ran), but it should be loud.

## Adjacent — context.Background() in agent loop

`executor.Start(context.Background(), ...)` at line 135. F016
family. The agent-loop caller has no Ctrl-C-cancels-current-
iteration path. Out of scope for F039; flag for the same audit
that F016 closed for the daemon side.

## Verification

- Add `TestRunLoop_RemovesTempfileBetweenIterations`: run with
  MaxIterations=3, check `/tmp/mooncake-plan-*` count between
  iterations — should drop to 0 after each iteration, not
  accumulate to 3.
- `chmod` check on a plan file after a real run: `stat -c
  %a .mooncake/iterations/00001.plan.yml` → `600` post-fix.
- `go test ./internal/agent/...`

## References

- Go 1.22 release notes — loop-variable-per-iteration change
  that retroactively fixes the variable-capture risk here.
- `internal/agentd/store.go` — established 0700/0600 convention
  for mooncake state files.
