# Spec 12: Package Install Summary

**Epic:** E1 Observable Runs (S1.4)  
**Effort:** S (2–4h)  
**Value:** Medium — turns opaque package steps into visible state transitions

---

## Problem

A package step that installs 10 packages and finds 30 already present shows only:

```
~ Install dev tools  [8s]
```

No way to know which packages were actually new vs already satisfied.

---

## Goal

After every package step, print a compact summary line showing what was new and
what was already present. Only shown in text/console output mode — agent and quiet
modes are unaffected.

```
~ Install dev tools  [8s]
  package  +neovim +fzf +ripgrep  (already: git zsh tmux curl)
```

If nothing was new (all already present), show nothing extra (step shows `✓`, no summary).

---

## Event

Add `EventPackageManaged` data struct (the event type already exists):

```go
type PackageManagedData struct {
    Installed      []string `json:"installed"`
    AlreadyPresent []string `json:"already_present"`
    Removed        []string `json:"removed,omitempty"`
    Manager        string   `json:"manager"`
}
```

Package handler emits this event at the end of `installPackages` / `removePackages`.

---

## Implementation

### `internal/events/event.go`

Add `PackageManagedData` struct.

### `internal/actions/package/handler.go`

In `installPackages`: track two slices `newPkgs` and `existingPkgs` as each
package is checked. Emit `EventPackageManaged` at the end via `ec.EmitEvent`.

In `removePackages`: track `removed` slice. Emit similarly.

### `internal/logger/console_subscriber.go`

Handle `EventPackageManaged` in `renderText`:

```go
case events.EventPackageManaged:
    if data, ok := event.Data.(events.PackageManagedData); ok {
        c.renderPackageManaged(data)
    }
```

```go
func (c *ConsoleSubscriber) renderPackageManaged(data events.PackageManagedData) {
    if len(data.Installed) == 0 && len(data.Removed) == 0 {
        return // all already present — no summary needed
    }
    var parts []string
    for _, p := range data.Installed {
        parts = append(parts, "+"+p)
    }
    for _, p := range data.Removed {
        parts = append(parts, "-"+p)
    }
    line := "  package  " + strings.Join(parts, " ")
    if len(data.AlreadyPresent) > 0 {
        line += "  (already: " + strings.Join(data.AlreadyPresent, " ") + ")"
    }
    fmt.Println(line)
}
```

Indentation: always 2 spaces (sub-line under the step).

### Agent subscriber

In `EventPackageManaged` case: include `installed`, `already_present`, `removed`
fields in the JSONL event for agent mode.

---

## Acceptance Criteria

1. Packages newly installed show `+name` in summary.
2. Packages already present show in `(already: ...)` list.
3. Packages removed show `-name` in summary.
4. When all packages already present, no summary line is printed.
5. Summary line is indented with 2 spaces under the step.
6. Agent JSONL includes `installed`/`already_present`/`removed` arrays.
7. Quiet mode shows no summary line.
