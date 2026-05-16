# Code Review TODO

Living queue. Each iteration consumes one entry from **In progress /
Queue**, produces a finding (or several), and the queue updates.

## Unblocked — ready for fix

Findings that have a complete fix description and don't depend on
something else landing first.

| ID | Title | Severity | Effort | Owner | Status |
|---|---|---|---|---|---|
| F002 | CLAUDE.md soft-cap list stale | doc | XS | — | open |
| F003 | service handler still has Execute/DryRun legacy paths | smell | S | — | open |
| F004 | service: 6× repeated sudo/exec block (in-package) | smell | S | — | open |
| F005 | Cross-package: 6 implementations of "sudo -S shell-out" | smell | M | — | open |
| F006 | tool handler: Execute/DryRun legacy paths | smell | S | — | open |
| F007 | tool: http.Get / DefaultClient with no timeout / context | risk | S | — | open |
| F008 | tool.renderToolTemplates: 9× manual field render | readability | XS | — | open |
| F009 | explain.DisplayFacts: gocyclo 44, split into sections | smell | S | — | open |
| F010 | explain test: TestDisplayFacts_NilFacts is dead (no call) | smell | XS | — | open |

## Findings index

| ID | Title | Severity | Status | Location |
|---|---|---|---|---|
| F001 | observe_disk Bsize cross-platform cast | risk | **done** | [findings/F001](./findings/F001-observe-disk-bsize-cast.md) |
| F002 | CLAUDE.md soft-cap list stale | doc | open | [findings/F002](./findings/F002-claude-md-soft-cap-list-stale.md) |
| F003 | service: legacy Execute/DryRun | smell | open | [findings/F003](./findings/F003-service-execute-dryrun-legacy-paths.md) |
| F004 | service: sudo/exec duplication in-package | smell | open | [findings/F004](./findings/F004-service-systemd-sudo-shell-duplication.md) |
| F005 | sudo -S shell-out helper cross-package | smell | open | [findings/F005](./findings/F005-sudo-shell-helper-cross-package.md) |
| F006 | tool handler legacy Execute/DryRun | smell | open | [findings/F006](./findings/F006-tool-handler-execute-dryrun-legacy.md) |
| F007 | tool: http no timeout / context | risk | open | [findings/F007](./findings/F007-tool-fetch-no-timeout-no-context.md) |
| F008 | tool.renderToolTemplates manual repetition | readability | open | [findings/F008](./findings/F008-tool-renderToolTemplates-manual-repetition.md) |
| F009 | explain.DisplayFacts section split | smell | open | [findings/F009](./findings/F009-explain-DisplayFacts-section-split.md) |
| F010 | explain TestDisplayFacts_NilFacts is dead | smell | open | [findings/F010](./findings/F010-explain-test-dead-nil-test.md) |

## Queue (next iterations, priority order)

1. ~~`internal/actions/service`~~ — done in this iteration → F003, F004, F005.
2. ~~`internal/actions/tool`~~ — done → F006, F007, F008.
3. ~~`internal/explain` — `DisplayFacts`~~ — done → F009, F010.
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
| 2026-05-16 | `internal/actions/service` (1,607 LOC) | F003, F004, F005 |
| 2026-05-16 | `internal/actions/tool` (1,676 LOC) | F006, F007, F008 |
| 2026-05-16 | `internal/explain.DisplayFacts` (gocyclo 44) | F009, F010 |

## Cross-cutting themes / patterns to track

Updated as the review uncovers patterns.

- **Spec-16 migration incomplete in `service` and `tool`** (F003,
  F006). Same shape as the arch-wins `copy` / `file` cleanup —
  every handler that still has `Execute`/`DryRun` is technical
  debt of the same kind. Audit remaining: `internal/actions/{copy,
  file, service, tool, ...}` grep `func \(.*\) Execute\(`.
- **HTTP calls without timeouts/context** (F007). Audit other
  packages that use `http.Get` / `http.DefaultClient`: `download`,
  `wait_http`, `mcp`, `llm`. Same pattern likely recurs.
- **`sudo -S` shell-out reimplemented in 6 packages** (F005).
  Inconsistent guard handling means become-on-unsupported-host
  produces 3 different error shapes today.
- **`make budget-status` is now the truth — CLAUDE.md inline list
  has drifted** (F002). Reviewers should re-run `make budget-status`
  before pinning numbers.

## Notes for future reviewers

- This pass is **delta on top of** the closed
  `docs-working/arch-report/2026-05-16-code-review.md`. Items there
  marked DONE should not be re-flagged unless a regression appears.
- `make budget-status` is the source of truth for soft caps. Always
  re-run before pinning numbers in a finding.
- `golangci-lint cache clean` before each lint run (cross-worktree
  cache contamination is a known foot-gun, see
  `memory/reference_golangci_cache_contamination.md`).
