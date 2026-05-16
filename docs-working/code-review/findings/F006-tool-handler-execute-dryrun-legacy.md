---
id: F006
title: tool handler: Execute/DryRun/Run dual-path (same as service)
severity: smell
package: internal/actions/tool
file: internal/actions/tool/handler.go
lines: 64-187, 190-206, 351-385
status: fixed
---

## ✅ Fixed

`Handler.Execute` and `Handler.DryRun` deleted; the install pipeline
moved into a new unexported `Handler.applyTool` that `Handler.Run`
calls for `ModeApply`. Plan mode keeps its existing branch in
Run (cleaner than DryRun's silent-no-op-on-Get-failure shape).

3 test call sites in `handler_mise_test.go` migrated from
`h.Execute(...)` / `(&Handler{}).Execute(...)` to `.Run(...)`. The
test fixture (`newToolCtx`) already sets `Mode: actions.ModeApply`,
so behavior is bit-identical to pre-fix. `install_test.go` and
`install_binary_test.go` had no Execute references despite the
finding's reference list — the migration was 3 lines, not the
~10 the finding implied.

`go test -race ./internal/actions/tool/...` green; lint clean.
The two TestApply_RoundTrip / TestIntegration_RoundTrip failures
in the broader sweep are the pre-existing TCP-integration flakes
(pass on rerun).

### Pre-fix payoff

`internal/actions/tool/handler.go`: -17 LOC (deleted DryRun);
Execute → applyTool rename keeps that block's size the same.

---

## What

Same Spec-16-incomplete shape as `service` (F003):

- `Execute()` (line 64-187) — the legacy entrypoint. 120+ lines of
  backend-dispatch + install pipeline + lockfile management.
- `DryRun()` (line 190-206) — a separate ~15-line dry-run path that
  consults `backend.Locate` and logs.
- `Run()` (line 351-385) — Spec-16 entrypoint. For non-plan modes
  delegates to `Execute()`; the plan branch re-implements the
  `backend.Locate` lookup that `DryRun()` does, slightly differently.

The Plan-mode and DryRun-mode logic disagrees in one detail:
`DryRun` (line 197-203) silently no-ops when `Get(t.Backend)` fails
(`err == nil` is the only branch); `Run` ModePlan (line 372-375)
returns the error. Different error semantics for what should be
"the same dry-run."

## Why it's a smell

Three entry points for what is now two cases (plan / apply). Same
prescription as `copy.Execute` and `file.Execute` deletion in
arch-wins (`27e9ade`).

## Suggested fix

Delete `Execute()` and `DryRun()`. Inline `Execute()`'s body into
`Run()` under `ctx.Mode() == ModeApply`, since `Run()` already
shells `ModePlan` to its plan branch.

Tests in `handler_mise_test.go`, `install_test.go`,
`install_binary_test.go` use `h.Execute(...)` directly — migrate
to `h.Run(...)` with an explicit `ctx.Mode()`. Pattern in
`internal/actions/file/handler_test.go` after the arch-wins
migration.

Expected payoff: ~30 LOC deleted from `handler.go`. Doesn't on its
own bring the package under the 1500 cap (1676 LOC currently),
but it's a step. The bigger wins for this package are in F008
(template field repetition) and the natural further split that
the per-backend files already do — see F009 candidate around
moving lockfile management out of `handler.go.Execute` into a
free function alongside `InstallURL`.

## Verification

- `go test ./internal/actions/tool/...`
- `grep -rn 'h\.Execute\|handler\.Execute' internal/actions/tool/` — zero hits.
- `make budget-status` — package drops ~30 LOC.

## References

- F003 — same migration in `service`.
- `27e9ade` arch-wins — same pattern in `copy`, `file`.
