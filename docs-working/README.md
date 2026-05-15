# docs-working

Working documents — specs, epics, and notes. Not canonical docs; those live in `docs/`.

## Structure

| Path | Contents |
|---|---|
| `streams.md` | Stream overview — goals, audiences, dependencies, what to work on next |
| `specs/action-surface/` | Stream 1: typed action library (spec-22, 24–28, 32, 36, disk-partition) |
| `specs/personal-fleet/` | Stream 3: personal fleet runtime (spec-45, 52–55, 58, implementation-order) |
| `specs/ecosystem/` | Stream 5: plugins, marketplace, integrations (spec-31) |
| `specs/done/` | Shipped specs — read-only reference |
| `epics/` | Epic-level planning docs |
| `analysis/` | Code quality audits, one-off investigations |
| `deferred/` | Deferred notes not yet specced |

See **[streams.md](./streams.md)** for the full stream breakdown, dependencies, and recommended work order.
See **[action-design-principles.md](./action-design-principles.md)** for the 11 non-negotiable design rules for any new action.

## Active specs by stream

**Stream 1 — Action Surface**
22 extended-handler-abi (phase 8 docs remains) · 24 pkg-surface (P6 ABI hooks) ·
25 text-surface (P5 ABI hooks) · 26 git-actions (ABI hooks) · 27 os-identity (ABI hooks) ·
28 os-scheduling (non-ufw drivers, deferred) · 32 step-action-dispatch ·
36 windows-support · disk-partition-action (exploration)

Phases 1–7 of spec-22 are complete (four-method ABI declared + MCP wiring landed).
Remaining: phase 8 docs, plus final ABI-hook phases for 24–27.

**Stream 2 — Safe Agent Runtime**

spec-23 framework-primitives ✅ (all three sections shipped) and
spec-30 transactions ✅ both moved to done. What remains in this stream
lives under Stream 1 (spec-22 phases 7–8).

**Stream 3 — Fleet & Cluster Management**
45 fleet-discovery (PR13 `fleet init` interactive flow) ·
52 fleet-exec · 53 fleet-watch · 54 fleet-ps · 55 fleet-doctor ·
58 fleet-drift

Fleet is 13/14 PRs; only `fleet init` interactive UX remains from the original plan.
Specs 52–55 are drafted QoL additions brainstormed from real use.
Spec 58 (drift detection) is the highest-leverage candidate from
GitHub issue #11 — see `clustermanagement/issue-11-analysis.md`.

GitHub issue #8 (ChangeGraph as core primitive) audited against current
state in `analysis/issue-8-changegraph-analysis.md` — ~60% already shipped
under spec-22 / spec-30 / spec-58; the genuinely-new bets are `observe.*`,
`explain`, `discover`, `rewind`.

**Stream 4 — Developer Experience**
*(shipped: spec-39 init, 40 config-discovery, 41 doctor, 42 recommend)*

**Stream 5 — Ecosystem**
31 tier2-plugin-model

**Bugs**
None open.

## Shipped specs (specs/done/)

01 run-recap · 02 skip-reasons · 03 agent-jsonl · 04 snapshot · 05 fact-query ·
06 quiet-mode · 07 step-display · 08 run-history · 09 structured-errors ·
10 mcp-server · 11 preset-registry · 12 package-summary · 13 single-step ·
14 snapshot-diff · 15 check-mode · 16 unify-dryrun-execute ·
18 mooncake-agent-daemon · 19 tool-action · 20 metrics · 21 modernization-cutover ·
29 wait-primitives · 30 transactions · 33 execution-context-split ·
34 typed-variable-context · 35 plan-diff ·
43 fleet-transport-and-sync · 44 ssh-bootstrap-transport · 46 fleet-status-and-logs ·
47 fleet-bootstrap-ux · 48 per-host-overlays-and-tags · 49 agentd-on-windows ·
50 extended-filter-keys · 51 local-apply-overlay-parity

## Epics (epics/)

| File | Topic |
|---|---|
| epic-agent-efficiency.md | Observable runs, compact output, snapshot, MCP interface |
| epic-spec-21-followup.md | Post-spec-21 modern action surface buildout |
| epic-cluster-management.md | Fleet management, GitOps for software state, AI remediation |
