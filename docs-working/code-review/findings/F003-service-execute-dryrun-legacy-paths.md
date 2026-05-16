---
id: F003
title: service handler still carries legacy Execute/DryRun alongside Run
severity: smell
package: internal/actions/service
file: internal/actions/service/handler.go
lines: 102-172, 1230-1332
status: done
fixed: 2026-05-16 — service handler confirmed to have only Metadata/Permissions/Validate/Run; Execute and DryRun were already deleted (landed as part of the arch-wins batch). TODO.md updated to reflect.
---

## What

`service` is the last large handler still on the dual-path shape
that `copy` and `file` shed in arch-wins (`27e9ade`, `b23fd4f`).

- `Execute()` (lines 102-113) — legacy Spec-1 entry. Calls `runApply`.
- `DryRun()` (lines 136-172) — separate code path that only logs
  what would happen, no actual plan-mode predication.
- `Run()` (lines 1230-1332) — Spec-16 unified entry. For `ModeApply`
  it delegates to `runApply` too. For `ModePlan` it has its own
  ~100-line implementation that duplicates the apply logic minus
  the writes.

This is the same shape `copy.Execute` had before deletion (and was
gocyclo 41), and the same shape `file.Execute` had. Both got
deleted in the arch-wins migration. `service` was not migrated
in that batch.

## Why it's a smell

1. **Three entry points doing two jobs.** Plan-mode predication
   already lives in `Run()` — `DryRun()` is no longer a behavior
   addition, it's a logging variant. Removing `DryRun()` is
   non-functional (Spec-16 routes Plan through `Run()`).

2. **`Run()`'s plan branch (lines 1259-1330) duplicates apply
   logic.** Two `Render` calls for the same name, two existence
   checks for unit/dropin paths, two `renderTemplateOrContent`
   invocations against the same content. A "would-change" mode
   that shells out to `systemctl is-active` and then a "do-change"
   mode that shells out to `systemctl is-active` again is a clear
   factor: extract a shared `inspectSystemd(svc, ec)` returning
   `(currentState systemdState, err error)` and let both predict
   and apply pivot on that struct.

3. **Plan mode is more restrictive than apply on darwin.** Line
   1243-1247: `if runtime.GOOS != "linux"` → `Checkable = false`.
   But the apply path on darwin works (handleLaunchdService). So
   plan-mode says "cannot inspect" while apply mode happily does
   the change. That's a bad UX: dry-run says "I can't tell" but
   the real run succeeds. Either implement launchd plan-mode (the
   `launchctl print` call already exists in `isLaunchdServiceLoaded`),
   or document explicitly that plan-mode is Linux-only and have
   the CLI surface that to users.

## Suggested fix

Stage 1 (mechanical, mirrors copy/file):

- Delete `Execute()` and `DryRun()`.
- Routing: `Run()` already covers both modes via `ctx.Mode()`.
- Tests: `*_test.go` calls `h.Execute(` and `h.DryRun(` get
  rewritten to `h.Run(` with `ctx.Mode()` set appropriately
  (see `internal/actions/file/handler_test.go` for the
  `TestHandler_PlanMode` and `TestDefaultModeFor` pattern).

Stage 2 (deeper, optional):

- Extract `inspectSystemd(svc *ServiceAction, name string, ec ...)
  (systemdSnapshot, error)` that returns `{active, enabled,
  unitContentMatch, dropinContentMatch}`. Both the apply path and
  the plan path consume this struct. Eliminates ~80 lines of
  duplicated read-and-compare logic.

- Add launchd plan-mode (mirror manageLaunchdServiceState's
  isLoaded check + plist content compare) so the dry-run / apply
  asymmetry on darwin goes away.

## Why it matters

The handler is **1,607 LOC, 107 over the 1,500 soft cap**
(F004). Stage 1 alone deletes ~70 lines (Execute body 12, DryRun
body 37, plus test churn). Stage 2 wins another ~50 LOC and gets
the apply / plan parity right on darwin. Combined: handler drops
under the cap without splitting per-OS.

If neither stage lands, the **next refactor pressure point** is
per-OS splitting (`service/{linux,darwin,windows}/`). That's a
bigger move because the shared helpers (`renderTemplateOrContent`,
`writeFileWithPrivileges`, `writeFileWithSudo`, `parseFileMode`)
have to go up into the package root. Worth doing once but harder
to back out of.

## Verification

- `go test ./internal/actions/service/...` — same pass set.
- `make budget-status` — `service` LOC should drop to ~1,540
  after Stage 1, ~1,490 after Stage 2.
- `gocyclo` of `Run` and `HandleService` — should stay under 35.

## References

- `27e9ade` arch-wins: deleted `copy.Execute`/`DryRun` and
  `file.Execute`/`DryRun` with the same prescription.
- `docs-working/arch-report/2026-05-16-code-review.md` §7 (file)
  for the exact migration pattern, including test rewrite shape.
