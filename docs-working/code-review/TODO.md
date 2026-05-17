# Code Review TODO

Living queue. Each iteration consumes one entry from **In progress /
Queue**, produces a finding (or several), and the queue updates.

## At-a-glance status (2026-05-18)

| | Count |
|---|---:|
| ✅ Findings filed and resolved (F001–F049, F036 skipped) | **48** |
| 🟡 Findings open / in progress | **0** |
| 📋 Packages still queued for review | see below |

**Queue clean**: F048 (fleet.yml strict YAML) shipped at `a6cb7741`;
F049 (pkg.Permissions manager-aware) shipped 2026-05-18. The
remaining work is *review*: read more packages, file new findings
if you spot smells. The companion manual-test queue at
`docs-working/archive/analysis/findings-2026-05-15/` is also closed
— see that folder's README.

## Findings index

| ID | Title | Severity | Status | Location |
|---|---|---|---|---|
| F001 | observe_disk Bsize cross-platform cast | risk | **done** | [findings/F001](./findings/F001-observe-disk-bsize-cast.md) |
| F002 | CLAUDE.md soft-cap list stale | doc | **done** | [findings/F002](./findings/F002-claude-md-soft-cap-list-stale.md) |
| F003 | service: legacy Execute/DryRun | smell | **done** | [findings/F003](./findings/F003-service-execute-dryrun-legacy-paths.md) |
| F004 | service: sudo/exec duplication in-package | smell | **done** | [findings/F004](./findings/F004-service-systemd-sudo-shell-duplication.md) |
| F005 | sudo -S shell-out helper cross-package | smell | **done** | [findings/F005](./findings/F005-sudo-shell-helper-cross-package.md) |
| F006 | tool handler legacy Execute/DryRun | smell | **done** | [findings/F006](./findings/F006-tool-handler-execute-dryrun-legacy.md) |
| F007 | tool: http no timeout / context | risk | **done** | [findings/F007](./findings/F007-tool-fetch-no-timeout-no-context.md) |
| F008 | tool.renderToolTemplates manual repetition | readability | **done** | [findings/F008](./findings/F008-tool-renderToolTemplates-manual-repetition.md) |
| F009 | explain.DisplayFacts section split | smell | **done** | [findings/F009](./findings/F009-explain-DisplayFacts-section-split.md) |
| F010 | explain TestDisplayFacts_NilFacts is dead | smell | **done** | [findings/F010](./findings/F010-explain-test-dead-nil-test.md) |
| F011 | cross-cutting: Execute/DryRun migration — all 21 handlers Run-only | smell | **done** | [findings/F011](./findings/F011-cross-cutting-execute-dryrun-spec16-incomplete.md) |
| F012 | cross-cutting: http no timeout (9 pkgs) | risk | **done** | [findings/F012](./findings/F012-cross-cutting-http-no-timeout.md) |
| F013 | config.Step stale "74" comment + Creates/Unless aliases | doc | **done** | [findings/F013](./findings/F013-config-step-stale-74-comment-and-alias-redundancy.md) |
| F014 | fleet.Apply WithoutCancel hangs Ctrl-C | risk | **done** | [findings/F014](./findings/F014-fleet-apply-context-withoutcancel-no-timeout.md) |
| F015 | agentd.Worker hub-close cleanup asymmetry | smell | **done** | [findings/F015](./findings/F015-agentd-worker-chdir-error-hub-leak.md) |
| F016 | agentd.Worker no-cancel context | risk | **done** | [findings/F016](./findings/F016-agentd-worker-context-background-no-cancel.md) |
| F017 | executor continue_on_error double emit | bug | **done** | [findings/F017](./findings/F017-executor-continue-on-error-double-emit.md) |
| F018 | shell scanner 64KB line cap | bug | **done** | [findings/F018](./findings/F018-shell-bufio-scanner-line-overflow.md) |
| F019 | secrets.Resolve misses step.Vars | bug | **done** | [findings/F019](./findings/F019-secrets-resolver-missing-vars-and-interface-maps.md) |
| F020 | apply.Runner os.Exit hostile to embedded callers | risk | **done** | [findings/F020](./findings/F020-apply-runner-os-exit-hostile-to-embedded-callers.md) |
| F021 | apply.Config.ExtraSubscribers doc-drift | doc | **done** | [findings/F021](./findings/F021-apply-config-extrasubscribers-doc-drift.md) |
| F022 | mcp uses NewTestLogger in production | smell | **done** | [findings/F022](./findings/F022-mcp-uses-NewTestLogger-in-production.md) |
| F023 | package handler swallows template-render errors | bug | **done** | [findings/F023](./findings/F023-package-handler-template-render-error-swallow.md) |
| F024 | planner walkAndRender misses map[string]interface{} | bug | **done** | [findings/F024](./findings/F024-planner-walkAndRender-missing-map-string-interface.md) |
| F025 | fleet.peerDiff misses Roles + SSH | bug | **done** | [findings/F025](./findings/F025-fleet-peerDiff-missing-roles-ssh-fields.md) |
| F026 | file/copy unbounded os.ReadFile in handler | risk | **done** | [findings/F026](./findings/F026-file-copy-unbounded-os-ReadFile-loads-entire-file-in-memory.md) |
| F027 | agentd self_upgrade sanityCheckBinary no-timeout | risk | **done** | [findings/F027](./findings/F027-agentd-self-upgrade-sanityCheckBinary-no-timeout.md) |
| F028 | git_clone askpass returns password for username prompt | bug | **done** | [findings/F028](./findings/F028-git-clone-askpass-returns-password-for-username-prompt.md) |
| F029 | agentd bearer-auth length side-channel | risk | **done** | [findings/F029](./findings/F029-agentd-bearerAuthMiddleware-length-side-channel.md) |
| F030 | security.FilePasswordProvider mode exact-equality | smell | **done** | [findings/F030](./findings/F030-security-FilePasswordProvider-rejects-more-restrictive-modes.md) |
| F031 | cmd/fleet.readToken no perms/insecure-flag check | smell | **done** | [findings/F031](./findings/F031-fleet-readToken-no-perms-check-no-insecure-flag-for-literal.md) |
| F032 | template/download legacy Execute shell injection | risk | **done** | [findings/F032](./findings/F032-template-download-legacy-shell-injection.md) |
| F033 | path-traversal validation silently ignored (11 sites) | bug | **done** | [findings/F033](./findings/F033-path-traversal-validation-silently-ignored.md) |
| F034 | pkg.repo gpg_key_fingerprint silently not verified | bug | **done** | [findings/F034](./findings/F034-pkg-repo-gpg-fingerprint-never-verified.md) |
| F035 | os.ssh_key silent chown failure | bug | **done** | [findings/F035](./findings/F035-os-ssh-key-silent-chown-failure.md) |
| F037 | vars action bypasses secrets resolver | bug | **done** | [findings/F037](./findings/F037-vars-action-bypasses-secrets-resolver.md) |
| F038 | shell line-overflow structured stream silent | bug | **done** | [findings/F038](./findings/F038-shell-line-overflow-structured-stream-silent.md) |
| F039 | pilot.RunLoop defer-in-loop + plan perms + silent save | smell | **done** | [findings/F039](./findings/F039-agent-loop-defer-in-for-loop-and-plan-perms.md) |
| F040 | llm.ClaudeClient timeout/model/body | smell | **done** | [findings/F040](./findings/F040-llm-claude-client-tight-timeout-stale-model-unbounded-body.md) |
| F041 | artifact_capture.readFileContent unbounded read | smell | **done** | [findings/F041](./findings/F041-artifact-capture-readFileContent-unbounded-read.md) |
| F042 | facts.Collect no ctx / per-cmd timeout | risk | **done** | [findings/F042](./findings/F042-facts-collect-no-context-no-per-cmd-timeout.md) |
| F043 | fleet init bearer-token prompt echoes to terminal | bug | **done** | [findings/F043](./findings/F043-fleet-init-token-prompt-echoes-to-terminal.md) |
| F048 | fleet machine manifest YAML non-strict | bug | **done** | [findings/F048](./findings/F048-fleet-machine-manifest-non-strict-yaml.md) |
| F049 | pkg.Permissions not manager-aware | bug | **done** | [findings/F049](./findings/F049-pkg-handler-permissions-not-manager-aware.md) |

## Still to review

Packages no reviewer has read in this pass. Absence of findings
≠ clean — just unread. Pick any entry, read it cold, file
`findings/F<NNN>-<slug>.md` if you spot something.

| # | Package / area | Notes |
|---|---|---|
| 1 | ~~`internal/fleet/machine.go`~~ | reviewed 2026-05-17 → F048 (non-strict YAML) |
| 2 | `internal/fleet/bootstrap_windows_target.go` | Windows-only; not exercised on Linux CI |
| 3 | `internal/agentd/{handlers,jsonl_sink,respond,config*,self_mac,self_shutdown*}.go` | not read in the original pass; `self_mac` + `self_shutdown*` are brand-new from the fleet WoL+shutdown work landing 2026-05-17 |
| 4 | `internal/presets/registry` (rest) | only `remote.go` covered (via F012); loader / validator / expander unread |
| 5 | `internal/actions/git_*` (except `git_clone`) | `git_checkout`, `git_config` — Reverse-capture pattern landed here, worth a read |
| 6 | `internal/actions/os_*` (except `service`, `systemd`, `ssh_key`) | `os_user`, `os_group`, `os_cron`, `os_mount`, `os_sysctl`, `os_firewall` — darwin parity just landed, skim for shared smells |
| 7 | `internal/actions/text_*` | `text_line`, `text_patch_ini`, `text_patch_yaml` (json done via F033) |
| 8 | `internal/actions/wait_*` (except `wait_http`, `wait_command`) | `wait_file`, `wait_port` |
| 9 | `internal/actions/windows_*` | `windows_firewall_rule`, `windows_scheduled_task` — shipped in spec-57, never reviewed |
| 10 | `cmd/*` (rest of CLI wiring) | ~10K LOC. Only `cmd/presets.go` + `cmd/fleet.go::readToken` spot-checked. Big files: `mooncake.go`, `fleet.go`, `step.go`, `tool.go` |
| 11 | Test-coverage gaps in churned packages | spec-66 wave 5, proposal-16 wave 3, R2.1c phase 2 — recently changed without tests catching up |

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
| 2026-05-16 | `internal/scaffold` | none (clean — atomic write, embed.FS, idempotent .gitignore) |
| 2026-05-16 | `internal/actions/wait_http`, `internal/actions/wait_command` | none (clean — proper ctx + timeouts) |
| 2026-05-16 | `cmd/fleet.go` (readToken) | F031 |
| 2026-05-16 | `internal/actions/{template,download}` legacy Execute | F032 (latent shell injection) |
| 2026-05-16 | `internal/actions/observe_logs` + `text_patch_json` + path-traversal audit | F033 |
| 2026-05-16 | `internal/actions/pkg_repo` | F034 (real silent security bypass) |
| 2026-05-16 | `internal/actions/os_ssh_key` | F035 (silent ownership failure) |
| 2026-05-16 | `internal/actions/container_image` | none locally (F016-family ctx.Background, already tracked) |
| 2026-05-16 | `internal/agent/loop.go` | F039 (defer-in-loop, 0644 plan files, silent save errors) |
| 2026-05-17 | `internal/fleet/machine.go` | F048 (non-strict YAML — fleet.yml silently accepts unknown fields) |

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
