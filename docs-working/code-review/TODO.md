# Code Review TODO

Living queue. Each iteration consumes one entry from **In progress /
Queue**, produces a finding (or several), and the queue updates.

> **Findings archive:** All resolved findings (F001–F051) have been
> moved to `docs-working/archive/code-review/findings/`. New findings
> go in that same folder; add a row to the index below.

## At-a-glance status (2026-05-27)

| | Count |
|---|---:|
| ✅ Findings filed and resolved (F001–F055, F036 skipped) | **54** |
| 🟡 Findings open (filed, not yet fixed) | **2** (F056, F057) |
| 📋 Packages still queued for review | **0 substantive** (1 broad follow-up — see below) |

**2026-05-27 cold-read pass closed the queue.** Eight package
areas read cold (agentd handlers + sinks + self-shutdown,
git_checkout/git_config, six os_* handlers, three text_* handlers,
wait_file/wait_port, two windows_* handlers, cmd/* CLI wiring,
fleet/bootstrap_windows_target). Two new findings filed:

- **F051** — cross-cutting `context.TODO()` in 11 os_* sites
  (mount/umount/ufw can hang on NFS or netfilter lock). Risk;
  F012/F016/F042 family. Filed open; fix is per-package `runCmd`
  helper with timeouts (option 1) or plumb ctx through
  `actions.Context` (option 2, bigger blast radius — defer).
- **F052** — `cmd/kernel/validate.go` has three direct `os.Exit`
  calls. Smell; F020 shape. Mechanical fix: replace with
  `cli.Exit(msg, code)` returns.

**2026-05-27 second cold-read** of `internal/executor/` (PICKUP
item #1: top under-reviewed candidate, 4,003 LOC, only
spot-checked previously). Read transaction.go, trycatch.go,
retry.go, finalize.go, inspect.go, preflight.go, dryrun.go,
scope.go, capture.go end-to-end; spot-checked `dispatchRunner` /
`dispatchPlanMode` / `postExecuteSuccess` / `handleStepError` in
executor.go. One finding filed:

- **F053** — `runWithRetry` uses uncancellable `time.Sleep`
  between attempts. Risk; F014/F016/F042/F051 family. Spec-69
  phase 2 promoted the retry loop out of shell's
  backoff.go, so every spec-69-migrated action (shell, command,
  download, http_request, package, os_user, os_cron,
  pkg_upgrade…) now blocks Ctrl-C / context cancel inside the
  delay window. With `backoff: exponential` the blocked window
  scales into many minutes. Fix is to plumb ctx into
  `runWithRetry` and replace `time.Sleep` with
  `select { <-timer.C; <-ctx.Done() }`.

Adjacent observations from this pass:

- ~~**scope.Clone() omits `ApplyStartedAt`.**~~ **Fixed** (2026-05-27,
  same-day cleanup PR). `VariableScope.Clone()` now propagates the
  field; new `TestScopeClone_PropagatesApplyStartedAt` pins it.
- ~~**Inconsistent nil-guards on `ec.Svc.Stats.{Changed,Executed,Global}`.**~~
  **Fixed** (2026-05-27, same-day cleanup PR). All 12 Stats deref
  sites in `executor.go` + `transaction.go` now route through
  `incStat` / `decStat` / `readStat` helpers in `context.go` —
  uniformly nil-safe, no per-site guards needed. New
  `TestStatHelpers_NilSafe` pins the contract (nil pointers are
  no-ops, not panics; decStat clamps at zero per MT-45).
- ~~**Reverse-step dispatch bypasses `step.started` / `step.completed`
  events.**~~ **Filed as F054 + fixed** (2026-05-27, hybrid scope).
  Investigation found spec-30 §"Key files" promised a six-event
  rollback surface that was never implemented (zero hits for
  `transaction_step_reversed` etc. in `internal/`). Closed by adding
  four event types (RollbackBegin / StepReversed / RollbackComplete
  / RollbackFailed) emitted at boundaries in `handleTxnBodyFailure`
  + a `Reverted` flag flowing from `RunCapture.markStepReverted` →
  `executor.StepRecord.Reverted` → `runlog.StepEntry.Reverted`.
  TransactionBegin / TransactionCommit deferred — needs compound-
  parent step.started semantics the executor doesn't expose today
  (transaction parents aren't dispatched; only their children are).
  Tracked under "Cross-cutting themes" below.
- ~~**`runWithRetry` error message hard-codes "command failed".**~~
  **Fixed** as part of F053 (commit `c6bd27ac`). Message is now
  action-agnostic ("step failed after N attempts").

**Round-2 cold-read** (2026-05-27 PM) covered the remaining
executor surface: errors.go, reverse_registry.go, result.go's
ReverseData wire-encoding tail, context.go's Privileged /
Performer / actions.Context impl, and executor.go's
checkSkipConditions / checkIdempotencyConditions /
handleWhenExpression / ExecuteStep / ExecuteSteps /
DispatchStepAction / Start / AddGlobalVariables /
getStepDisplayName / emitStepSkipped. One finding filed:

- ~~**F055** — `checkIdempotencyConditions` runs `unless:` shell-
  outs without ctx/timeout.~~ **Fixed.** Now uses
  `exec.CommandContext` with `ec.Svc.Ctx` + a 10s per-guard hard
  cap. Two regression tests in `cancel_test.go` pin both paths.

Adjacent observations (all closed in the same PR as F055 except
#3, which was reverted as out-of-scope):

- ~~**Redundant outer nil-guards around incStat calls.**~~
  **Fixed.** All 4 sites cleaned: `emitStepSkipped`,
  `handleStepError`, `postExecuteSuccess` (Executed + Changed),
  and the ExecuteStep global-counter site. The `incStat`/
  `decStat`/`readStat` helpers in `context.go` are nil-safe;
  the outer guards were dead defensive code from before the
  F053 centralization.
- ~~**`ec.Svc` deref consistency.**~~ **Fixed.** Dropped the
  `if ec.Svc != nil` guard from `MergeUserVars` to match the
  10+ peer accessors that deref unconditionally. Svc is
  always non-nil in production (Start / executePlanWithCapture
  always fills it).
- **Five dead `//nolint:unused` functions** (`markStepFailed`,
  `handleVars`, `shouldSkipByTags`, `parseFileMode`, plus the
  re-exports in `export_test.go`). **Reverted as out-of-scope**
  for this PR. Initial scope wanted to delete them, but each is
  test-only — production callers are gone but `executor_test.go`
  still exercises them (`TestHandleVars`, `TestParseFileMode`,
  `TestShouldSkipByTags`, `TestMarkStepFailed`). Removing the
  functions requires removing the tests too, which is a behavior-
  loss decision worth a separate PR with explicit operator buy-in
  ("are these tests preserving knowledge we want to keep, or
  archeology?"). Logged as a TODO under "Cross-cutting themes"
  below.
- ~~**Vars-file read errors silently swallowed.**~~ **Fixed.**
  `executor.go:967` now logs failures at `log.Infof("[WARNING] ...")`
  (was `Debugf`). `continue` semantics preserved — failing the
  run hard would be too aggressive for the agentd payload race-
  condition case where a mid-deploy worker might not see a
  freshly-published file yet.

Everything else surfaced by the cold-read pass fell out as
documented design intent (self_shutdown delay goroutine,
files_handler fsync swallow, apply SIGINT hard-exit per F016
follow-up, agentd Windows TCP-loopback default per spec-49) or
false-positive (text_patch_yaml "nil derefs" guarded by walk
invariant, git_config exit-1 sentinel actually correct,
cmd/mooncake `TrimLeft` edge guarded by `=` check, cmd/agentd
`absPath` inside the bearer-auth boundary). All reviewed areas
recorded in the table below.

**Plan + template cold-read** (2026-05-27 PM, follow-on to the
round-2 executor pass — closes the last under-reviewed core
packages from PICKUP item #1). Read `internal/plan/{plan,
integrity,io,validate,planner_transaction,planner_trycatch}.go`
end-to-end (~700 LOC) + `internal/template/{renderer,
listresolver}.go` end-to-end (~620 LOC). Two findings filed:

- **F056** — `plan.SavePlanToFile` writes via bare `os.Create`,
  resulting in 0644 plan files under the typical umask 022.
  F037 family. Plans carry secret refs + the full playbook
  structure; on a multi-user host the contents leak. Local
  fix: swap to `os.OpenFile(..., 0o600)`. Same shape as the
  pilot.RunLoop perms fix from 2026-05-22.
- **F057** — `Pongo2Renderer.Render` acquires its per-receiver
  `r.mu` before `pongo2.FromString`, but `FromString` mutates
  pongo2's package-level `DefaultSet`. The mutex serializes
  one renderer's calls but doesn't synchronise two distinct
  renderers' calls against the shared global. Latent race —
  unreachable today under the single-worker FIFO invariant
  (agentd/worker.go:14-17), but the mutex's presence is
  misleading: a future maintainer adding parallelism would
  see the lock, conclude "handled," and ship a race. Fix
  options A (package-level mutex, 1-line change) and B
  (per-renderer TemplateSet, larger but cleaner) documented
  in the finding.

Adjacent observations from the plan + template pass (smells,
not standalone findings):

- **plan.io.go yaml.Unmarshal is non-strict.** `LoadPlanFromFile`
  at `internal/plan/io.go:130` decodes a plan via
  `yaml.Unmarshal` with no KnownFields setting. F048 family
  (`fleet.machine` non-strict YAML, fixed 2026-05-17). A
  user-edited plan file with a typo'd field gets silently
  dropped on read; the apply proceeds with subtly different
  behavior than the operator typed.
- **plan.io.go partial-write on failure.** `SavePlanToFile` at
  `internal/plan/io.go:58` writes directly via `file.Write`
  (no temp-file + atomic rename). A write that errors midway
  leaves a partial/empty plan file at the destination.
  `apply --from-plan` against that path would error with a
  decode failure, but a `cat` would show truncated content.
  Atomic-write pattern (write to temp, then rename) would
  match the convention used by `internal/agentd/store.go`.
- **validate.go AllowStale silent demotion.** Each stale-check
  branch in `ValidateForApply` (host mismatch, hash mismatch,
  file missing, age exceeded) has an `if !opts.AllowStale {
  return err }` shape. When AllowStale is true, each error is
  just swallowed — the caller has no signal which checks
  actually fired. The docstring says "caller is responsible
  for logging the reasons separately if desired" but provides
  no API surface to do so. Operators running `--allow-stale`
  want the override AND the explanation. Easy fix: return
  `[]StaleReason` alongside the nil-when-allowed error path,
  or accept a callback.

The companion manual-test queue at
`docs-working/archive/analysis/findings-2026-05-15/` remains
closed — see that folder's README.

## Findings index

| ID | Title | Severity | Status | Location |
|---|---|---|---|---|
| F001 | observe_disk Bsize cross-platform cast | risk | **done** | [findings/F001](../archive/code-review/findings/F001-observe-disk-bsize-cast.md) |
| F002 | CLAUDE.md soft-cap list stale | doc | **done** | [findings/F002](../archive/code-review/findings/F002-claude-md-soft-cap-list-stale.md) |
| F003 | service: legacy Execute/DryRun | smell | **done** | [findings/F003](../archive/code-review/findings/F003-service-execute-dryrun-legacy-paths.md) |
| F004 | service: sudo/exec duplication in-package | smell | **done** | [findings/F004](../archive/code-review/findings/F004-service-systemd-sudo-shell-duplication.md) |
| F005 | sudo -S shell-out helper cross-package | smell | **done** | [findings/F005](../archive/code-review/findings/F005-sudo-shell-helper-cross-package.md) |
| F006 | tool handler legacy Execute/DryRun | smell | **done** | [findings/F006](../archive/code-review/findings/F006-tool-handler-execute-dryrun-legacy.md) |
| F007 | tool: http no timeout / context | risk | **done** | [findings/F007](../archive/code-review/findings/F007-tool-fetch-no-timeout-no-context.md) |
| F008 | tool.renderToolTemplates manual repetition | readability | **done** | [findings/F008](../archive/code-review/findings/F008-tool-renderToolTemplates-manual-repetition.md) |
| F009 | explain.DisplayFacts section split | smell | **done** | [findings/F009](../archive/code-review/findings/F009-explain-DisplayFacts-section-split.md) |
| F010 | explain TestDisplayFacts_NilFacts is dead | smell | **done** | [findings/F010](../archive/code-review/findings/F010-explain-test-dead-nil-test.md) |
| F011 | cross-cutting: Execute/DryRun migration — all 21 handlers Run-only | smell | **done** | [findings/F011](../archive/code-review/findings/F011-cross-cutting-execute-dryrun-spec16-incomplete.md) |
| F012 | cross-cutting: http no timeout (9 pkgs) | risk | **done** | [findings/F012](../archive/code-review/findings/F012-cross-cutting-http-no-timeout.md) |
| F013 | config.Step stale "74" comment + Creates/Unless aliases | doc | **done** | [findings/F013](../archive/code-review/findings/F013-config-step-stale-74-comment-and-alias-redundancy.md) |
| F014 | fleet.Apply WithoutCancel hangs Ctrl-C | risk | **done** | [findings/F014](../archive/code-review/findings/F014-fleet-apply-context-withoutcancel-no-timeout.md) |
| F015 | agentd.Worker hub-close cleanup asymmetry | smell | **done** | [findings/F015](../archive/code-review/findings/F015-agentd-worker-chdir-error-hub-leak.md) |
| F016 | agentd.Worker no-cancel context | risk | **done** | [findings/F016](../archive/code-review/findings/F016-agentd-worker-context-background-no-cancel.md) |
| F017 | executor continue_on_error double emit | bug | **done** | [findings/F017](../archive/code-review/findings/F017-executor-continue-on-error-double-emit.md) |
| F018 | shell scanner 64KB line cap | bug | **done** | [findings/F018](../archive/code-review/findings/F018-shell-bufio-scanner-line-overflow.md) |
| F019 | secrets.Resolve misses step.Vars | bug | **done** | [findings/F019](../archive/code-review/findings/F019-secrets-resolver-missing-vars-and-interface-maps.md) |
| F020 | apply.Runner os.Exit hostile to embedded callers | risk | **done** | [findings/F020](../archive/code-review/findings/F020-apply-runner-os-exit-hostile-to-embedded-callers.md) |
| F021 | apply.Config.ExtraSubscribers doc-drift | doc | **done** | [findings/F021](../archive/code-review/findings/F021-apply-config-extrasubscribers-doc-drift.md) |
| F022 | mcp uses NewTestLogger in production | smell | **done** | [findings/F022](../archive/code-review/findings/F022-mcp-uses-NewTestLogger-in-production.md) |
| F023 | package handler swallows template-render errors | bug | **done** | [findings/F023](../archive/code-review/findings/F023-package-handler-template-render-error-swallow.md) |
| F024 | planner walkAndRender misses map[string]interface{} | bug | **done** | [findings/F024](../archive/code-review/findings/F024-planner-walkAndRender-missing-map-string-interface.md) |
| F025 | fleet.peerDiff misses Roles + SSH | bug | **done** | [findings/F025](../archive/code-review/findings/F025-fleet-peerDiff-missing-roles-ssh-fields.md) |
| F026 | file/copy unbounded os.ReadFile in handler | risk | **done** | [findings/F026](../archive/code-review/findings/F026-file-copy-unbounded-os-ReadFile-loads-entire-file-in-memory.md) |
| F027 | agentd self_upgrade sanityCheckBinary no-timeout | risk | **done** | [findings/F027](../archive/code-review/findings/F027-agentd-self-upgrade-sanityCheckBinary-no-timeout.md) |
| F028 | git_clone askpass returns password for username prompt | bug | **done** | [findings/F028](../archive/code-review/findings/F028-git-clone-askpass-returns-password-for-username-prompt.md) |
| F029 | agentd bearer-auth length side-channel | risk | **done** | [findings/F029](../archive/code-review/findings/F029-agentd-bearerAuthMiddleware-length-side-channel.md) |
| F030 | security.FilePasswordProvider mode exact-equality | smell | **done** | [findings/F030](../archive/code-review/findings/F030-security-FilePasswordProvider-rejects-more-restrictive-modes.md) |
| F031 | cmd/fleet.readToken no perms/insecure-flag check | smell | **done** | [findings/F031](../archive/code-review/findings/F031-fleet-readToken-no-perms-check-no-insecure-flag-for-literal.md) |
| F032 | template/download legacy Execute shell injection | risk | **done** | [findings/F032](../archive/code-review/findings/F032-template-download-legacy-shell-injection.md) |
| F033 | path-traversal validation silently ignored (11 sites) | bug | **done** | [findings/F033](../archive/code-review/findings/F033-path-traversal-validation-silently-ignored.md) |
| F034 | pkg.repo gpg_key_fingerprint silently not verified | bug | **done** | [findings/F034](../archive/code-review/findings/F034-pkg-repo-gpg-fingerprint-never-verified.md) |
| F035 | os.ssh_key silent chown failure | bug | **done** | [findings/F035](../archive/code-review/findings/F035-os-ssh-key-silent-chown-failure.md) |
| F037 | vars action bypasses secrets resolver | bug | **done** | [findings/F037](../archive/code-review/findings/F037-vars-action-bypasses-secrets-resolver.md) |
| F038 | shell line-overflow structured stream silent | bug | **done** | [findings/F038](../archive/code-review/findings/F038-shell-line-overflow-structured-stream-silent.md) |
| F039 | pilot.RunLoop defer-in-loop + plan perms + silent save | smell | **done** | [findings/F039](../archive/code-review/findings/F039-agent-loop-defer-in-for-loop-and-plan-perms.md) |
| F040 | llm.ClaudeClient timeout/model/body | smell | **done** | [findings/F040](../archive/code-review/findings/F040-llm-claude-client-tight-timeout-stale-model-unbounded-body.md) |
| F041 | artifact_capture.readFileContent unbounded read | smell | **done** | [findings/F041](../archive/code-review/findings/F041-artifact-capture-readFileContent-unbounded-read.md) |
| F042 | facts.Collect no ctx / per-cmd timeout | risk | **done** | [findings/F042](../archive/code-review/findings/F042-facts-collect-no-context-no-per-cmd-timeout.md) |
| F043 | fleet init bearer-token prompt echoes to terminal | bug | **done** | [findings/F043](../archive/code-review/findings/F043-fleet-init-token-prompt-echoes-to-terminal.md) |
| F048 | fleet machine manifest YAML non-strict | bug | **done** | [findings/F048](../archive/code-review/findings/F048-fleet-machine-manifest-non-strict-yaml.md) |
| F049 | pkg.Permissions not manager-aware | bug | **done** | [findings/F049](../archive/code-review/findings/F049-pkg-handler-permissions-not-manager-aware.md) |
| F050 | preset fetch unbounded body | risk | **done** | [findings/F050](../archive/code-review/findings/F050-preset-fetch-unbounded-body.md) |
| F051 | os_* handlers context.TODO() (11 sites) | risk | **done** | [findings/F051](../archive/code-review/findings/F051-os-handlers-context-todo-cross-cutting.md) |
| F052 | cmd/kernel/validate.go os.Exit (3 sites) | smell | **done** | [findings/F052](../archive/code-review/findings/F052-kernel-validate-os-exit-hostile-to-callers.md) |
| F053 | executor.runWithRetry time.Sleep not cancellable | risk | **done** | [findings/F053](../archive/code-review/findings/F053-executor-retry-sleep-not-cancellable.md) |
| F054 | spec-30 rollback events never implemented | smell | **done** | [findings/F054](../archive/code-review/findings/F054-rollback-events-never-implemented.md) |
| F055 | executor `unless:` runs without ctx/timeout | risk | **done** | [findings/F055](../archive/code-review/findings/F055-idempotency-unless-no-ctx-no-timeout.md) |
| F056 | plan.SavePlanToFile uses default umask (0644 on shared hosts) | smell | **open** | [findings/F056](../archive/code-review/findings/F056-plan-io-default-umask-perms.md) |
| F057 | Pongo2Renderer per-renderer mutex misses pongo2's global TemplateSet | smell | **open** | [findings/F057](../archive/code-review/findings/F057-pongo2-per-renderer-mutex-misses-global-state.md) |

## Still to review

Cold-read queue **cleared on 2026-05-27**. One broad follow-up
remains; pick it up when a stream is between specs:

| # | Package / area | Notes |
|---|---|---|
| 1 | Test-coverage gaps in churned packages | spec-66 wave 5, proposal-16 wave 3, R2.1c phase 2 — recently changed without tests catching up. Scope this with the user before starting; it's a test-writing pass, not a review pass. |

The previous 10-row queue is fully consumed:

- ~~`internal/fleet/machine.go`~~ → F048 (2026-05-17)
- ~~`internal/fleet/bootstrap_windows_target.go`~~ → reviewed 2026-05-27, clean
- ~~`internal/agentd/{handlers,jsonl_sink,respond,config*,self_mac,self_shutdown*}.go`~~ → reviewed 2026-05-27, clean (self_shutdown delay-goroutine race is documented design intent)
- ~~`internal/presets/registry`~~ → **deleted in 4db53ad6** (orphan package, presets-CLI retirement)
- ~~`internal/actions/git_checkout`, `git_config`~~ → reviewed 2026-05-27, clean (reverse-capture pattern sound)
- ~~`internal/actions/{os_user,os_group,os_cron,os_mount,os_sysctl,os_firewall}`~~ → reviewed 2026-05-27 → F051 (cross-cutting ctx.TODO)
- ~~`internal/actions/{text_line,text_patch_ini,text_patch_yaml}`~~ → reviewed 2026-05-27, clean (writeAtomic duplicated 3x is minor; the suspected nil-derefs in text_patch_yaml are guarded by walk's invariant)
- ~~`internal/actions/{wait_file,wait_port}`~~ → reviewed 2026-05-27, clean
- ~~`internal/actions/{windows_firewall_rule,windows_scheduled_task}`~~ → reviewed 2026-05-27, clean (proper psQuote/xmlEscape/-EncodedCommand defense-in-depth)
- ~~`cmd/*` (rest)~~ → reviewed 2026-05-27 → F052 (validate.go os.Exit)

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
| 2026-05-27 | `internal/agentd/{handlers,jsonl_sink,respond,config*,self_mac,self_shutdown*}` | none (clean — bearer-auth and shutdown semantics sound) |
| 2026-05-27 | `internal/fleet/bootstrap_windows_target.go` | none (clean — psQuote / xmlEscape / -EncodedCommand defense, 15s timeout on critical step) |
| 2026-05-27 | `internal/actions/git_checkout`, `internal/actions/git_config` | none (clean — reverse-capture pattern correct, git invoked via argv) |
| 2026-05-27 | `internal/actions/{os_user,os_group,os_cron,os_mount,os_sysctl,os_firewall}` | F051 (cross-cutting context.TODO() — mount/umount/ufw can hang on NFS or netfilter lock) |
| 2026-05-27 | `internal/actions/{text_line,text_patch_ini,text_patch_yaml}` | none (clean — suspected nil-derefs guarded by walk invariant; writeAtomic duplication is minor) |
| 2026-05-27 | `internal/actions/{wait_file,wait_port}` | none (clean — Ticker + ctx-aware select, dial timeout bounded) |
| 2026-05-27 | `internal/actions/{windows_firewall_rule,windows_scheduled_task}` | none (clean — proper PowerShell escaping + base64-encoded command + idempotent delete-and-recreate) |
| 2026-05-27 | `cmd/mooncake.go` + `cmd/{kernel,fleet,step,tool,agentd}` | F052 (kernel/validate.go os.Exit — F020 shape) |
| 2026-05-27 | `internal/executor/{transaction,trycatch,retry,finalize,inspect,preflight,dryrun,scope,capture}.go` + `executor.go` spot-check (`dispatchRunner`, `dispatchPlanMode`, `postExecuteSuccess`, `handleStepError`) | F053 (`runWithRetry time.Sleep` uncancellable). Three adjacent smells noted inline (scope.Clone omits ApplyStartedAt; Stats nil-guards inconsistent across dispatchRunner vs postExecuteSuccess; reverse-step dispatch bypasses step.started/completed events) — not standalone findings, see "at-a-glance status" block above. |
| 2026-05-27 | **Round 2:** `internal/executor/{errors,reverse_registry}.go` + `result.go` ReverseData wire-encoding tail + `context.go` tail (Privileged / Performer / actions.Context impl) + `executor.go` end-to-end (`checkIdempotencyConditions`, `checkSkipConditions`, `handleWhenExpression`, `ExecuteStep`, `ExecuteSteps`, `DispatchStepAction`, `Start`, `AddGlobalVariables`, `getStepDisplayName`, `emitStepSkipped`) | F055 (idempotency `unless:` runs without ctx/timeout — same F051/F053 family). Four adjacent observations noted inline below (vars-file silent-swallow; redundant outer nil-guards × 5 sites; handleVars dead code; ec.Svc nil-guard inconsistency). |
| 2026-05-27 | `internal/plan/{plan,integrity,io,validate,planner_transaction,planner_trycatch}.go` end-to-end (~700 LOC) | F056 (plan-file default umask perms — F037 shape). Three adjacent observations: plan.io.go yaml.Unmarshal non-strict (F048 shape); plan.io.go partial-write on `os.Create` failure leaves zero/garbage file; validate.go AllowStale silent demotion — caller has no signal which checks fired. |
| 2026-05-27 | `internal/template/{renderer,listresolver}.go` end-to-end (~620 LOC) | F057 (pongo2 per-renderer mutex doesn't protect the global TemplateSet — latent race, unreachable today under single-worker invariant; reaches the moment anything parallelises). |

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
- **Blocking calls that don't watch `ctx.Done()`** (F014, F016,
  F042, F051, **F053**). The family is wider than HTTP; any
  `time.Sleep` / `exec.Command` (without `CommandContext`) /
  pipe-read without ctx is a latent UX cliff. F053 (executor
  retry sleeper) is the latest. The pattern audit: `grep -rn
  'time\.Sleep' internal/` outside `_test.go` returns ~15 sites;
  most are fine (short fixed sleeps in retry-with-cap helpers
  for facts/observe), but each should justify why it doesn't
  need ctx-awareness.
- **Compound-parent lifecycle events missing** (followup to
  F054). Transaction parents (and try-block parents from
  spec-23) don't dispatch — only their children do — so there's
  no natural emission point for `transaction.begin` /
  `transaction.commit` / `try.begin` / `try.commit` events.
  Spec-30 designed for begin/commit; F054 implemented the
  rollback four and left begin/commit for a separate scope.
  Real fix is a compound-parent lifecycle hook in the dispatch
  loop, not a per-spec event-set bolt-on. Defer until at least
  one more compound shape (transaction + try) makes the
  generic hook obviously worth designing.
- **Test-only `//nolint:unused` helpers in `internal/executor/
  executor.go`** (5 functions): `markStepFailed`, `handleVars`,
  `shouldSkipByTags`, `parseFileMode`, plus the re-exports in
  `export_test.go`. Production callers are gone (replaced by
  registered action handlers, planner-side tag filtering,
  in-handler file-mode parsing), but `executor_test.go` still
  exercises each via the export_test seam. Either delete the
  functions + their dedicated tests (preferred — they test
  obsolete behavior that no production code path can reach),
  or document why they're held for a future restoration.
  Decision is operator-side: are the tests preserving knowledge
  we want to keep, or archaeology? Cleanup deferred from the
  F055 cleanups PR (2026-05-27) because the deletion turned out
  to require coordinated test removal — bigger blast radius
  than the same-PR cleanups absorbed.
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
