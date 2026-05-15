# docs-working

Working documents — specs, epics, and notes. Not canonical docs; those live in `docs/`.

## Structure

| Path | Contents |
|---|---|
| `streams.md` | Stream overview — goals, audiences, dependencies, what to work on next |
| `specs/action-surface/` | Stream 1: typed action library (spec-17, 22, 24–29, 32) |
| `specs/safe-agent-runtime/` | Stream 2: LLM execution safety (spec-23, 30) |
| `specs/fleet-cluster/` | Stream 3: declarative cluster management (future specs) |
| `specs/developer-experience/` | Stream 4: solo dev UX (future specs) |
| `specs/ecosystem/` | Stream 5: plugins, marketplace, integrations (spec-31) |
| `specs/done/` | Shipped specs — read-only reference |
| `epics/` | Epic-level planning docs |
| `analysis/` | Code quality audits, one-off investigations |
| `deferred/` | Deferred notes not yet specced |

See **[streams.md](./streams.md)** for the full stream breakdown, dependencies, and recommended work order.
See **[action-design-principles.md](./action-design-principles.md)** for the 11 non-negotiable design rules for any new action.

## Active specs by stream

**Stream 1 — Action Surface**
22 extended-handler-abi (phases 7–8 remain) · 24 pkg-surface (P6 ABI hooks) ·
25 text-surface (P5 ABI hooks) · 26 git-actions (ABI hooks) · 27 os-identity (ABI hooks) ·
28 os-scheduling (non-ufw drivers) · 32 step-action-dispatch ·
37 step-output-capture · 38 read-json-yaml

Phases 1–6 of spec-22 are complete (all 4 handler methods declared across priority set).
Remaining: MCP wiring (phase 7), docs (phase 8), final ABI-hook phases for 24–28.

**Stream 2 — Safe Agent Runtime**
23 framework-primitives (§2 try/catch/finally remains; §1 on_change + §3 !secret ✅)

spec-30 transactions ✅ moved to done.

**Stream 3 — Fleet & Cluster Management**
45 fleet-discovery (PR13 `fleet init` interactive flow) ·
52 fleet-exec · 53 fleet-watch · 54 fleet-ps · 55 fleet-doctor

Fleet is 13/14 PRs; only `fleet init` interactive UX remains from the original plan.
Specs 52–55 are drafted QoL additions brainstormed from real use.

**Stream 4 — Developer Experience**
*(shipped: spec-39 init, 40 config-discovery, 41 doctor, 42 recommend)*

**Stream 5 — Ecosystem**
31 tier2-plugin-model

**Bugs**
`bug/bug-symlink-force-plan-inspect.md` — `file.write` `state: link` + `force: true` fails `mooncake plan` when path is a non-symlink dir. Fix sketched; small scope.

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
