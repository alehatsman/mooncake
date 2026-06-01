# Goals

## The thesis

> **Mooncake is a typed, idempotent, audited execution layer between an
> actor (human, script, or AI) and a system.**

The same properties that make declarative config managers good
(idempotency, dry-run, typed actions, audit trails) are *exactly* the
properties that make an execution layer safe for AI agents. The same
control surface a platform engineer wants over a fleet is what a solo
developer wants over their personal machines — just at a different
scale.

The product is not "Ansible for AI." It is a **system-call ABI for
intent → physical change**, with three increasingly large rings of
value built on top:

1. **The kernel** — declarative typed actions, idempotency, planning,
   facts. *(Shipped.)*
2. **The runtime** — host daemon, fleet orchestration, audit, policy.
   *(Personal fleet shipped; enterprise hub deferred.)*
3. **The economy** — preset marketplace, agent SDK, signed plans,
   integrations. *(Future.)*

## The core insight

Every actor — sysadmin in a terminal, CI pipeline, Claude/Cursor agent
— ultimately mutates a host through some thin interface (shell, API
calls, file writes). Today that interface is unconstrained: anyone
with the credential can do anything.

Mooncake turns it into a **constrained, observable funnel**:

```
┌──────────┐   ┌──────────┐   ┌──────────┐
│  Human   │   │  Script  │   │   AI     │
└────┬─────┘   └────┬─────┘   └────┬─────┘
     │              │              │
     └──────────────┼──────────────┘
                    ▼
           ┌────────────────┐
           │    Mooncake    │   ← typed actions, plan/dry-run, policy,
           │   execution    │     audit, idempotency, rollback
           │     engine     │
           └────────┬───────┘
                    ▼
               ┌─────────┐
               │ System  │
               └─────────┘
```

If every mutation flows through this funnel, you get auditability,
idempotency, policy enforcement, reversibility, and agent safety —
each for free, because the engine guarantees them rather than the
actor.

That last bullet is the wedge. Nobody else is selling this to AI
developers.

## What "done" looks like, per user

Mooncake serves three audiences with one engine. Each scenario below is
the success bar for that audience.

### 1. Solo developer — dotfiles + dev box on autoagent

Pick up a fresh laptop / VM / WSL / Mac. Run one command. End up with:
dotfiles + dev tools + packages + services + drift detection + audit
trail. Works on Linux, macOS, Windows (WSL or native).

**State:** ~95% there. `mooncake init` ✓, default config discovery ✓,
`mooncake plan --diff` ✓, `mooncake apply` ✓, `mooncake doctor` ✓,
`mooncake history` ✓, parameterized preset system ✓ (in-tree library
retired in favor of a Git-native module system — see
[`sharing_and_modules.md`](./sharing_and_modules.md)), snapshot/diff ✓,
structured errors ✓, run history ✓. Gap: module distribution + lockfile
+ `mooncake share` UX are in design; "import existing dotfiles" doesn't
exist.

### 2. Multi-device on local network — the personal fleet

From any box, `fleet apply config.yml` runs across every machine you
own. Interleaved logs. `fleet status` shows health. `fleet bootstrap
user@new-box` adds a new machine in 60 seconds. Per-host overlays land
naturally. No hub, no SaaS, peer-to-peer over LAN.

**State:** ~99% there. agentd + bearer auth + SSE + sandboxed sync ✓,
multiplexed `fleet apply` ✓, `fleet status` ✓, `fleet logs/facts/exec/
watch/ps/discover/init/upgrade/doctor` ✓, native SSH driver ✓,
per-host overlays + tag selectors ✓, mDNS ✓, `fleet apply <machine>`
✓, Windows agentd ✓. The "Friday-evening demo" success criteria from
the personal-fleet epic are all met.

### 3. AI agent developer — Docker for AI agents

An LLM agent has no shell, no raw file API. Only the Mooncake typed
ABI. Every mutation is dry-runnable, mediated, reversible, audited.
The agent declares intent ("install postgres, create user, create db")
as a `transaction:` block — if step 3 fails, steps 1+2 auto-revert.
Policy DSL says `deny: agent.touches("/etc/passwd")`. Plans are
signed; daemon refuses unsigned ones in prod. Per-action quotas +
egress policy. Deterministic replay for debugging.

**State:** ~80% there. MCP server with `run_step`/`get_facts`/
`get_snapshot`/`check_plan`/`run_plan` ✓, agent loop ✓, structured
JSONL + structured errors ✓, plan-mode with content diffs ✓, snapshot
+ diff ✓, run audit trail ✓, SSE event stream ✓, secret redaction ✓,
four-method ABI (`Permissions`/`Diff`/`Cost`/`Reverse`) declared
across priority handlers **and wired through MCP** ✓, spec-23
`on_change` / `!secret` / `try/catch/finally` ✓, spec-30
`transaction:` with LIFO rollback ✓ (`examples/transactions/
rollback-demo.yml`). Gap: policy DSL, plan signing, per-action
quotas, egress policy, sandbox mode, deterministic replay, risk
scoring on top of `Cost()` — none specced.

### 4. Platform team — fleet control plane with audit by default

Same engine, scaled out. Inventory of hosts, fleet plans with canary/
wave strategy, signed audit log, RBAC, approval gates, dashboards,
drift heatmaps. Free CLI; control plane priced per-host or per-run.

**State:** intentionally deferred until a paying user asks. The
personal-fleet stream proves the wire protocol and agentd shape; the
enterprise hub is a separate epic that builds on it.

## The product surface, layered

```
┌──────────────────────────────────────────────────────────────┐
│  L5: Marketplace + Agent SDK + integrations (GitHub, IDEs)   │ ← future
├──────────────────────────────────────────────────────────────┤
│  L4: Cloud Hub (SaaS or self-hosted) — fleet, audit, policy  │ ← deferred
├──────────────────────────────────────────────────────────────┤
│  L3: Host daemon (agentd) — TCP+SSE, bearer auth, sync       │ ← shipped (personal fleet)
├──────────────────────────────────────────────────────────────┤
│  L2: CLI + MCP server + agent loop                           │ ← shipped
├──────────────────────────────────────────────────────────────┤
│  L1: Kernel — actions, planner, executor, facts, snapshot    │ ← shipped
└──────────────────────────────────────────────────────────────┘
```

Each layer is independently usable. A solo dev only ever sees L1+L2.
An agent developer adds the MCP server (L2) + transactions/secrets.
The personal fleet uses L3. An enterprise eventually consumes all
five.

## The unfair-advantage statement

The combination that no other tool ships today:

```
plan + snapshot + reverse + deterministic replay,
all typed end-to-end
```

Three of four are in master and demoable. Deterministic replay is the
last open piece on that line. An Ansible+OPA+AWX combo can audit
(AWX) and gate (OPA) but cannot automatically revert a half-applied
transaction byte-identically to pre-state — because no handler in that
stack declares a `Reverse()` method. Mooncake's `transaction:` blocks
do that as a built-in.

## The strategic constraint

The code is shipping faster than the lighthouse-user funnel can
absorb. The next bottleneck is **adoption, not engineering** — two or
three real agent-developer users, written up, would matter more right
now than another spec landing.
