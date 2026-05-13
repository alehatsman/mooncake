# docs-working

Working documents — specs, epics, and notes. Not canonical docs; those live in `docs/`.

## Structure

| Directory | Contents |
|---|---|
| `specs/` | Active draft specs |
| `specs/done/` | Shipped specs (read-only reference) |
| `epics/` | Epic-level planning docs — groups of related specs, future work |
| `analysis/` | Code quality audits, one-off investigations, research notes |
| `deferred/` | Deferred notes and reference material not yet specced |

## Active specs (specs/)

| # | File | Topic | Epic | Status |
|---|---|---|---|---|
| 17 | spec-17-package-batch-and-template.md | Batched packages + templated names | E8 | partial |
| 22 | spec-22-extended-handler-abi.md | Extended handler ABI (Diff/Reverse/Cost) | E9.1 | not started |
| 23 | spec-23-framework-primitives.md | `on_change`, `try/catch`, `!secret` refs | E9.2 | not started |
| 24 | spec-24-pkg-surface.md | `pkg.install` / `pkg.remove` / `pkg.repo` | E9.3 | not started |
| 25 | spec-25-text-surface.md | `text.line`, structural patches | E9.3 | not started |
| 26 | spec-26-git-actions.md | `git.clone`, `git.checkout`, `git.config` | E9.3 | not started |
| 27 | spec-27-os-identity.md | `os.user`, `os.group`, `os.ssh_key` | E9.3 | not started |
| 28 | spec-28-os-scheduling.md | `os.cron`, `os.firewall`, `os.mount`, `os.sysctl` | E9.3 | not started |
| 29 | spec-29-wait-primitives.md | `wait.port`, `wait.http`, `wait.file` | E9.3 | not started |
| 30 | spec-30-transactions.md | `transaction:` blocks with reverse-on-failure | E9.4 | not started |
| 31 | spec-31-tier2-plugin-model.md | Tier-2 plugin model (`notify.*` proof of concept) | E9.5 | not started |
| 32 | spec-32-step-action-dispatch.md | Collapse step action dispatch | structural | partial |

Note: spec-22 is a blocking dependency for specs 23–30.

## Shipped specs (specs/done/)

01 run-recap · 02 skip-reasons · 03 agent-jsonl · 04 snapshot · 05 fact-query ·
06 quiet-mode · 07 step-display · 08 run-history · 09 structured-errors ·
10 mcp-server · 11 preset-registry · 12 package-summary · 13 single-step ·
14 snapshot-diff · 15 check-mode · 16 unify-dryrun-execute ·
18 mooncake-agent-daemon · 19 tool-action · 20 metrics · 21 modernization-cutover

## Epics (epics/)

| File | Topic |
|---|---|
| epic-agent-efficiency.md | Observable runs, compact output, snapshot, MCP interface |
| epic-spec-21-followup.md | Post-spec-21 modern action surface buildout |
| epic-cluster-management.md | Fleet management, GitOps for software state, AI remediation |
