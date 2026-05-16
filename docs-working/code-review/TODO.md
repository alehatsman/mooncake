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
| F011 | Cross-cutting: 24 handlers still have Execute/DryRun/Run | smell | XL | — | open |
| F012 | Cross-cutting: 9 packages with http.Get / no timeout | risk | M | — | open |
| F016 | agentd.Worker: context.Background → applies cannot be cancelled | risk | M | — | open |
| F021 | apply.Config.ExtraSubscribers doc claims publisher closes them; runner does | doc | XS | — | open |
| F026 | file/copy handlers use unbounded os.ReadFile on user paths — large files load entire content into RAM | risk | M | — | open |
| F028 | git_clone askpass returns password for the username prompt too — auth fails for bare HTTPS URLs | bug | S | — | open |
| F029 | agentd.bearerAuthMiddleware leaks Authorization-header length via timing side-channel | risk | XS | — | open |
| F030 | security.FilePasswordProvider rejects 0400 / stricter-than-0600 modes — exact-equality check | smell | XS | — | open |

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
| F011 | cross-cutting: 24 handlers w/ legacy paths | smell | open | [findings/F011](./findings/F011-cross-cutting-execute-dryrun-spec16-incomplete.md) |
| F012 | cross-cutting: http no timeout (9 pkgs) | risk | open | [findings/F012](./findings/F012-cross-cutting-http-no-timeout.md) |
| F013 | config.Step stale "74" comment + Creates/Unless aliases | doc | **done** | [findings/F013](./findings/F013-config-step-stale-74-comment-and-alias-redundancy.md) |
| F014 | fleet.Apply WithoutCancel hangs Ctrl-C | risk | **done** | [findings/F014](./findings/F014-fleet-apply-context-withoutcancel-no-timeout.md) |
| F015 | agentd.Worker hub-close cleanup asymmetry | smell | **done** | [findings/F015](./findings/F015-agentd-worker-chdir-error-hub-leak.md) |
| F016 | agentd.Worker no-cancel context | risk | open | [findings/F016](./findings/F016-agentd-worker-context-background-no-cancel.md) |
| F017 | executor continue_on_error double emit | bug | **done** | [findings/F017](./findings/F017-executor-continue-on-error-double-emit.md) |
| F018 | shell scanner 64KB line cap | bug | **done** | [findings/F018](./findings/F018-shell-bufio-scanner-line-overflow.md) |
| F019 | secrets.Resolve misses step.Vars | bug | **done** | [findings/F019](./findings/F019-secrets-resolver-missing-vars-and-interface-maps.md) |
| F020 | apply.Runner os.Exit hostile to embedded callers | risk | **done** | [findings/F020](./findings/F020-apply-runner-os-exit-hostile-to-embedded-callers.md) |
| F021 | apply.Config.ExtraSubscribers doc-drift | doc | open | [findings/F021](./findings/F021-apply-config-extrasubscribers-doc-drift.md) |
| F022 | mcp uses NewTestLogger in production | smell | **done** | [findings/F022](./findings/F022-mcp-uses-NewTestLogger-in-production.md) |
| F023 | package handler swallows template-render errors | bug | **done** | [findings/F023](./findings/F023-package-handler-template-render-error-swallow.md) |
| F024 | planner walkAndRender misses map[string]interface{} | bug | **done** | [findings/F024](./findings/F024-planner-walkAndRender-missing-map-string-interface.md) |
| F025 | fleet.peerDiff misses Roles + SSH | bug | **done** | [findings/F025](./findings/F025-fleet-peerDiff-missing-roles-ssh-fields.md) |
| F026 | file/copy unbounded os.ReadFile in handler | risk | open | [findings/F026](./findings/F026-file-copy-unbounded-os-ReadFile-loads-entire-file-in-memory.md) |
| F027 | agentd self_upgrade sanityCheckBinary no-timeout | risk | **done** | [findings/F027](./findings/F027-agentd-self-upgrade-sanityCheckBinary-no-timeout.md) |
| F028 | git_clone askpass returns password for username prompt | bug | open | [findings/F028](./findings/F028-git-clone-askpass-returns-password-for-username-prompt.md) |
| F029 | agentd bearer-auth length side-channel | risk | open | [findings/F029](./findings/F029-agentd-bearerAuthMiddleware-length-side-channel.md) |
| F030 | security.FilePasswordProvider mode exact-equality | smell | open | [findings/F030](./findings/F030-security-FilePasswordProvider-rejects-more-restrictive-modes.md) |

## Queue (next iterations, priority order)

1. ~~`internal/actions/service`~~ — done in this iteration → F003, F004, F005.
2. ~~`internal/actions/tool`~~ — done → F006, F007, F008.
3. ~~`internal/explain` — `DisplayFacts`~~ — done → F009, F010.
4. ~~`internal/config.Step`~~ — done → F013.
5. ~~`internal/agentd/worker`~~ — done → F015, F016.
6. ~~`internal/mcp/tools`~~ — done → F022 (NewTestLogger in
   production). apply.Runner integration looks clean post-refactor.
7. ~~`internal/executor/executor`~~ — partial → F017
   (continue_on_error double-emit). Other extractions look clean.
8. **`internal/fleet`** — biggest non-cmd package (4,245 LOC).
   Partial: F014 (apply.go post-stream recovery). Rest of the
   package (controller, bootstrap, multiplex, peers) still queued.
9. **`internal/agentd`** — 3,100 LOC, growing fast in the last 24h.
10. ~~`internal/plan`~~ — partial → F024 (walkAndRender same
    closed-kind-set bug as F019, planner side).
11. ~~`internal/apply/runner.go`~~ — done → F020 (signal-handler
    hostile to embedded callers), F021 (Config.ExtraSubscribers
    doc-drift).
12. ~~`internal/actions/package`~~ — partial → F023
    (template-render swallow). 901 LOC handler.go; runCmd
    is another F005 hit (no IsBecomeSupported/SudoPass guards).
    Per-package isPackageInstalled is a perf footgun on big
    lists; track separately if it becomes a UX complaint.
13. ~~`internal/actions/copy` after the migration~~ — done; clean
    Run-only post-migration (283 LOC). F026 (unbounded ReadFile)
    is a separate concern.
14. ~~`internal/actions/file` after the migration~~ — done; clean
    Run-only post-migration (515 LOC). F026 also applies here.
15. **`cmd/`** — 10,022 LOC of CLI wiring. Spot-check the largest
    files.
16. ~~`internal/secrets/resolver`~~ — done → F019 (silent miss).
17. ~~`internal/control`~~ — reviewed, no findings (clean
    foundation-tier package).
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
| 2026-05-16 | cross-cutting audit (Execute/DryRun + HTTP timeouts) | F011, F012 |
| 2026-05-16 | `internal/config.Step` doc-drift | F013 |
| 2026-05-16 | `internal/fleet/apply.go` (partial) | F014 |
| 2026-05-16 | `internal/agentd/worker.go` | F015, F016 |
| 2026-05-16 | `internal/executor/executor.go` (partial) | F017 |
| 2026-05-16 | `internal/actions/shell/handler.go` | F018 |
| 2026-05-16 | `internal/secrets/resolver/resolve.go` | F019 |
| 2026-05-16 | `internal/apply/runner.go` + `config.go` | F020, F021 |
| 2026-05-16 | `internal/mcp/tools.go` | F022 |
| 2026-05-16 | `internal/plan/filter/tags.go` | none (clean) |
| 2026-05-16 | `internal/actions/package` (901 LOC) | F023 |
| 2026-05-16 | `internal/agentd/files_handler.go` | none (clean — sec-conscious) |
| 2026-05-16 | `internal/agentd/runs_handler.go` | none (clean — sees the F018 pattern done right) |
| 2026-05-16 | `internal/fleet/controller.go` / `orchestrator.go` | none (clean — orchestrator uses ctx, unlike apply.Runner per F020) |
| 2026-05-16 | `internal/plan/planner.go` walkAndRender | F024 |
| 2026-05-16 | `internal/fleet/multiplex.go` | none (clean) |
| 2026-05-16 | `internal/fleet/peers.go` | F025 |
| 2026-05-16 | `internal/snapshot/{minimal,diff}.go` | none (clean) |
| 2026-05-16 | `internal/actions/{copy,file}` post-migration | F026 (unbounded ReadFile) |
| 2026-05-16 | `internal/presets/registry/remote.go` | already covered by F012 (http no timeout) |
| 2026-05-16 | `cmd/presets.go` spot-check | none (clean — preset Type schema matches handler switch) |
| 2026-05-16 | `internal/agentd/store.go` | none (clean — ULID-validated, atomic writes, daemon-restart reconcile) |
| 2026-05-16 | `internal/agentd/self_upgrade.go` | F027 (sanityCheckBinary no-timeout) |
| 2026-05-16 | `internal/actions/git_clone` | F028 (askpass username bug) |
| 2026-05-16 | `internal/agentd/middleware.go` | F029 (bearer-auth length side-channel) |
| 2026-05-16 | `internal/security/{password,redact}.go` | F030 (file-perms exact-equality) |
| 2026-05-16 | `internal/runlog`, `internal/fleet/transport`, `internal/lockfile`, `internal/template` | none (all clean) |

## Cross-cutting themes / patterns to track

Updated as the review uncovers patterns.

- **Spec-16 migration incomplete in `service` and `tool`** (F003,
  F006). Same shape as the arch-wins `copy` / `file` cleanup —
  every handler that still has `Execute`/`DryRun` is technical
  debt of the same kind. Audit remaining: `internal/actions/{copy,
  file, service, tool, ...}` grep `func \(.*\) Execute\(`.
- **HTTP calls without timeouts/context** (F007, F012, F014).
  Now confirmed in 9 packages. F012 proposes the cross-cutting
  fix; F014 documents the at-call-site fix for `fleet.Apply`'s
  WithoutCancel pattern.
- **Stale doc-strings track action / field counts** (F002 in
  CLAUDE.md, F013 in config.go). Pattern: pin the number → it
  drifts within the next sprint. Lean on `make budget-status`
  and `make handler-list` (if it exists; if not, worth adding).
- **Stale `//nolint:gocyclo` directives** (F017 adjacent obs).
  Functions that were over the cap and got extracted no longer
  need the suppression — but it stays in the code. Quick audit:
  `grep -rn 'nolint:gocyclo' internal/ | xargs -I{} ...`.
- **Cancellation / cleanup invariants in agentd worker** (F015,
  F016). The pattern "every exit path of executeRun must run the
  same cleanup" isn't enforced — F015 found one missed close.
  Defer-based cleanup is the fix in both cases.
- **Unbounded buffer / scanner sizes** (F018). `bufio.Scanner`
  with default 64 KB max, `bytes.Buffer` with no cap. Pattern
  recurs in any subprocess-output capture path; audit:
  - `internal/actions/shell/handler.go` (F018)
  - `internal/actions/assert/handler.go` (HTTP body)
  - `internal/actions/observe_logs/handler.go`
  - `internal/actions/wait_command/handler.go`
- **Reflection-walker coverage gaps** (F019). Walker handles a
  closed set of kinds; future kinds (interface{}, time.Time,
  custom types) silently pass through. A "verification walk" at
  the end of Resolve() would catch missed markers as a hard error.
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
