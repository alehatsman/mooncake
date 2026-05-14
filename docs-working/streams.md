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
needs `Diff()`, `Reverse()`, and `Cost()` — which is most of specs 23–30
and the final phase of every action spec in this stream. Ship it next; the
remaining per-action phases collapse to "implement four methods" once the
ABI lands.

**Active specs:**

| Spec | Topic | Status |
|---|---|---|
| 32 | Collapse step action dispatch | not started |
| 22 | Extended handler ABI (`Diff` / `Reverse` / `Cost` / `Permissions`) | not started — **blocks the final phase of every action spec below** |
| 17 | Batched packages + templated `names` | not started |
| 24 | `pkg.install` / `pkg.remove` / `pkg.repo` / `pkg.hold` / `pkg.upgrade` / `pkg.list` | P1–P5 shipped; P6 (ABI hooks) blocked on 22; P7 (docs) pending |
| 25 | `text.line` · `text.patch.{ini,json,yaml}` | P1–P4 shipped; P5 (ABI hooks) blocked on 22; P6 (docs) pending |
| 26 | `git.clone` (incl. credentials + submodules) · `git.checkout` · `git.config` | P1–P4 shipped; P5 (ABI hooks) blocked on 22; P6 (docs) pending |
| 27 | `os.user` · `os.group` · `os.ssh_key` | P1–P3 shipped; P4 (ABI hooks) blocked on 22; P5 (docs) pending |
| 28 | `os.cron` · `os.sysctl` · `os.systemd` · `os.mount` · `os.firewall` | P1–P5 shipped (ufw driver only; nftables / firewalld deferred); P6 (ABI hooks) blocked on 22 |
| 37 | Step output capture — collision + plan-mode policy | drafted; prereq for 38 |
| 38 | `read.json` / `read.yaml` | drafted; depends on 37 |

**Suggested order:** 37 → 38 (read-side observation gap) → 22 (against a real
consumer — likely spec-30 transactions as the first concrete user of
`Reverse`) → 32 → 17. Individual action specs land their ABI hook phases as
soon as 22 ships.

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

**Two flavors, same kernel:**

- **Enterprise** (`epics/epic-cluster-management.md`) — central hub, RBAC,
  drift heatmaps, AI remediation. For platform teams running 50–10k nodes.
- **Personal Fleet** (`epics/epic-personal-fleet.md`) — peer-to-peer, no hub,
  trust-on-first-use. For solo devs running their own 1–10 boxes. The DX
  funnel that gets devs onto Mooncake before they bring it to work.

**Enterprise sub-epics:**

| Sub-epic | Topic |
|---|---|
| C1 | Node registry — agentd registers with hub, hub maintains fleet inventory |
| C2 | Fleet plans — apply a plan to N nodes with rollout strategies |
| C3 | Drift detection — continuous convergence loop, drift heatmap |
| C4 | AI-assisted remediation — hub + agent loop generates fix plans on drift |
| C5 | RBAC + approval gates — who can run what on what nodes |

**Personal-fleet specs** (under `specs/personal-fleet/`):

| Spec | Topic |
|---|---|
| [39](specs/personal-fleet/spec-39-fleet-transport-and-sync.md) (P1) | agentd network transport + file sync to `<state_dir>/synced/` + `fleet apply` with multiplexed SSE logs |
| [40](specs/personal-fleet/spec-40-ssh-bootstrap-transport.md) (P2) | SSH bootstrap transport — install mooncake + agentd on a fresh box |
| [41](specs/personal-fleet/spec-41-fleet-discovery.md) (P3) | Discovery — mDNS + SSH-config import + static `peers.toml` |
| [42](specs/personal-fleet/spec-42-fleet-status-and-logs.md) (P4) | `mooncake fleet status` / `logs` / `facts` |
| [43](specs/personal-fleet/spec-43-fleet-bootstrap-ux.md) (P5) | `mooncake fleet bootstrap` + `pair` — wraps spec-40 |
| [44](specs/personal-fleet/spec-44-per-host-overlays-and-tags.md) (P6) | Per-host overlays + tag selectors |

**Key architectural decision before speccing the enterprise side:** where does
the hub live? Same binary (`mooncake hub`)? Separate service? SQLite first,
Postgres when multi-tenant? Decide before writing C1 spec.

---

## Stream 4: Developer Experience

**Goal:** The best dotfiles + dev box management tool that exists. This is the
funnel — solo devs adopt Mooncake here, then bring it into work via streams 2
and 3.

**Audience:** Individual developers managing personal machines, dotfiles, dev
environments.

**Flagship epic:** [`epics/epic-personal-fleet.md`](epics/epic-personal-fleet.md)
— "multi-machine sync" made concrete: `mooncake fleet apply` across your own
1–10 boxes, interleaved logs in your terminal, peer-to-peer, no hub. Sits at
the intersection of Stream 4 (this stream) and Stream 3 (fleet plumbing).

**Other future work (not yet specced):**

- `mooncake doctor` — interactive: "your nvim config is 14 days stale vs git; pull?"
- `mooncake init dotfiles` — scaffolded repo + first run
- Drift detection UX — "your dev box no longer matches your committed config"
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
| 1 | Action Surface | **specs 37 + 38** (read.json / read.yaml) — closes the observation gap; concrete user value, no ABI bets |
| 2 | Safe Agent Runtime | spec-30 (transactions) — first real consumer of `Reverse`, drives the spec-22 design |
| 3 | Action Surface | spec-22 (extended ABI) once spec-30 has the use case to design against |
| 4 | Fleet & Cluster | C1 spec (node registry) — write spec first |
| 5 | Ecosystem | spec-31 (plugin model) — independent, can run in parallel |
