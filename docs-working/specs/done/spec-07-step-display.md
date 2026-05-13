# Spec 07: Step Display — Timing and Changed/OK Distinction

**Epic:** E1 Observable Runs (S1.3 + S1.5)  
**Effort:** S (2–4h)  
**Value:** Medium-High — reduces noise while highlighting what actually changed

---

## Problem

Two related display gaps in human output mode:

1. Every step shows the same ✓ symbol regardless of whether it changed anything or was
   already satisfied. Hard to spot what actually happened.

2. No duration shown — a step that took 47s looks identical to one that took 10ms.

---

## Goal

Distinguish changed vs ok steps visually. Show duration on slow steps (>2s). No change
to agent/quiet output formats.

---

## Symbol Set

| Symbol | Meaning |
|--------|---------|
| `✓` | Ran, nothing needed (idempotent check passed) |
| `~` | Ran and changed something |
| `✗` | Failed |
| `-` | Skipped (when/creates condition) |

Current code uses `✓` for both "ran+changed" and "ran+ok". This spec adds `~` for the
changed case and `-` for skip (currently a word).

---

## Timing Display

Only show `[<duration>]` when step duration ≥ 2s. Hidden otherwise to reduce noise.

```
✓  Check SSH config
~  Install neovim  [8s]
✓  Set shell to zsh
~  Update pacman mirrors  [47s]
-  Install OpenJDK  [when: apt_available]
✗  Install pyenv  [12s]  exit code 1
```

---

## Implementation

### `EventStepCompleted` already carries `Changed bool` and `DurationMs int64`

The data is already in the event — this is purely a display change in `ConsoleSubscriber`.

### Changes to `internal/logger/console_subscriber.go`

```go
func (c *ConsoleSubscriber) handleStepCompleted(e events.EventStepCompleted) {
    sym := "✓"
    if e.Changed {
        sym = "~"
    }
    line := fmt.Sprintf("%s  %s", sym, e.Name)
    if e.DurationMs >= 2000 {
        line += fmt.Sprintf("  [%s]", fmtDuration(e.DurationMs))
    }
    c.println(line)
}

func (c *ConsoleSubscriber) handleStepSkipped(e events.EventStepSkipped) {
    c.println(fmt.Sprintf("-  %s  [%s]", e.Name, e.Reason))
}
```

```go
// fmtDuration converts milliseconds to "Xs" or "Xm Ys".
func fmtDuration(ms int64) string {
    s := ms / 1000
    if s < 60 {
        return fmt.Sprintf("%ds", s)
    }
    return fmt.Sprintf("%dm %ds", s/60, s%60)
}
```

### No new flags, no new structs

All data already flows through existing event fields.

---

## Acceptance Criteria

1. A step that changes something displays `~` not `✓`.
2. A step that was already satisfied displays `✓`.
3. A skipped step displays `-  <name>  [<reason>]`.
4. Steps with duration ≥ 2s show `[Xs]` or `[Xm Ys]` at the end of the line.
5. Steps with duration < 2s show no duration.
6. Agent and quiet output formats are unaffected.
7. Recap line counts (changed=N ok=N skipped=N) remain accurate.
