# Agent shortcuts — mooncake

Discoverable surface for LLM agents working in this repo. For full project
guide see [LLM_GUIDE.md](./LLM_GUIDE.md); for rules of engagement see
[CLAUDE.md](./CLAUDE.md). This file is the working-memory cheat sheet.

## 👉 What to work on right now

Open [`docs-working/PICKUP.md`](./docs-working/PICKUP.md) first.
Curated short list of pickups ranked by leverage, with claim
slugs and "where to read" pointers. Saves you the 10-files-to-skim
problem.

## Focused feedback (use these — avoid `task ci`)

```
task check-pkg PKG=internal/apply     # build + test -race + lint, one package
task build-pkg PKG=internal/apply
task test-pkg  PKG=internal/apply
task test-fn   FN=TestApplyRunner PKG=internal/apply
task lint-pkg  PKG=internal/apply
task lint-new                         # lint only lines changed since HEAD~1
task lint-fix                         # auto-fix what golangci-lint can fix
```

Sub-second for most edits. Reserve `task ci` for pre-push or a full sweep.

## Code lookups (replace grep+read cycles)

```
task sym Q='Runner'                                          # find symbol locations
task doc SYM=fmt.Sprintf                                     # docs for a symbol
task refs    LOC=internal/apply/runner.go:33:6               # references to symbol at LOC
task callers LOC=internal/apply/runner.go:49:6               # call hierarchy
task impl    LOC=internal/actions/interfaces.go:241:6        # interface implementations
```

Typical chain: `task sym Q=Foo` → pick the line you want → feed it as `LOC=` to refs/callers/impl.

## Soft-cap state (read before any refactor)

```
task budget-status
```

Prints current handler LOC, gocyclo>35 functions, and `config.Step` universal-field count vs the three CLAUDE.md soft caps. Use it to pick the next refactor target.

## Architecture snapshot

```
task arch-snapshot     # regenerate docs-working/ARCH_SNAPSHOT.md
```

Package graph + LOC + coupling metrics. Re-run after structural changes.

## Hard rules (see CLAUDE.md for full)

- **Do not commit/push** unless the user explicitly requests it.
- **Use a worktree** for any implementation work: `git worktree add ../mooncake-<slug> -b worktree-<slug>`. Doc-only edits can stay on the current branch.
- **Claim work** in `~/.mooncake/claims.jsonl` before starting (`claimed` → `in-progress` → `done`/`abandoned`).
- **Soft caps** (handler LOC > 1500, gocyclo > 35, Step fields > 40) are tracked, not auto-blocked. `task budget-status` shows current state.

## Where things live

| Concern | Location |
|---|---|
| Action implementations | `internal/actions/<name>/` (one package per action type) |
| Step schema (action fields, universal fields) | `internal/config/config.go` |
| Planner | `internal/plan/` |
| Executor | `internal/executor/` |
| Facts (host introspection) | `internal/facts/` |
| Presets | `internal/presets/` + `presets/` (the example tree) |
| CLI entry points | `cmd/` |
| Generated docs | `docs-next/generated/` (regen via `task docs-generate`) |
| Generated schema | `internal/config/schema.json` (regen via `task schema-generate`) |
| Architecture report | `docs-working/arch-report/` |
| Manual-test findings (closed) | `docs-working/archive/analysis/findings-2026-05-15/` |
| Code-review findings | `docs-working/code-review/findings/` (queue in `code-review/TODO.md`) |
| Pickup list (start here) | `docs-working/PICKUP.md` |

## When tests look wrong

- `task test-fn FN=TestX PKG=...` to laser-focus.
- `go test -race -count=1 -v -run '^TestX$/^case_name$' ./...` to drill into a subtest.
- Race failures are real — never `-race=false` to make them go away.
