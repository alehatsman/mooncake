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
32 dispatch · 22 extended-handler-abi · 17 package-batch · 24 pkg-surface ·
25 text-surface · 26 git-actions · 27 os-identity · 28 os-scheduling · 29 wait-primitives

**Stream 2 — Safe Agent Runtime** *(needs spec-22 first)*
23 framework-primitives · 30 transactions

**Stream 3 — Fleet & Cluster Management**
*(no numbered specs yet — see epics/epic-cluster-management.md)*

**Stream 4 — Developer Experience**
*(no specs yet)*

**Stream 5 — Ecosystem**
31 tier2-plugin-model

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
