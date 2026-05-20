---
id: F011
title: Cross-cutting — 24 action handlers still carry Execute + DryRun + Run (Spec-16 migration only partial)
severity: smell
package: cross-cutting
files: internal/actions/{artifact_capture,artifact_validate,assert,command,download,file_insert,file_delete_range,file_patch_apply,file_replace,include_vars,package,preset,print,repo_apply_patchset,repo_search,repo_tree,service,shell,template,tool,unarchive,vars}/handler.go
status: done
progress: |
  Batch 1 (worktree-fix-f011-batch1, 2026-05-16): 5 small handlers
  migrated — shell, command, print, vars, include_vars. Pattern: Run
  absorbed Execute's body into its apply branch; DryRun deleted (its
  log-only "[DRY-RUN] Would …" behavior is superseded by the typed
  Result.Reason on Run's plan branch). Tests: h.Execute → h.Run
  (mock contexts default to ModeApply); legacy TestHandler_DryRun
  blocks deleted where they only asserted log presence.
  Batch 2 (worktree-fix-f011-batch2, 2026-05-16): +1 handler —
  preset. Same pattern. Service + tool were migrated separately by
  other agents in the interim.
  Batch 3 (worktree-fix-f011-batch3, 2026-05-16): +4 handlers —
  repo_search, repo_tree, file_delete_range, file_replace. Pattern
  variants observed: file_delete_range + file_replace had Run as the
  authoritative apply body already (Execute was dead) — just deleted
  Execute + DryRun + the legacy TestHandler_DryRun that pointed at a
  non-existent /tmp path. repo_search + repo_tree's Run delegated to
  Execute via plan-mode result patching — Execute renamed to private
  runImpl, Run calls runImpl + stamps Checkable/Reason on plan mode.
  Batch 4 (close-out, 2026-05-16): the remaining 10 handlers landed
  across three commits — 4fffeb3e (file_insert + file_patch_apply +
  package + template, full delete pattern), 8f0a90e2 (artifact_capture
  + download + repo_apply_patchset + unarchive, Execute→runApply
  rename + DryRun delete), c9ee5dad (artifact_validate + assert,
  inline). Follow-up 69aba93a removed the dead helpers left over
  after deletion and restored NamesExpr in package.Run. Merged to
  master at bf2fdd77 with commit message "Spec-16 migration complete,
  all 21 handlers Run-only" (21 not 24 — service + tool migrated in
  parallel by other agents during the campaign).
verified: 2026-05-17 — confirmed done on master @ f2bfec0a. `grep -rln '^func \(h \*Handler\) (Execute|DryRun)' internal/actions/` returns zero matches across all 21 handler.go files (artifact_capture, artifact_validate, assert, command, download, file, file_delete_range, file_insert, file_patch_apply, file_replace, include_vars, package, preset, print, repo_apply_patchset, repo_search, repo_tree, service, shell, template, tool, unarchive, vars). Spec-16 unified entry point is the only path. NB: the closeout commit fa6b053c flipped this finding's TODO.md entry but never updated the frontmatter — caught and corrected here.
---

## What

Spec-16 unified entry point (`Run(ctx, step)`) is now defined on
**every** action handler that has an `Execute()`. The arch-wins
migration (`27e9ade` + `b23fd4f`) deleted the legacy
`Execute()`/`DryRun()` for `copy` and `file` and migrated their
tests. **Twenty-four handlers still carry the legacy pair.**

Audit command:

```sh
for f in $(grep -rln 'func (.*Handler) Execute(' internal/actions/ | grep -v _test.go); do
    has_run=$(grep -l 'func (.*Handler) Run(' "$f")
    has_dryrun=$(grep -l 'func (.*Handler) DryRun(' "$f")
    echo "$f  R=$([ -n "$has_run" ] && echo Run || echo -)  D=$([ -n "$has_dryrun" ] && echo DryRun || echo -)"
done
```

All 24 entries return `Run=Run, DryRun=DryRun`. There is no
"already migrated" outlier — `copy` and `file` were the only ones
that lost their legacy pair, and they're not in the list above
because the Execute method was deleted.

Affected packages:

`artifact_capture`, `artifact_validate`, `assert`, `command`,
`download`, `file_delete_range`, `file_insert`, `file_patch_apply`,
`file_replace`, `include_vars`, `package`, `preset`, `print`,
`repo_apply_patchset`, `repo_search`, `repo_tree`, `service`,
`shell`, `template`, `tool`, `unarchive`, `vars` (×24 total).

## Per-handler shape

Spot-check confirmed three distinct patterns:

1. **`Run` delegates to `Execute` for ModeApply** — `tool`,
   `service`, `print`, `shell`. Plan mode is a separate branch in
   `Run`. DryRun does its own thing.
2. **`Run` and `Execute` are independent** — handlers where `Run`
   was added with new (effects-based) logic but `Execute` keeps
   doing direct `os.*` calls. Likely `download`, `package`.
3. **`Run` reimplements `Execute`** — a fork happened during
   Spec-16 introduction. `vars`, `include_vars` candidates.

Without auditing each, we don't know which pattern each handler
uses. The migration prescription in arch-wins (delete Execute,
delete DryRun, route through Run) is sound for pattern (1) but
needs more care for (2) and (3).

## Why it's worth a cross-cutting finding (and not just per-package)

- **LOC pressure.** 24 handlers × ~30 LOC of Execute body + ~15
  LOC of DryRun body = roughly **1,000 LOC of deletable legacy
  code** if all migrations work like `copy` and `file`. Several
  packages near the soft-cap (`service` 1,607, `tool` 1,676,
  `package` 1,216, `file` 1,349) would drop below or further
  below cap.
- **The actions.Handler interface still declares `Execute` and
  `DryRun`.** Until all handlers migrate, the interface can't
  shrink. That's the **load-bearing** part of this finding: the
  legacy methods aren't just per-handler debt, they're an
  interface obligation that other consumers (planner, executor)
  may still call.
- **Per-handler review (F003 for service, F006 for tool) is
  good, but going one-by-one over 24 handlers without a tracking
  list creates re-work.** A single cross-cutting ticket gives
  fixers a checkbox list.

## Suggested fix

Stage 1 — per-handler triage. For each handler, classify into
the three patterns above. Write a one-liner per handler so the
fixer knows the risk:

```
artifact_capture  pattern-1  (Run delegates) — safe delete
artifact_validate pattern-?
assert            pattern-?
command           pattern-?
download          pattern-? + network in Execute (verify Run matches)
file_delete_range pattern-?
...
```

Stage 2 — migrate per-handler in PR-sized batches (3-5
handlers per batch). Sample PR diff per handler is `27e9ade`'s
`copy` chunk — same shape.

Stage 3 — once every handler has Run-only:

```go
// internal/actions/interfaces.go
type Handler interface {
    Metadata() ActionMetadata
    Validate(*config.Step) error
    Run(Context, *config.Step) (Result, error)
    // Execute and DryRun removed.
}
```

Plus the executor's dispatch logic (`internal/executor`) drops
the legacy fast path. That's a separate PR after the per-handler
migrations land.

## Estimated effort

- Triage: 2-3 hours (open each handler, classify).
- Migration batches: ~30 min per handler in the easy case,
  ~2 hours for handlers where Run and Execute have diverged.
  24 handlers × 30-min avg = ~12 hours of work.
- Interface change: 1 PR after all 24 are done, ~1 hour.

## Verification

After each batch:

- `go test ./internal/actions/<package>/...`
- `make budget-status` — LOC numbers drop.
- `grep -rn 'h\.Execute(' internal/actions/` — handler tests that
  call Execute directly should be migrated to Run.

## References

- F003 (service migration scoped finding)
- F006 (tool migration scoped finding)
- `27e9ade` arch-wins — the copy / file migration that
  established the pattern.
- `docs-working/arch-report/2026-05-16-code-review.md` §7
  ("file/handler.go — Spec-16 incomplete") for the prescription.
