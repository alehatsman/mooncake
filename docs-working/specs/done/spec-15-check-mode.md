# Spec 15: Check Mode (Non-Destructive Audit)

**Status:** Shipped, then superseded by [Spec 16](spec-16-unify-dryrun-execute.md).
The `Checker` interface and `--check` flag described below were both
removed in Spec 16. Their *purpose* — state-aware drift detection — is
now delivered through the unified `Run(ctx, step)` method in
`ModePlan` and the `mooncake plan` command. The `--check` flag is kept
for one release as a deprecated alias that prints a note pointing at
`mooncake plan`.

This spec is preserved as historical context for the design discussion
that led into Spec 16.

**Epic:** E4 Check Mode (S4.1–S4.3)
**Effort:** L (1–2 days)
**Value:** High — turns mooncake into a safe drift-detection tool; required for production use

---

## Problem

Running a playbook always mutates state. There's no way to ask "what would this
do?" or "is this machine in the expected state?" without making changes.

This blocks two use cases:
1. Pre-flight: agents and humans want to review the change set before applying
2. Drift detection: scheduled audit runs that verify state without touching anything

---

## Goal

```
mooncake run --check main.yml
```

Runs every step's "would-change?" query without making any writes. Output:

```
▷ Install neovim           would install
▷ Set shell to zsh         ok (already /usr/bin/zsh)
▷ Link .zshrc              would change (current differs)
- Install OpenJDK          skip [when: apt_available]
▷ Run migrations           skip (shell steps not checkable)

CHECK RECAP  would-change=2  ok=1  skipped=2  not-checkable=1
```

Exit code: 0 if would-change=0, 1 if any would-change.

---

## Symbol Set (check mode only)

| Symbol | Meaning |
|--------|---------|
| `▷` | Would run / would check |
| `↑` | Would change something |
| `✓` | Already in desired state |
| `-` | Skipped (when/creates condition) |
| `?` | Not checkable (shell action — can't predict) |

---

## Per-Action Check Behavior

### package
Query installed state only. Report:
- `✓ already installed` if present
- `↑ would install` if absent

### file
Diff current file vs desired content/template. Report:
- `✓ already matches` if content identical
- `↑ would change` if different (optionally show diff with `--diff`)
- `↑ would create` if file doesn't exist

### shell
Cannot predict — always report `?  not checkable`. Do not run the command.

### template
Same as file — diff rendered template vs current file.

### service
Query service state. Report `✓` or `↑ would start/stop/restart`.

### link (symlink)
Check if link exists and points to correct target. Report `✓` or `↑ would create/retarget`.

---

## Implementation

### Flag

`--check` boolean flag on `mooncake run`. Passed into `executor.Config`.

### `executor.ExecutionContext`

Add `CheckMode bool`. When true, `DispatchStepAction` calls `handler.Check()`
instead of `handler.Execute()`.

### `actions.Handler` interface extension

```go
type Checker interface {
    Check(ctx Context, step *config.Step) (CheckResult, error)
}

type CheckResult struct {
    WouldChange bool
    Reason      string // "would install", "already installed", "not checkable"
    Checkable   bool   // false for shell
}
```

Handlers that don't implement `Checker` are treated as `Checkable: false`.

### Events

New event type: `EventStepChecked`:
```go
type StepCheckedData struct {
    StepID      string `json:"step_id"`
    Name        string `json:"name"`
    Action      string `json:"action"`
    WouldChange bool   `json:"would_change"`
    Checkable   bool   `json:"checkable"`
    Reason      string `json:"reason"`
    Level       int    `json:"level"`
    Depth       int    `json:"depth,omitempty"`
}
```

`EventRunCompleted` gains a `WouldChangeSteps int` field when run in check mode.

### Console subscriber

Render `EventStepChecked` using check-mode symbols.

### Recap line

```
CHECK RECAP  would-change=N  ok=N  skipped=N  not-checkable=N
```

---

## Handlers to implement `Check()`

Priority order:
1. `package` — uses existing `isPackageInstalled`
2. `file` — read current file, compare to desired content
3. `link` — `os.Lstat` check
4. `service` — query systemd/launchd state
5. `shell` — always `Checkable: false`
6. `template` — render template, compare to current file

---

## Acceptance Criteria

1. `--check` runs without writing anything (verified by test: no filesystem changes).
2. Package steps report `✓` for installed packages, `↑` for absent ones.
3. File steps show `✓` when content matches, `↑` when different.
4. Shell steps show `?  not checkable` and do not execute the command.
5. Skipped steps (when/creates) show `-` same as normal mode.
6. Check recap shows `would-change=N` count.
7. Exit code 1 if `would-change > 0`, 0 otherwise.
8. `--check --output agent` emits JSONL with `event: "step_checked"` objects.
