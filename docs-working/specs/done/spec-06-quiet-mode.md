# Spec 06: Quiet Mode

**Epic:** E2 Compact Output Modes  
**Effort:** XS (< 1h)  
**Value:** Medium — essential for CI pipelines and scripting where only failures matter

---

## Problem

`--output-format` currently has `human` (verbose TUI) and `agent` (JSONL). There is no
mode that silences all output except errors and the final recap. CI pipelines that shell
out to mooncake get noisy logs full of ✓ lines they don't care about.

---

## Goal

`mooncake run --output-format quiet` prints nothing unless a step fails, then prints the
error, and always ends with the recap line.

---

## Behaviour

### Successful run
```
RECAP  changed=3  ok=58  skipped=8  failed=0  duration=4m12s
```
Nothing else.

### Run with one failure
```
FAIL  Install pyenv  exit code 1
  stderr: curl: command not found

RECAP  changed=2  ok=57  skipped=8  failed=1  duration=4m14s
```

### Rules
- No step-start lines, no ✓ lines, no skip lines.
- Each failed step prints `FAIL  <name>  <error>` followed by indented stderr (max 10 lines).
- Recap always printed, even on failure.
- Exit code mirrors current behaviour (non-zero if any step failed).

---

## Implementation

### New subscriber: `QuietSubscriber`

`internal/logger/quiet_subscriber.go`

```go
type QuietSubscriber struct {
    mu      sync.Mutex
    failures []failureEntry
    summary events.RunSummary   // populated on EventRunCompleted
}

type failureEntry struct {
    Name   string
    Error  string
    Stderr string
}
```

- `EventStepFailed` → append to `failures` slice
- `EventRunCompleted` → flush all stored failures to stdout, then print recap line

### Wire-up in `cmd/mooncake.go`
```go
const outputFormatQuiet = "quiet"

// in flag validation:
case outputFormatQuiet:
    publisher.Subscribe(logger.NewQuietSubscriber())
```

### No new flags needed
`--output-format quiet` is the complete interface.

---

## Acceptance Criteria

1. `mooncake run --output-format quiet` on a clean machine prints only the recap line.
2. A failed step causes its name + error to print before the recap.
3. `--output-format human` and `--output-format agent` are unaffected.
4. Exit code is non-zero when any step fails.
