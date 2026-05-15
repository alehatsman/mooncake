# docs-working

Working documents — specs, epics, and notes. Not canonical docs; those live in `docs/`.

## Structure

| Path | Contents |
|---|---|
| `streams.md` | Stream overview — goals, audiences, dependencies, what to work on next |
| `specs/action-surface/` | Stream 1: typed action library (spec-22, 24–28, 32, 36, disk-partition) |
| `specs/personal-fleet/` | Stream 3: personal fleet runtime (spec-55, 58, implementation-order) |
| `specs/ecosystem/` | Stream 5: plugins, marketplace, integrations (spec-31) |
| `specs/done/` | Shipped specs — read-only reference |
| `epics/` | Epic-level planning docs |
| `analysis/` | Code quality audits, one-off investigations |
| `deferred/` | Deferred notes not yet specced |

See **[streams.md](./streams.md)** for the full stream breakdown, dependencies, and recommended work order.
See **[action-design-principles.md](./action-design-principles.md)** for the 11 non-negotiable design rules for any new action.
See **[non-goals.md](./non-goals.md)** for the seven things Mooncake will not become — check every proposed feature against this list.

## Active specs by stream

**Stream 1 — Action Surface**
22 extended-handler-abi (phase 8 docs remains) · 24 pkg-surface (P6 ABI hooks) ·
25 text-surface (P5 ABI hooks) · 26 git-actions (ABI hooks) · 27 os-identity (ABI hooks) ·
28 os-scheduling (non-ufw drivers, deferred) · 32 step-action-dispatch ·
36 windows-support · disk-partition-action (exploration)

Phases 1–7 of spec-22 are complete (four-method ABI declared + MCP wiring landed).
Remaining: phase 8 docs, plus final ABI-hook phases for 24–27.
**The full observe.* family shipped** — 9 typed handlers (port, process, http,
service, cpu, memory, disk, gpu, logs) plus cross-peer fan-out via
`mooncake fleet observe`. Specs 59–64 all moved to `done/`. Only spec-59
phase 6 (`--inspect-real` CLI flag) remains as a small follow-up.

**Stream 2 — Safe Agent Runtime**

spec-23 framework-primitives ✅ (all three sections shipped) and
spec-30 transactions ✅ both moved to done. What remains in this stream
lives under Stream 1 (spec-22 phases 7–8).

**Stream 3 — Fleet & Cluster Management**
55 fleet-doctor (fan-out wrapper) · 58 fleet-drift

Fleet is **14/14 PRs** — original plan complete (PR13 interactive `fleet init`
shipped `a43db9c`/`f3d64c9` on 2026-05-15). Specs 45 / 52 / 53 / 54 moved to
`done/`. Spec 55 (fan-out doctor) is drafted — the single-host probe ladder
has already shipped. Spec 58 (drift detection) is the highest-leverage
candidate from GitHub issue #11 — see `clustermanagement/issue-11-analysis.md`.

GitHub issue #8 (ChangeGraph as core primitive) audited against current
state in `analysis/issue-8-changegraph-analysis.md` — ~60% already shipped
under spec-22 / spec-30 / spec-58; the genuinely-new bets are `observe.*`,
`explain`, `discover`, `rewind`.

**Stream 4 — Developer Experience**
*(shipped: spec-39 init, 40 config-discovery, 41 doctor, 42 recommend)*

**Stream 5 — Ecosystem**
31 tier2-plugin-model

**Bugs**

- [#12](https://github.com/alehatsman/mooncake/issues/12) Cross-device EXDEV in `fleet upgrade` linux replace ([analysis](analysis/issue-12-fleet-upgrade-exdev-rename.md)) — `os.Rename` from `/var/lib/.../upgrade/` to `/usr/local/bin/` fails on WSL / multi-fs hosts. Fall back to copy+remove on EXDEV.
- [#13](https://github.com/alehatsman/mooncake/issues/13) `windows.firewall_rule` corrupts non-ASCII via ConvertTo-Json ([analysis](analysis/issue-13-windows-firewall-utf8-encoding.md)) — PowerShell's default OEM codepage maps non-ASCII bytes to `0x1a` on stdout, breaking the action's drift-detection query. Force UTF-8 output in `realPSRun`.
- [#14](https://github.com/alehatsman/mooncake/issues/14) `windows.scheduled_task` drift detection unstable ([analysis](analysis/issue-14-scheduled-task-drift-unstable.md)) — Task Scheduler injects schema-defaulted elements on register that `NormaliseTaskXML` doesn't strip; plan always reports "would update" even after a clean apply.
- [#15](https://github.com/alehatsman/mooncake/issues/15) `fleet apply` fails on non-regular files in plan-dir ([analysis](analysis/issue-15-fleet-apply-plan-dir-walk.md)) — Plan-dir sync refuses to walk past a socket / FIFO; `fleet apply /tmp/x.yml` chokes on `/tmp/.X11-unix/X0`. Skip non-regular files in the walker.
- [#16](https://github.com/alehatsman/mooncake/issues/16) `fleet exec --timeout` escaped by shell-compound commands ([analysis](analysis/issue-16-fleet-exec-timeout-process-group.md)) — `--timeout 2s 'sleep 30'` kills correctly; `--timeout 2s 'sleep 30; echo done'` runs the full 30s. Kernel SIGKILLs just the shell, not the process group. Fix: `Setpgid` + kill -pid.
- [#17](https://github.com/alehatsman/mooncake/issues/17) `register:` captured outputs invisible to template renders ([analysis](analysis/issue-17-register-captures-not-visible.md)) — Documented `register: r` + `{{ r.stdout }}` pattern from `docs-next/examples/actions/shell.yml` renders empty. Affects local `apply` and `fleet apply` both. Breaks every spec-59 `observe.*` consumer + LLM-agent workflows.
- [#18](https://github.com/alehatsman/mooncake/issues/18) `observe.http` silently follows redirects with no opt-out ([analysis](analysis/issue-18-observe-http-redirect.md)) — `--expect-status 301` against a 301-returning URL reports 200/FOUND=false because the cmdlet follows the redirect transparently. Common ops checks (HTTP→HTTPS verify, migration redirect verify) impossible without dropping to `shell: curl -I`. Add `follow_redirects` (default 10; 0=off) + `capture_redirect_chain`.
- [#19](https://github.com/alehatsman/mooncake/issues/19) `fleet upgrade` linux still fails after #12: `ProtectSystem` blocks writes to `/usr/local/bin` ([analysis](analysis/issue-19-fleet-upgrade-protectsystem-blocks-bin.md)) — EXDEV fallback works but hits EROFS because the embedded systemd unit has `ProtectSystem=true` and no `ReadWritePaths=/usr/local/bin`. Add the whitelist.
- [#20](https://github.com/alehatsman/mooncake/issues/20) `for_each: [list]` inline form rejected by schema ([analysis](analysis/issue-20-for-each-list-form-rejected.md)) — JSON schema declares `for_each` as string-only; Go struct accepts both string and sequence forms. Validator error message is empty-parens fallback. Also: `with_items` is documented in 5+ places but doesn't exist on Step.
- [#21](https://github.com/alehatsman/mooncake/issues/21) `failed_when` fabricates "exit code 1" on actually-succeeded commands ([analysis](analysis/issue-21-failed-when-fabricates-exit-code.md)) — when `failed_when: "true"` fires on a clean exit-0 command, the error reads "command failed with exit code 1" but the underlying command exited 0 (stdout captured fine). Misattributes the failure source. Also: `continue_on_error` renders the same step with both `✗` and `~` markers.
- [#23](https://github.com/alehatsman/mooncake/issues/23) `continue_on_error: true` on a `try` compound step doesn't compose ([analysis](analysis/issue-23-continue-on-error-doesnt-compose-with-try.md)) — the universal `continue_on_error` field is silently ignored when set on a try/catch/transaction compound. Operators can't "run cleanup on deploy failure but don't halt the playbook" without breaking catch.

## Shipped specs (specs/done/)

01 run-recap · 02 skip-reasons · 03 agent-jsonl · 04 snapshot · 05 fact-query ·
06 quiet-mode · 07 step-display · 08 run-history · 09 structured-errors ·
10 mcp-server · 11 preset-registry · 12 package-summary · 13 single-step ·
14 snapshot-diff · 15 check-mode · 16 unify-dryrun-execute ·
17 package-batch-and-template · 18 mooncake-agent-daemon · 19 tool-action ·
20 metrics · 21 modernization-cutover ·
23 framework-primitives · 29 wait-primitives · 30 transactions ·
33 execution-context-split · 34 typed-variable-context · 35 plan-diff ·
37 step-output-capture · 38 read-json-yaml ·
39 mooncake-init · 40 config-discovery-and-dry-run · 41 mooncake-doctor ·
42 onboarding-and-recommend · 43 fleet-transport-and-sync ·
44 ssh-bootstrap-transport · 45 fleet-discovery · 46 fleet-status-and-logs ·
47 fleet-bootstrap-ux · 48 per-host-overlays-and-tags · 49 agentd-on-windows ·
50 extended-filter-keys · 51 local-apply-overlay-parity ·
52 fleet-exec · 53 fleet-watch · 54 fleet-ps ·
56 windows-fleet-bootstrap · 57 windows-firewall-and-scheduled-task-actions ·
59 typed-observability · 60 observe-system-resources · 61 observe-logs ·
62 observe-gpu · 63 observe-streaming (deferral) · 64 fleet-observe

## Epics (epics/)

| File | Topic |
|---|---|
| epic-agent-efficiency.md | Observable runs, compact output, snapshot, MCP interface |
| epic-spec-21-followup.md | Post-spec-21 modern action surface buildout |
| epic-cluster-management.md | Fleet management, GitOps for software state, AI remediation |
