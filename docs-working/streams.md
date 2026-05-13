# Mooncake — Development Streams

Five parallel threads of work, each serving a distinct purpose and audience.
Specs live under `specs/<stream-name>/`. Epics live under `epics/`.

---

## Stream 1: Action Surface

**Goal:** A complete, extensible vocabulary of typed system mutations.

Every other stream depends on this. The action library is the kernel's API —
the set of things Mooncake can do. It needs to be complete enough for real-world
provisioning, deep enough for AI agents, and extensible enough that the
community can fill the long tail without forking.

**Audience:** All three wedges — solo devs, AI agent developers, platform teams.

**Key dependency:** spec-22 (extended handler ABI) unlocks everything that
needs `Diff()`, `Reverse()`, and `Cost()` — which is most of specs 23–30.
Ship it first.

**Active specs:**

| Spec | Topic | Blocks |
|---|---|---|
| 32 | Collapse step action dispatch | — |
| 22 | Extended handler ABI (Diff / Reverse / Cost / Permissions) | 23, 30 |
| 17 | Batched packages + templated `names` | — |
| 24 | `pkg.install` / `pkg.remove` / `pkg.repo` | — |
| 25 | `text.line`, structural patches (JSON/YAML/INI) | — |
| 26 | `git.clone`, `git.checkout`, `git.config` | — |
| 27 | `os.user`, `os.group`, `os.ssh_key` | — |
| 28 | `os.cron`, `os.firewall`, `os.mount`, `os.sysctl` | — |
| 29 | `wait.port`, `wait.http`, `wait.file` | — |

**Suggested order:** 32 → 22 → 17 → 24–29 in parallel

---

## Stream 2: Safe Agent Runtime

**Goal:** AI agents can only mutate the system through Mooncake's typed ABI.
Every mutation is mediated, auditable, and reversible. The agent literally
cannot escape the sandbox.

This is the most defensible wedge in the vision. Nobody else is selling a
typed, dry-runnable, reversible execution layer to AI developers. Existing
sandboxes (Daytona, E2B, Modal) sandbox the *environment*; Mooncake sandboxes
the *intent*. They are complementary — Mooncake fits inside their VMs.

**Audience:** AI agent developers building on Claude, Cursor, Codex, or any
LLM that can call tools.

**Depends on:** Stream 1 (spec-22 `Reverse()` is required for transactions).

**Active specs:**

| Spec | Topic |
|---|---|
| 23 | Framework primitives: `on_change`, `try`/`catch`/`finally`, `!secret` refs |
| 30 | `transaction:` blocks with automatic reverse-on-failure |

**Future work (not yet specced):**

- Policy DSL — `deny: agent.touches("/etc/passwd")`
- Plan signing — daemon refuses unsigned plans in production mode
- Per-action quotas — "this agent may make at most 10 file edits"
- Egress policy — "this agent may only download from npm/pypi/github"
- Sandbox mode — agent has no shell or file API, only the Mooncake ABI
- Cost / risk classifier — classify each step by blast radius before execution
- Deterministic replay — rerun an agent's exact plan for debugging

**Suggested order:** spec-23 → spec-30 (needs spec-22 first) → policy DSL

---

## Stream 3: Fleet & Cluster Management

**Goal:** GitOps for software state at fleet scale. The same guarantees
Mooncake gives on one node — idempotent, dry-run, audited, AI-assisted —
applied across a fleet of nodes.

This is the monetizable wedge. Same engine, scaled out. The hub adds exactly
one thing to the kernel: awareness that multiple nodes exist.

**Audience:** Platform teams managing clusters, HPC operators, any team running
N machines that need to stay in declared state.

**Distinct from traditional cluster managers (e.g. BCM cmdaemon):**
cmdaemon manages everything at and below the OS (hardware, BMC, provisioning,
WLM). Mooncake manages everything above the OS (software state, config, tools).
They are complementary, not competing.

**No numbered specs yet** — see `epics/epic-cluster-management.md` for the
full breakdown. Proposed sub-epics:

| Sub-epic | Topic |
|---|---|
| C1 | Node registry — agentd registers with hub, hub maintains fleet inventory |
| C2 | Fleet plans — apply a plan to N nodes with rollout strategies |
| C3 | Drift detection — continuous convergence loop, drift heatmap |
| C4 | AI-assisted remediation — hub + agent loop generates fix plans on drift |
| C5 | RBAC + approval gates — who can run what on what nodes |

**Key architectural decision before speccing:** where does the hub live? Same
binary (`mooncake hub`)? Separate service? SQLite first, Postgres when
multi-tenant? Decide before writing C1 spec.

---

## Stream 4: Developer Experience

**Goal:** The best dotfiles + dev box management tool that exists. This is the
funnel — solo devs adopt Mooncake here, then bring it into work via streams 2
and 3.

**Audience:** Individual developers managing personal machines, dotfiles, dev
environments.

**No active specs** — this stream is mostly UX and CLI work, not new actions.

**Future work (not yet specced):**

- `mooncake doctor` — interactive: "your nvim config is 14 days stale vs git; pull?"
- `mooncake init dotfiles` — scaffolded repo + first run
- Drift detection UX — "your dev box no longer matches your committed config"
- Multi-machine sync — same config across laptop / desktop / VPS with per-host overrides
- TUI dashboard — what's installed, what's missing, what's drifted
- `mooncake share <preset>` — push a preset to the marketplace

---

## Stream 5: Ecosystem

**Goal:** Mooncake becomes a standard. Community fills the long tail of
actions, presets, and integrations without forking the core.

**Audience:** Contributors, teams wanting integrations, organizations building
on top of Mooncake.

**Active specs:**

| Spec | Topic |
|---|---|
| 31 | Tier-2 plugin model — `notify.*` as proof of concept for loadable action plugins |

**Future work (not yet specced):**

- WASM plugin SDK — write custom actions in Rust / TS / Go
- Preset marketplace — `mooncake install postgres@2.1.0`, signed and versioned
- GitHub Actions / GitLab CI step — drop-in CI integration
- IDE extensions — VSCode / Cursor preview AI changes through Mooncake before applying
- Foundation-model fine-tuning dataset — open corpus of Mooncake YAML

---

## Stream dependencies

```
Stream 1 (Action Surface)
    └── Stream 2 (Safe Agent Runtime)   ← needs spec-22 Reverse()
    └── Stream 3 (Fleet & Cluster)      ← uses actions at fleet scale
    └── Stream 5 (Ecosystem)            ← plugin model extends action surface

Stream 4 (Developer Experience)         ← independent; uses L1+L2 only
```

Stream 1 is the infrastructure layer. Streams 2, 3, and 5 layer on top of it.
Stream 4 is the most independent — it is primarily UX work on the existing kernel.

---

## What to work on next

| Priority | Stream | First spec |
|---|---|---|
| 1 | Action Surface | spec-32 (dispatch cleanup) → spec-22 (extended ABI) |
| 2 | Safe Agent Runtime | spec-23 (framework primitives) after spec-22 |
| 3 | Fleet & Cluster | C1 spec (node registry) — write spec first |
| 4 | Ecosystem | spec-31 (plugin model) — independent, can run in parallel |
| 5 | Developer Experience | `mooncake doctor` — define scope before speccing |
