# Spec 01: Run Recap Line

## Problem

The current `renderRunCompleted` prints a verbose multi-line block:
```
──────────────────────────────────────────────────
✓ Execution completed successfully

  Duration: 274000ms
  Total steps: 79
  ✓ Successful: 61
  ✗ Failed: 0
  ⊘ Skipped: 8
  Changed: 12
──────────────────────────────────────────────────
```
This is noisy for humans and wastes tokens for agents. The changed count is
also wrong — currently set to `statsExecuted` when success, not actual changed
tracking (bug noted separately).

## Goal

Replace the block with a single compact recap line, always printed last:

```
RECAP  ok=61  changed=12  skipped=8  failed=0  4m32s
```

On failure:
```
RECAP  ok=45  changed=8  skipped=8  failed=1  2m10s  ✗ Install fzf: exit code 1
```

## Behavior

- Always printed, even on failure
- Single line, no surrounding borders
- Fields: `ok`, `changed`, `skipped`, `failed`, duration
- Zero-value fields still printed (so format is stable and parseable)
- On failure: append `✗ <step_name>: <error>` inline
- Duration format: use human-friendly (`4m32s`, `47s`, `120ms`) not raw ms

## Implementation

`internal/logger/console_subscriber.go` — rewrite `renderRunCompleted`.

`RunCompletedData` already has all needed fields. Duration is `DurationMs`.

Fix changed tracking: currently `changedSteps = statsExecuted` when success
(executor.go ~line 940). Need to track `result.Changed == true` per step in
`ExecuteSteps` and increment a real `statsChanged` counter alongside
`statsExecuted`.

## Out of scope

- Colored output (can add later)
- Per-step timing in recap
- Machine-readable recap format (that's spec-03 agent mode)

## Open questions

- Should zero-value fields be suppressed? (`failed=0` vs omitting failed entirely)
  — keep them: stable format is easier to parse and avoids "did it fail silently?"
