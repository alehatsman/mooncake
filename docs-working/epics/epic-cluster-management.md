# Epic: Cluster Management — GitOps for Software State at Fleet Scale

> Status: brainstorming / future epic. Not a roadmap. Iterate here before
> moving to formal specs under `docs-working/`.
>
> **Sibling epic (✅ shipped):** [`epic-personal-fleet.md`](done/epic-personal-fleet.md) covers
> the same primitives at the *solo-developer* scale (1–10 personal machines,
> peer-to-peer, no hub). All 14 PRs of its plan landed on `master`. This doc is the *platform-team* version (central hub,
> RBAC, drift heatmaps, AI remediation). They share agentd and the wire
> protocol; they diverge on control plane and trust model.

---

## The thesis

> **Mooncake's cluster management story is: GitOps for software state, with
> AI-assisted remediation, at fleet scale.**

The same kernel that manages dotfiles on a laptop — idempotent actions, typed
mutations, dry-run, facts, snapshot/diff — can manage software configuration
across a fleet of nodes. The hub adds exactly one thing to the kernel: awareness
that multiple nodes exist. Everything else (idempotency, dry-run, audit, AI
remediation) already works; you fan it out.

This is not a separate product. It is the natural outward expansion of Mooncake's
existing rings:

```
L1: Kernel     — actions, planner, executor, facts, snapshot  (done)
L2: CLI + MCP  — local tooling, agent interface               (done)
L3: agentd     — long-running per-node daemon                 (done)
L4: Hub        — fleet coordinator, inventory, drift, RBAC    ← this epic
```

---

## How Mooncake got here

Mooncake started as a dotfiles manager. The natural progression:

1. **Dotfiles** → typed, idempotent config application on one machine
2. **agentd** → that machine now has a daemon accepting async plan submissions
3. **facts + metrics** → the daemon knows what the machine *is* and what it
   *looks like right now*
4. **remote state application** → plans can be pushed to remote agentd instances

At that point you have all the primitives for cluster management. The only
missing piece is a coordinator that knows about multiple nodes and orchestrates
across them.

---

## How this differs from traditional cluster managers (e.g. BCM cmdaemon)

BCM's cmdaemon is a useful comparison point. It is the central orchestration
daemon for NVIDIA HPC clusters — a large C++ system that manages hardware
power states, node provisioning, WLM job lifecycles, BMC/IPMI interfaces, and
cluster topology. It is excellent at what it does.

### The fundamental difference

| Dimension | cmdaemon (BCM) | Mooncake Hub |
|---|---|---|
| **Model** | Imperative state machine | Declarative convergence |
| **Driver** | Event-driven (node up/down, job start) | Desired-state diff |
| **Scope** | Hardware + OS + software | Software state only |
| **Mutations** | Typed but stateful (power on, boot node) | Typed and idempotent |
| **AI integration** | None | First-class actor in the audit log |
| **Dry-run** | Partial (some ops) | Full (all actions) |
| **Audit** | Event log | Typed events, content diffs, signed |

### What cmdaemon does that Mooncake intentionally does not

- **Hardware control plane**: PDU power, BMC/IPMI/Redfish, firmware updates.
  Mooncake assumes the node already exists and the OS is running.
- **OS provisioning / PXE boot**: Image distribution, boot server coordination,
  multi-phase install workflows. Out of scope.
- **Workload manager integration**: Slurm/PBS/LSF job lifecycle tracking,
  dynamic node allocation per job. Not a config management concern.
- **HA quorum voting**: Distributed consensus for head-node failover. Out of
  scope for v1.

### The clean boundary

> cmdaemon manages everything **at and below the OS**.
> Mooncake manages everything **above the OS**.

They are complementary. A node provisioned by cmdaemon (or any other tool) that
has agentd running is a valid Mooncake fleet member. In a BCM cluster, Mooncake
handles the software configuration layer that cmdaemon does not: dev tools,
dotfiles for HPC users, application configs, ML environment setup, drift
detection on installed software.

### What Mooncake does that cmdaemon cannot

- **Declarative desired state with full dry-run**: every change previewed before
  applied, at node and fleet level.
- **AI-driven remediation**: hub detects drift → LLM generates a remediation
  plan → human approves → Mooncake applies to affected nodes with wave rollout.
  cmdaemon has no concept of an AI actor proposing and executing config changes.
- **Deterministic replay**: an agent's exact plan can be replayed on any node
  for debugging or auditing. No equivalent in cmdaemon.
- **Content-level diffs**: Mooncake knows what a file *was* and what it *became*.
  cmdaemon tracks entity state, not content.
- **Preset marketplace**: reusable, versioned, community-shareable config
  workflows. cmdaemon has custom scripts; Mooncake has a typed ecosystem.

---

## Primitives already in place

| Primitive | Status | Role in cluster management |
|---|---|---|
| Typed actions (13) | Production | The mutation vocabulary |
| Facts (cached per node) | Production | Know what each node *is* |
| Metrics (live per node) | Working | Know what each node looks like *now* |
| snapshot/diff | Working | Know what changed and when |
| agentd (Unix socket daemon) | Working | Per-node async plan executor |
| MCP server | Working | AI agent interface |
| Agent loop | Working | Iterate-until-done for AI actors |
| Run log / structured events | Working | Per-node audit trail |

What is missing (thin list):

1. **Node registry** — hub knows which agentd instances exist, their tags, their
   last-seen facts snapshot.
2. **Fleet plans** — apply a plan to all nodes matching a tag selector.
3. **Rollout strategies** — canary, wave, serial/parallel, halt-on-failure.
4. **Convergence loop** — continuous: collect fleet facts → diff vs declared
   state → propose remediations → apply on approval.
5. **Cross-node dependencies** — `depends_on` between waves (e.g., DB tier
   before app tier).

---

## Proposed epics

### C1: Node registry + fleet inventory

A hub that nodes register with. agentd sends a heartbeat on startup and
periodically thereafter, pushing its current facts snapshot and run history
summary. The hub persists a fleet model.

**agentd side:**
- `--hub <url> --token <token>` flags on `mooncake agentd`
- On startup: POST facts + metadata to hub `/v1/nodes/register`
- Periodic heartbeat (60s): POST updated facts + metrics summary
- On run complete: POST run record to hub

**Hub side:**
- Accept node registrations, store facts snapshot per node
- Expose `GET /v1/nodes` — list all nodes with last-seen, facts, tags, status
- Expose `GET /v1/nodes/{id}/facts` — full facts for a node
- Expose `GET /v1/nodes/{id}/runs` — run history for a node
- Node tags: static (set at registration) + dynamic (derived from facts)

**CLI:**
```
mooncake fleet nodes               # list all fleet nodes
mooncake fleet nodes --tag role=web  # filter by tag
mooncake fleet facts --query go.version  # query a fact across fleet
```

### C2: Fleet plans + rollout strategies

Apply a plan to N nodes matching a tag selector, with configurable rollout
strategy.

**Plan format extension:**
```yaml
# fleet-plan.yml
hosts:
  tags:
    role: gpu-worker
    env: prod
rollout:
  strategy: wave      # serial | parallel | wave | canary
  wave_size: 10%
  halt_on_failure: true
  pause_between_waves: 30s
steps:
  - include: presets/cuda/env.yml
  - template:
      src: templates/nccl.conf.j2
      dest: /etc/nccl.conf
```

The hub fans this out: resolves matching nodes → groups into waves → dispatches
individual plans to each node's agentd → collects results → advances or halts.

**CLI:**
```
mooncake fleet apply fleet-plan.yml --dry-run
mooncake fleet apply fleet-plan.yml
mooncake fleet status <run-id>         # stream fleet run progress
```

### C3: Drift detection + convergence loop

Continuous background loop in the hub:

1. Collect facts from all nodes (via heartbeat or on-demand pull)
2. For each node, resolve its declared plans (from git or plan store)
3. Run plan in check mode against node's fact snapshot
4. If drift detected: emit drift event, update node status in inventory
5. Optionally: auto-generate remediation plan (AI or deterministic diff)

**Hub endpoints:**
- `GET /v1/nodes/{id}/drift` — current drift status vs declared plan
- `GET /v1/fleet/drift` — fleet-wide drift heatmap
- `POST /v1/nodes/{id}/remediate` — trigger remediation (dry-run first)

**CLI:**
```
mooncake fleet drift                    # show fleet drift heatmap
mooncake fleet drift --node web-03      # drift detail for one node
mooncake fleet remediate --dry-run      # preview remediation plans
mooncake fleet remediate --approve      # apply approved remediations
```

### C4: AI-assisted remediation

The most differentiated feature. When drift is detected, the hub can engage
the agent loop to generate a remediation plan.

Flow:
1. Hub detects drift on node X (config Y has diverged from declared state Z)
2. Hub calls agent loop: "here is the drift diff, generate a remediation plan"
3. Agent outputs a Mooncake YAML plan
4. Hub runs the plan in check mode against node X's facts → produces a diff
5. Human reviews diff in hub UI or CLI, approves
6. Hub dispatches plan to node X's agentd
7. Run events stream back, audit log updated

This loop is what cmdaemon fundamentally cannot do: the remediation is
typed, dry-runnable, audited, and proposed by an AI that understands the
desired state — not hand-written scripts.

### C5: RBAC + approval gates

Who can run what action on what nodes.

**Roles (minimum viable):**
- `read` — view inventory, facts, run history
- `propose` — submit plans for approval
- `approve` — approve pending plans
- `execute` — run plans directly (no approval required)

**Policy hooks:**
- Any step touching `/etc/` → requires approval
- Any action on `env=prod` nodes → requires approval
- Any `shell` action → always requires approval (configurable)

These mirror the RBAC ideas in the vision (§8.2) but scoped to the fleet
context.

---

## What stays out of scope

To keep Mooncake's model clean, the following are **explicitly out of scope**
for the cluster management epic:

- **Hardware control** (PDU, BMC, BIOS, firmware) — assume node already exists
- **OS provisioning** (PXE, image distribution) — assume OS is running
- **Workload manager integration** (Slurm/PBS job lifecycle) — not config
- **HA quorum for the hub itself** — solve when needed; SQLite → Postgres path
- **Network topology modeling** — Mooncake sees nodes as peers, not as a
  physical topology graph

---

## The AI angle on cluster management

The vision doc (§7) describes the safe agent runtime as the most defensible
wedge. Cluster management amplifies this:

> Every node in a fleet has an agentd. Every agentd has an MCP interface.
> Every MCP interface is callable by an AI agent. The hub is the orchestrator
> that gives the agent fleet-wide visibility and safe, audited write access.

A Claude agent that can see all nodes' facts, detect drift across the fleet,
generate remediation plans, have a human approve them, and apply them in waves
with rollback capability — that is not a feature any existing cluster manager
offers. cmdaemon has none of this. Ansible AWX has none of this. Mooncake
can have it because the kernel was designed for it from the start.

---

## Sequencing instinct

| Epic | Dependency | Notes |
|---|---|---|
| C1 Node registry | agentd working ✅ | Build first; everything else needs it |
| C2 Fleet plans | C1 | Need node list to fan out to |
| C3 Drift detection | C1 + check mode ✅ | Continuous convergence loop |
| C4 AI remediation | C3 + agent loop ✅ | The killer demo |
| C5 RBAC | C2 + hub auth | Can be lightweight at first |

Start with C1 (node registry). It is the thinnest useful increment: agentd
phones home, hub knows what exists. Everything else layers on top.

---

## Open questions

1. **Hub persistence**: SQLite first (self-contained, zero-deps), Postgres when
   multi-tenant or high write volume is needed. What's the migration path?
2. **Node identity**: How does a node prove it is who it says it is? mTLS
   client certs issued by hub CA? Token-based? Both?
3. **Declared state location**: Where does the hub find the desired plan for
   each node? Git repo (GitOps model)? Hub's own plan store? Both?
4. **Fleet plan format**: New top-level file type (`fleet-plan.yml`) vs.
   metadata in existing config.yml (`hosts:` block)? New type is cleaner.
5. **Offline nodes**: If agentd is unreachable, hub marks node as
   `disconnected` and queues plans. What's the retry/expiry policy?
6. **Cross-node DAG**: "DB tier before app tier" needs a dependency graph in
   the fleet plan. Start with `depends_on_wave` (simple) or full DAG?
7. **Hub HA**: Two hub instances = split brain. Start with single-hub +
   SQLite; defer HA until there are paying customers who need it.
