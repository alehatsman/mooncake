# Code Review TODO

Living queue. Each iteration consumes one entry from **In progress /
Queue**, produces a finding (or several), and the queue updates.

## Unblocked — ready for fix

Findings that have a complete fix description and don't depend on
something else landing first.

| ID | Title | Severity | Effort | Owner | Status |
|---|---|---|---|---|---|
| F002 | CLAUDE.md soft-cap list stale | doc | XS | — | open |

## Findings index

| ID | Title | Severity | Status | Location |
|---|---|---|---|---|
| F001 | observe_disk Bsize cross-platform cast | risk | **done** | [findings/F001](./findings/F001-observe-disk-bsize-cast.md) |
| F002 | CLAUDE.md soft-cap list stale | doc | open | [findings/F002](./findings/F002-claude-md-soft-cap-list-stale.md) |

## Queue (next iterations, priority order)

1. **`internal/actions/service`** — 1,607 LOC, over soft-cap. Look
   for legacy Execute/DryRun like copy/file had.
2. **`internal/actions/tool`** — 1,676 LOC, over soft-cap.
3. **`internal/explain` — `DisplayFacts`** — gocyclo 44, only
   non-test function over the gocyclo cap.
4. **`internal/config.Step`** — 36/40 field count, growing. Look
   for fields that could be planner-private already.
5. **`internal/agentd/worker`** — has it landed cleanly on
   `apply.Runner`? Anything still hand-wired?
6. **`internal/mcp/tools`** — same question after the runCollector
   deletion.
7. **`internal/executor/executor`** — `ExecuteStep` after the
   extractions. Anything else heavy?
8. **`internal/fleet`** — biggest non-cmd package (4,245 LOC),
   never deep-reviewed at function level.
9. **`internal/agentd`** — 3,100 LOC, growing fast in the last 24h.
10. **`internal/plan`** — planner, including the new `plan/filter`.
11. **`internal/apply/runner.go`** — kernel entrypoint. Verify the
    Subscribers + Close lifecycle change actually closes.
12. **`internal/actions/package`** — 1,216 LOC, within-20% warning.
13. **`internal/actions/copy` after the migration** — verify no
    dead-weight remains after arch-wins.
14. **`internal/actions/file` after the migration** — same.
15. **`cmd/`** — 10,022 LOC of CLI wiring. Spot-check the largest
    files.
16. **`internal/secrets/resolver`** — new, small, easy.
17. **`internal/control`** — new, smallest, foundation-tier.
18. **`internal/plan/filter`** — new.
19. **`internal/presets/registry`** — renamed but otherwise old.
20. **Per-action handlers not above** — git_*, os_*, text_*, wait_*,
    windows_*. Skim for shared smells.
21. **`internal/snapshot`** — minimal_test recently churned.
22. **Tests** — coverage gaps in changed packages.

## Reviewed (done)

| Date | Area | Findings produced |
|---|---|---|
| 2026-05-16 | baseline (build/test/lint/budget) | F001, F002 |

## Cross-cutting themes / patterns to track

Updated as the review uncovers patterns.

- _(none yet — first iteration)_

## Notes for future reviewers

- This pass is **delta on top of** the closed
  `docs-working/arch-report/2026-05-16-code-review.md`. Items there
  marked DONE should not be re-flagged unless a regression appears.
- `make budget-status` is the source of truth for soft caps. Always
  re-run before pinning numbers in a finding.
- `golangci-lint cache clean` before each lint run (cross-worktree
  cache contamination is a known foot-gun, see
  `memory/reference_golangci_cache_contamination.md`).
