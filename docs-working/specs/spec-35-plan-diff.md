# Spec 35 — `mooncake plan --diff`

**Status**: Implemented (commit TBD)  
**Epic**: Agent Efficiency S2.4  

---

## Problem

`mooncake plan` already shows *which* steps would change and *why* (the `Reason`
string — "content differs (1024 -> 1156 bytes)"). It does not show *what* would
change. For file operations the diff text is the most useful thing an operator or
agent can see before deciding to apply.

Current output:
```
↑ Write nginx config                               content differs (1024 -> 1156 bytes)
↑ Write /etc/hosts                                 content differs (42 -> 56 bytes)
```

Desired output with `--diff`:
```
↑ Write nginx config                               content differs (1024 -> 1156 bytes)
  --- /etc/nginx/nginx.conf
  +++ /etc/nginx/nginx.conf (proposed)
  @@ -1,5 +1,7 @@
   worker_processes auto;
  +worker_connections 1024;
  +events { worker_connections 1024; }
   ...

↑ Write /etc/hosts                                 content differs (42 -> 56 bytes)
  --- /etc/hosts
  +++ /etc/hosts (proposed)
  @@ -3,2 +3,3 @@
   127.0.0.1 localhost
  +192.168.1.10 myserver
```

---

## What needs to change

### 1. `effects.ContentDiff` — add unified diff text

`internal/effects/default.go`

```go
type ContentDiff struct {
    OldSize int    `json:"old_size"`
    NewSize int    `json:"new_size"`
    OldHash string `json:"old_hash"`
    NewHash string `json:"new_hash"`
    // UnifiedDiff is the unified diff text between old and new content.
    // Only populated in ModePlan. Empty string means not available
    // (e.g. old content was unreadable, or file is being created from scratch).
    UnifiedDiff string `json:"unified_diff,omitempty"`
}
```

Populate `UnifiedDiff` in `newContentDiff()` — already has `oldB` and `newB` in
scope. Use `github.com/pmezard/go-difflib/difflib` (added to go.mod):

```go
diff := difflib.UnifiedDiff{
    A:        difflib.SplitLines(string(oldB)),
    B:        difflib.SplitLines(string(newB)),
    FromFile: path,
    ToFile:   path + " (proposed)",
    Context:  3,
}
text, _ := difflib.GetUnifiedDiffString(diff)
```

Cap diff output: if the diff exceeds 4000 bytes, truncate with a
`[...truncated N lines...]` marker. Large diffs in plan output are noise.

For new files (no `oldB`), `UnifiedDiff` is left empty — the `Reason` already
says "would create file (N bytes)".

### 2. `plan.StepInspection` — add Detail field

`internal/plan/plan.go`

```go
type StepInspection struct {
    StepID      string `json:"step_id" yaml:"step_id"`
    ActionType  string `json:"action_type,omitempty" yaml:"action_type,omitempty"`
    WouldChange bool   `json:"would_change" yaml:"would_change"`
    Checkable   bool   `json:"checkable" yaml:"checkable"`
    Reason      string `json:"reason,omitempty" yaml:"reason,omitempty"`
    Skipped     bool   `json:"skipped,omitempty" yaml:"skipped,omitempty"`
    // Detail carries action-specific plan data. Currently only populated
    // for file write steps where content would change: effects.ContentDiff.
    Detail any `json:"detail,omitempty" yaml:"detail,omitempty"`
}
```

### 3. Planner — pipe Effect.Detail into StepInspection

`internal/plan/planner.go` or wherever `StepInspection` is built from the
executor result.

Find where `Result.WouldChange` and `Result.Reason` are read to build
`StepInspection`, and also copy `Result`'s detail data.

The chain is: `Effect.Detail` → handler stores it in `Result` (via `SetData` or
a new mechanism) → planner copies it into `StepInspection.Detail`.

Check if handlers currently propagate `Effect.Detail` to `Result`. If not, add
a `SetDetail(any)` method to the `actions.Result` interface and implement it on
`executor.Result`.

### 4. `mooncake plan --diff` flag

`cmd/mooncake.go`

Add `--diff` / `-d` boolean flag to the `plan` subcommand:

```go
&cli.BoolFlag{
    Name:    "diff",
    Aliases: []string{"d"},
    Usage:   "Show content diffs for steps that would change files",
},
```

### 5. `formatPlanText` — render diffs under ↑ steps

When `--diff` is set, after printing each `↑` step line, check if
`ins.Detail` is a `effects.ContentDiff` with a non-empty `UnifiedDiff`.
If so, print it indented by two spaces.

```
↑ Write nginx config                               content differs (1024 -> 1156 bytes)
  --- /etc/nginx/nginx.conf
  +++ /etc/nginx/nginx.conf (proposed)
  @@ -1,5 +1,7 @@
  ...
```

JSON/YAML output formats already carry `Detail` via `StepInspection` — no
changes needed there.

---

## Scope

Only `file` action write steps produce diffs (the `WriteFile` effect). Other
`↑` steps (mkdir, chmod, symlink, package installs) already have descriptive
`Reason` strings that are sufficient. No diff needed for those.

Shell/command steps are not checkable (`?`) — no change needed.

---

## Files to change

| File | Change |
|---|---|
| `internal/effects/default.go` | Add `UnifiedDiff` to `ContentDiff`, populate in `newContentDiff()` |
| `internal/actions/performer.go` | No change — `Effect.Detail any` already exists |
| `internal/actions/interfaces.go` | Add `SetDetail(any)` to `Result` interface |
| `internal/executor/result.go` | Implement `SetDetail`, add `Detail any` field to `Result` |
| `internal/plan/plan.go` | Add `Detail any` to `StepInspection` |
| `internal/plan/planner.go` | Copy `Result.Detail` → `StepInspection.Detail` |
| `cmd/mooncake.go` | Add `--diff` flag, update `formatPlanText` |

---

## Out of scope

- Diff for `template` action (would need to render the template twice, once for
  current content and once for new — complex, defer)
- Colour highlighting of diff lines (nice to have, separate concern)
- `--diff` in JSON/YAML output format (Detail is already included via the struct)
